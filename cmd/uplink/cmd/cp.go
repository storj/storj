// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	progressbar "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
	"storj.io/storj/uplink/setup"
)

var (
	progress *bool
	expires  *string
)

func init() {
	cpCmd := addCmd(&cobra.Command{
		Use:   "cp",
		Short: "Copies a local file or Storj object to another location locally or in Storj",
		RunE:  copyMain,
	}, RootCmd)
	progress = cpCmd.Flags().Bool("progress", true, "if true, show progress")
	expires = cpCmd.Flags().String("expires", "", "optional expiration date of an object. Please use format (yyyy-mm-ddThh:mm:ssZhh:mm)")
}

// upload transfers src from local machine to s3 compatible object dst
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
			return fmt.Errorf("Invalid expiration date: (%s) has already passed", *expires)
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

	access, err := setup.LoadEncryptionAccess(ctx, cfg.Enc)
	if err != nil {
		return err
	}

	project, bucket, err := cfg.GetProjectAndBucket(ctx, dst.Bucket(), access)
	if err != nil {
		return err
	}

	defer closeProjectAndBucket(project, bucket)

	reader := io.Reader(file)
	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.New64(fileInfo.Size()).SetUnits(progressbar.U_BYTES).SetWidth(80)
		bar.ShowSpeed = true
		bar.Start()
		reader = bar.NewProxyReader(reader)
	}

	opts := &libuplink.UploadOptions{}

	if *expires != "" {
		opts.Expires = expiration.UTC()
	}

	opts.Volatile.RedundancyScheme = cfg.GetRedundancyScheme()
	opts.Volatile.EncryptionParameters = cfg.GetEncryptionScheme().ToEncryptionParameters()

	if err := bucket.UploadObject(ctx, dst.Path(), reader, opts); err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("Created %s\n", dst.String())

	return nil
}

// download transfers s3 compatible object src to dst on local machine
func download(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if !dst.IsLocal() {
		return fmt.Errorf("destination must be local path: %s", dst)
	}

	access, err := setup.LoadEncryptionAccess(ctx, cfg.Enc)
	if err != nil {
		return err
	}

	project, bucket, err := cfg.GetProjectAndBucket(ctx, src.Bucket(), access)
	if err != nil {
		return err
	}

	defer closeProjectAndBucket(project, bucket)

	object, err := bucket.OpenObject(ctx, src.Path())
	if err != nil {
		return convertError(err, src)
	}

	rc, err := object.DownloadRange(ctx, 0, object.Meta.Size)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, rc.Close()) }()

	var bar *progressbar.ProgressBar
	var reader io.ReadCloser
	if showProgress {
		bar = progressbar.New64(object.Meta.Size).SetUnits(progressbar.U_BYTES).SetWidth(80)
		bar.ShowSpeed = true
		bar.Start()
		reader = bar.NewProxyReader(rc)
	} else {
		reader = rc
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
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	if dst.Base() != "-" {
		fmt.Printf("Downloaded %s to %s\n", src.String(), dst.String())
	}

	return nil
}

// copy copies s3 compatible object src to s3 compatible object dst
func copyObject(ctx context.Context, src fpath.FPath, dst fpath.FPath) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	access, err := setup.LoadEncryptionAccess(ctx, cfg.Enc)
	if err != nil {
		return err
	}

	project, bucket, err := cfg.GetProjectAndBucket(ctx, dst.Bucket(), access)
	if err != nil {
		return err
	}

	defer closeProjectAndBucket(project, bucket)

	object, err := bucket.OpenObject(ctx, src.Path())
	if err != nil {
		return convertError(err, src)
	}

	rc, err := object.DownloadRange(ctx, 0, object.Meta.Size)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, rc.Close()) }()

	var bar *progressbar.ProgressBar
	var reader io.Reader
	if *progress {
		bar = progressbar.New64(object.Meta.Size).SetUnits(progressbar.U_BYTES)
		bar.Start()
		reader = bar.NewProxyReader(rc)
	} else {
		reader = rc
	}

	// if destination object name not specified, default to source object name
	if strings.HasSuffix(dst.Path(), "/") {
		dst = dst.Join(src.Base())
	}

	opts := &libuplink.UploadOptions{
		Expires:     object.Meta.Expires,
		ContentType: object.Meta.ContentType,
		Metadata:    object.Meta.Metadata,
	}
	opts.Volatile.RedundancyScheme = cfg.GetRedundancyScheme()
	opts.Volatile.EncryptionParameters = cfg.GetEncryptionScheme().ToEncryptionParameters()
	err = bucket.UploadObject(ctx, dst.Path(), reader, opts)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("%s copied to %s\n", src.String(), dst.String())

	return nil
}

// copyMain is the function executed when cpCmd is called
func copyMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No object specified for copy")
	}
	if len(args) == 1 {
		return fmt.Errorf("No destination specified")
	}

	ctx := process.Ctx(cmd)

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
		return errors.New("At least one of the source or the desination must be a Storj URL")
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
