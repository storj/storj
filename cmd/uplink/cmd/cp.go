// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	progressbar "github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/ranger/httpranger"
	"storj.io/common/sync2"
	"storj.io/uplink"
	"storj.io/uplink/private/object"
)

var (
	progress     *bool
	expires      *string
	metadata     *string
	parallelism  *int
	byteRangeStr *string
)

func init() {
	cpCmd := addCmd(&cobra.Command{
		Use:   "cp SOURCE DESTINATION",
		Short: "Copies a local file or Storj object to another location locally or in Storj",
		RunE:  copyMain,
		Args:  cobra.ExactArgs(2),
	}, RootCmd)

	progress = cpCmd.Flags().Bool("progress", true, "if true, show progress")
	expires = cpCmd.Flags().String("expires", "", "optional expiration date of an object. Please use format (yyyy-mm-ddThh:mm:ssZhh:mm)")
	metadata = cpCmd.Flags().String("metadata", "", "optional metadata for the object. Please use a single level JSON object of string to string only")
	parallelism = cpCmd.Flags().Int("parallelism", 1, "controls how many parallel uploads/downloads of a single object will be performed")
	byteRangeStr = cpCmd.Flags().String("range", "", "Downloads the specified range bytes of an object. For more information about the HTTP Range header, see https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35")

	setBasicFlags(cpCmd.Flags(), "progress", "expires", "metadata", "parallelism", "range")
}

// upload transfers src from local machine to s3 compatible object dst.
func upload(ctx context.Context, src fpath.FPath, dst fpath.FPath, expiration time.Time, metadata []byte, showProgress bool) (err error) {
	if !src.IsLocal() {
		return fmt.Errorf("source must be local path: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	// if object name not specified, default to filename
	if strings.HasSuffix(dst.String(), "/") || dst.Path() == "" {
		dst = dst.Join(src.Base())
	}

	var file *os.File
	if src.Base() == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(src.Path())
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, file.Close()) }()
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("source cannot be a directory: %s", src)
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	if *parallelism < 1 {
		return fmt.Errorf("parallelism must be at least 1")
	}

	var customMetadata uplink.CustomMetadata
	if len(metadata) > 0 {
		err := json.Unmarshal(metadata, &customMetadata)
		if err != nil {
			return err
		}

		if err := customMetadata.Verify(); err != nil {
			return err
		}
	}

	var bar *progressbar.ProgressBar
	if *parallelism <= 1 {
		reader := io.Reader(file)

		if showProgress {
			bar = progressbar.New64(fileInfo.Size())
			reader = bar.NewProxyReader(reader)
			bar.Start()
		}

		upload, err := project.UploadObject(ctx, dst.Bucket(), dst.Path(), &uplink.UploadOptions{
			Expires: expiration,
		})
		if err != nil {
			return err
		}

		err = upload.SetCustomMetadata(ctx, customMetadata)
		if err != nil {
			abortErr := upload.Abort()
			err = errs.Combine(err, abortErr)
			return err
		}

		_, err = io.Copy(upload, reader)
		if err != nil {
			abortErr := upload.Abort()
			err = errs.Combine(err, abortErr)
			return err
		}

		if err := upload.Commit(); err != nil {
			return err
		}
	} else {
		err = func() (err error) {
			if showProgress {
				bar = progressbar.New64(fileInfo.Size())
				bar.Start()
			}

			info, err := project.BeginUpload(ctx, dst.Bucket(), dst.Path(), &uplink.UploadOptions{
				Expires: expiration,
			})
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					err = errs.Combine(err, project.AbortUpload(ctx, dst.Bucket(), dst.Path(), info.UploadID))
				}
			}()

			var (
				limiter = sync2.NewLimiter(*parallelism)
				es      errs.Group
				mu      sync.Mutex
			)

			cancelCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			addError := func(err error) {
				mu.Lock()
				defer mu.Unlock()

				es.Add(err)
				cancel()
			}

			objectSize := fileInfo.Size()
			partSize := 64 * memory.MiB.Int64() // TODO make it configurable
			numberOfParts := (objectSize + partSize - 1) / partSize

			for i := uint32(0); i < uint32(numberOfParts); i++ {
				partNumber := i + 1
				offset := int64(i) * partSize
				length := partSize
				if offset+length > objectSize {
					length = objectSize - offset
				}
				var reader io.Reader
				reader = io.NewSectionReader(file, offset, length)
				if showProgress {
					reader = bar.NewProxyReader(reader)
				}

				ok := limiter.Go(cancelCtx, func() {
					err := uploadPart(cancelCtx, project, dst, info.UploadID, partNumber, reader)
					if err != nil {
						addError(err)
						return
					}
				})
				if !ok {
					break
				}
			}

			limiter.Wait()

			if err := es.Err(); err != nil {
				return err
			}

			_, err = project.CommitUpload(ctx, dst.Bucket(), dst.Path(), info.UploadID, &uplink.CommitUploadOptions{
				CustomMetadata: customMetadata,
			})
			return err
		}()
		if err != nil {
			return err
		}
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("Created %s\n", dst.String())

	return nil
}

