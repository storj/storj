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
	"time"

	progressbar "github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/uplink"
)

var (
	progress *bool
	expires  *string
	metadata *string
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

	setBasicFlags(cpCmd.Flags(), "progress", "expires", "metadata")
}

// upload transfers src from local machine to s3 compatible object dst.
func upload(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) (err error) {
	if !src.IsLocal() {
		return fmt.Errorf("source must be local path: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

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

	reader := io.Reader(file)
	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.New64(fileInfo.Size())
		reader = bar.NewProxyReader(reader)
		bar.Start()
	}

	var customMetadata uplink.CustomMetadata
	if *metadata != "" {
		err := json.Unmarshal([]byte(*metadata), &customMetadata)
		if err != nil {
			return err
		}

		if err := customMetadata.Verify(); err != nil {
			return err
		}
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

	if bar != nil {
		bar.Finish()
	}
	if err != nil {
		return err
	}

	fmt.Printf("Created %s\n", dst.String())

	return nil
}

// download transfers s3 compatible object src to dst on local machine.
func download(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if !dst.IsLocal() {
		return fmt.Errorf("destination must be local path: %s", dst)
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

	var bar *progressbar.ProgressBar
	var reader io.ReadCloser
	if showProgress {
		info := download.Info()
		bar = progressbar.New64(info.System.ContentLength)
		reader = bar.NewProxyReader(download)
		bar.Start()
	} else {
		reader = download
	}

	if fileInfo, err := os.Stat(dst.Path()); err == nil && fileInfo.IsDir() {
		dst = dst.Join((src.Base()))
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

	_, err = io.Copy(file, reader)
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
		return upload(ctx, src, dst, *progress)
	}

	// if downloading
	if dst.IsLocal() {
		return download(ctx, src, dst, *progress)
	}

	// if copying from one remote location to another
	return copyObject(ctx, src, dst)
}
