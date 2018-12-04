// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	progressbar "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
	"storj.io/storj/pkg/utils"
)

var (
	progress *bool
)

func init() {
	cpCmd := addCmd(&cobra.Command{
		Use:   "cp",
		Short: "Copies a local file or Storj object to another location locally or in Storj",
		RunE:  copyMain,
	}, CLICmd)
	progress = cpCmd.Flags().Bool("progress", true, "if true, show progress")
}

// upload transfers src from local machine to s3 compatible object dst
func upload(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) error {
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
	var err error
	if src.Base() == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(src.Path())
		if err != nil {
			return err
		}
		defer utils.LogClose(file)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("source cannot be a directory: %s", src)
	}

	metainfo, streams, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	create := storj.CreateObject{
		RedundancyScheme: cfg.GetRedundancyScheme(),
		EncryptionScheme: cfg.GetEncryptionScheme(),
	}
	obj, err := metainfo.CreateObject(ctx, dst.Bucket(), dst.Path(), &create)
	if err != nil {
		return convertError(err, dst)
	}

	mutableStream, err := obj.CreateStream(ctx)
	if err != nil {
		return err
	}

	reader := io.Reader(file)
	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.New(int(fileInfo.Size())).SetUnits(progressbar.U_BYTES)
		bar.Start()
		reader = bar.NewProxyReader(reader)
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)
	defer utils.LogClose(upload)
	_, err = io.Copy(upload, reader)
	if err != nil {
		return err
	}

	err = obj.Commit(ctx)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("Created %s\n", dst.String())

	return nil
}

// download transfers s3 compatible object src to dst on local machine
func download(ctx context.Context, src fpath.FPath, dst fpath.FPath, showProgress bool) error {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if !dst.IsLocal() {
		return fmt.Errorf("destination must be local path: %s", dst)
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
	defer utils.LogClose(download)

	var bar *progressbar.ProgressBar
	var reader io.Reader
	if showProgress {
		bar = progressbar.New(int(readOnlyStream.Info().Size)).SetUnits(progressbar.U_BYTES)
		bar.Start()
		reader = bar.NewProxyReader(download)
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
		defer utils.LogClose(file)
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
func copy(ctx context.Context, src fpath.FPath, dst fpath.FPath) error {
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
	defer utils.LogClose(download)

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

	create := storj.CreateObject{
		RedundancyScheme: cfg.GetRedundancyScheme(),
		EncryptionScheme: cfg.GetEncryptionScheme(),
	}
	obj, err := metainfo.CreateObject(ctx, dst.Bucket(), dst.Path(), &create)
	if err != nil {
		return convertError(err, dst)
	}

	mutableStream, err := obj.CreateStream(ctx)
	if err != nil {
		return err
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)
	defer utils.LogClose(upload)
	_, err = io.Copy(upload, reader)
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