func uploadPart(ctx context.Context, project *uplink.Project, dst fpath.FPath, uploadID string, partNumber uint32, reader io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	upload, err := project.UploadPart(ctx, dst.Bucket(), dst.Path(), uploadID, partNumber)
	if err != nil {
		return err
	}

	_, err = sync2.Copy(ctx, upload, reader)
	if err != nil {
		return err
	}

	return upload.Commit()
}

// WriterAt wraps writer and progress bar to display progress correctly.
type WriterAt struct {
	object.WriterAt
	bar *progressbar.ProgressBar
}

// WriteAt writes bytes to wrapped writer and add amount of bytes to progress bar.
func (w *WriterAt) WriteAt(p []byte, off int64) (n int, err error) {
	n, err = w.WriterAt.WriteAt(p, off)
	w.bar.Add(n)
	return
}

// Truncate truncates writer to specific size.
func (w *WriterAt) Truncate(size int64) error {
	w.bar.SetTotal(size)
	return w.WriterAt.Truncate(size)
}

// download transfers s3 compatible object src to dst on local machine.
func download(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if !dst.IsLocal() {
		return fmt.Errorf("destination must be local path: %s", dst)
	}

	if *parallelism < 1 {
		return fmt.Errorf("parallelism must be at least 1")
	}

	if *parallelism > 1 && *byteRangeStr != "" {
		return fmt.Errorf("--parellelism and --range flags are mutually exclusive")
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	if fileInfo, err := os.Stat(dst.Path()); err == nil && fileInfo.IsDir() {
		dst = dst.Join(src.Base())
	}

	var file *os.File
	if dst.Base() == "-" {
		file = os.Stdout
	} else {
		file, err = os.Create(dst.Path())
		if err != nil {
			return err
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Printf("error closing file: %+v\n", err)
			}
		}()
	}

	var bar *progressbar.ProgressBar
	var contentLength int64
	if *parallelism <= 1 {
		var downloadOpts *uplink.DownloadOptions

		if *byteRangeStr != "" {
			// TODO: if range option will be frequently used we may think about avoiding this call
			statObject, err := project.StatObject(ctx, src.Bucket(), src.Path())
			if err != nil {
				return err
			}
			bRange, err := httpranger.ParseRange(*byteRangeStr, statObject.System.ContentLength)
			if err != nil && bRange == nil {
				return fmt.Errorf("error parsing range: %w", err)
			}
			if len(bRange) == 0 {
				return fmt.Errorf("invalid range")
			}
			if len(bRange) > 1 {
				return fmt.Errorf("retrieval of multiple byte ranges of data not supported: %d provided", len(bRange))
			}
			downloadOpts = &uplink.DownloadOptions{
				Offset: bRange[0].Start,
				Length: bRange[0].Length,
			}
			contentLength = bRange[0].Length
		}

		download, err := project.DownloadObject(ctx, src.Bucket(), src.Path(), downloadOpts)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, download.Close()) }()

		var reader io.ReadCloser
		if showProgress {
			if contentLength <= 0 {
				info := download.Info()
				contentLength = info.System.ContentLength
			}
			bar = progressbar.New64(contentLength)
			reader = bar.NewProxyReader(download)
			bar.Start()
		} else {
			reader = download
		}

		_, err = io.Copy(file, reader)
	} else {
		var writer object.WriterAt
		if showProgress {
			bar = progressbar.New64(0)
			bar.Set(progressbar.Bytes, true)
			writer = &WriterAt{file, bar}
			bar.Start()
		} else {
			writer = file
		}

		// final DownloadObjectAt method signature is under design so we can still have some
		// inconsistency between naming e.g. concurrency - parallelism.
		err = object.DownloadObjectAt(ctx, project, src.Bucket(), src.Path(), writer, &object.DownloadObjectAtOptions{
			Concurrency: *parallelism,
		})
	}

	if bar != nil {
		bar.Finish()
	}
	if err != nil {
		return err
	}

	if dst.Base() != "-" {
		fmt.Printf("Downloaded %s to %s\n", src.String(), dst.String())
	}

	return nil
}

