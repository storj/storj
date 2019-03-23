// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	progressbar "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
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

// copy copies s3 compatible object src to s3 compatible object dst
func copy(ctx context.Context, src fpath.FPath, dst fpath.FPath) (err error) {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	metainfo, streams, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	readOnlyStream, err := metainfo.GetObjectStream(ctx, src.Bucket(), src.Path())
	if err != nil {
		return convertError(err, src)
	}

	download := stream.NewDownload(ctx, readOnlyStream, streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	var bar *progressbar.ProgressBar
	var reader io.Reader
	if *progress {
		bar = progressbar.New(int(readOnlyStream.Info().Size)).SetUnits(progressbar.U_BYTES)
		bar.Start()
		reader = bar.NewProxyReader(download)
	} else {
		reader = download
	}

	// if destination object name not specified, default to source object name
	if strings.HasSuffix(dst.Path(), "/") {
		dst = dst.Join(src.Base())
	}

	createInfo := storj.CreateObject{
		RedundancyScheme: cfg.GetRedundancyScheme(),
		EncryptionScheme: cfg.GetEncryptionScheme(),
	}
	obj, err := metainfo.CreateObject(ctx, dst.Bucket(), dst.Path(), &createInfo)
	if err != nil {
		return convertError(err, dst)
	}

	err = uploadStream(ctx, streams, obj, reader)
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
	return copy(ctx, src, dst)
}
