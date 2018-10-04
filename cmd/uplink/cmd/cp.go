// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/objects"
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
	})
	progress = cpCmd.Flags().Bool("progress", true, "if true, show progress")
}

func cleanAbsPath(p string) string {
	prefix := strings.HasSuffix(p, "/")
	p = path.Join("/", p)
	if !strings.HasSuffix(p, "/") && prefix {
		p += "/"
	}
	return p
}

// upload uploads args[0] from local machine to s3 compatible object args[1]
func upload(ctx context.Context, bs buckets.Store, srcFile *fpath.FPath, destObj *fpath.FPath) error {
	var err error
	if destObj.Scheme() != "sj" {
		return fmt.Errorf("Invalid destination, not a Storj Bucket")
	}

	// if object name not specified, default to filename
	if strings.HasSuffix(destObj.Path(), "/") || destObj.Path() == "" {
		destObj.Join(srcFile.Base())
	}

	var f *os.File
	if srcFile.Base() == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(srcFile.Path())
		if err != nil {
			return err
		}
		defer utils.LogClose(f)
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	r := io.Reader(f)
	var bar *pb.ProgressBar
	if *progress {
		bar = pb.New(int(fi.Size())).SetUnits(pb.U_BYTES)
		bar.Start()
		r = bar.NewProxyReader(r)
	}

	o, err := bs.GetObjectStore(ctx, destObj.Bucket())
	if err != nil {
		return err
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	_, err = o.Put(ctx, paths.New(destObj.Path()), r, meta, expTime)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("Created %s\n", destObj.String())

	return nil
}

// download downloads s3 compatible object args[0] to args[1] on local machine
func download(ctx context.Context, bs buckets.Store, srcObj *fpath.FPath, destObj *fpath.FPath) error {
	var err error
	if srcObj.Scheme() != "sj" {
		return fmt.Errorf("Invalid source, not a Storj Bucket")
	}

	o, err := bs.GetObjectStore(ctx, srcObj.Bucket())
	if err != nil {
		return err
	}

	if fi, err := os.Stat(destObj.Path()); err == nil && fi.IsDir() {
		destObj.Join(srcObj.Base())
	}

	var f *os.File
	if destObj.Path() == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(destObj.Path())
		if err != nil {
			return err
		}
		defer utils.LogClose(f)
	}

	rr, _, err := o.Get(ctx, paths.New(srcObj.Path()))
	if err != nil {
		return err
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	var bar *pb.ProgressBar
	if *progress {
		bar = pb.New(int(rr.Size())).SetUnits(pb.U_BYTES)
		bar.Start()
		r = bar.NewProxyReader(r)
	}

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	if destObj.Base() != "-" {
		fmt.Printf("Downloaded %s to %s\n", srcObj.String(), destObj.String())
	}

	return nil
}

// copy copies s3 compatible object args[0] to s3 compatible object args[1]
func copy(ctx context.Context, bs buckets.Store, srcObj *fpath.FPath, destObj *fpath.FPath) error {
	o, err := bs.GetObjectStore(ctx, srcObj.Bucket())
	if err != nil {
		return err
	}

	rr, _, err := o.Get(ctx, paths.New(srcObj.Path()))
	if err != nil {
		return err
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	var bar *pb.ProgressBar
	if *progress {
		bar = pb.New(int(rr.Size())).SetUnits(pb.U_BYTES)
		bar.Start()
		r = bar.NewProxyReader(r)
	}

	if destObj.Bucket() != srcObj.Bucket() {
		o, err = bs.GetObjectStore(ctx, destObj.Bucket())
		if err != nil {
			return err
		}
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	// if destination object name not specified, default to source object name
	if strings.HasSuffix(destObj.Path(), "/") {
		destObj.Join(srcObj.Base())
	}

	_, err = o.Put(ctx, paths.New(destObj.Path()), r, meta, expTime)
	if err != nil {
		return err
	}

	if bar != nil {
		bar.Finish()
	}

	fmt.Printf("%s copied to %s\n", srcObj.String(), destObj.String())

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

	u0, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	u1, err := fpath.New(args[1])
	if err != nil {
		return err
	}

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	// if uploading
	if u0.IsLocal() {
		if u1.Scheme() != "sj" && !u1.IsLocal() {
			return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
		}
		return upload(ctx, bs, &u0, &u1)
	}

	// if downloading
	if u1.IsLocal() {
		if u0.Scheme() != "sj" && !u0.IsLocal() {
			return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
		}
		return download(ctx, bs, &u0, &u1)
	}

	// if copying from one remote location to another
	return copy(ctx, bs, &u0, &u1)
}