// copy copies s3 compatible object src to s3 compatible object dst.
func copyObject(ctx context.Context, src fpath.FPath, dst fpath.FPath) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	download, err := project.DownloadObject(ctx, src.Bucket(), src.Path(), nil)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, download.Close()) }()

	downloadInfo := download.Info()

	var bar *progressbar.ProgressBar
	var reader io.Reader
	if *progress {
		bar = progressbar.New64(downloadInfo.System.ContentLength)
		reader = bar.NewProxyReader(download)
		bar.Start()
	} else {
		reader = download
	}

	// if destination object name not specified, default to source object name
	if strings.HasSuffix(dst.Path(), "/") {
		dst = dst.Join(src.Base())
	}

	upload, err := project.UploadObject(ctx, dst.Bucket(), dst.Path(), &uplink.UploadOptions{
		Expires: downloadInfo.System.Expires,
	})

	_, err = io.Copy(upload, reader)
	if err != nil {
		abortErr := upload.Abort()
		return errs.Combine(err, abortErr)
	}

	err = upload.SetCustomMetadata(ctx, downloadInfo.Custom)
	if err != nil {
		abortErr := upload.Abort()
		return errs.Combine(err, abortErr)
	}

	err = upload.Commit()
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}
	if err != nil {
		return err
	}

	fmt.Printf("%s copied to %s\n", src.String(), dst.String())

	return nil
}

// copyMain is the function executed when cpCmd is called.
func copyMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no object specified for copy")
	}
	if len(args) == 1 {
		return fmt.Errorf("no destination specified")
	}

	ctx, _ := withTelemetry(cmd)

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	dst, err := fpath.New(args[1])
	if err != nil {
		return err
	}

	// if both local
	if src.IsLocal() && dst.IsLocal() {
		return errors.New("at least one of the source or the destination must be a Storj URL")
	}

	// if uploading
	if src.IsLocal() {
		var expiration time.Time
		if *expires != "" {
			expiration, err = time.Parse(time.RFC3339, *expires)
			if err != nil {
				return err
			}
			if expiration.Before(time.Now()) {
				return fmt.Errorf("invalid expiration date: (%s) has already passed", *expires)
			}
		}

		return upload(ctx, src, dst, expiration, []byte(*metadata), *progress)
	}

	// if downloading
	if dst.IsLocal() {
		return download(ctx, src, dst, *progress)
	}

	// if copying from one remote location to another
	return copyObject(ctx, src, dst)
}
