// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testuplink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"
)

// upload transfers src from local machine to s3 compatible object dst
func upload(ctx context.Context, bs buckets.Store, src fpath.FPath, dst fpath.FPath) error {
	if !src.IsLocal() {
		return fmt.Errorf("source must be local path: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	// if object name not specified, default to filename
	if strings.HasSuffix(dst.Path(), "/") || dst.Path() == "" {
		dst = dst.Join(src.Base())
	}

	var f *os.File
	var err error
	if src.Base() == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(src.Path())
		if err != nil {
			return err
		}
		defer utils.LogClose(f)
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return fmt.Errorf("source cannot be a directory: %s", src)
	}

	o, err := bs.GetObjectStore(ctx, dst.Bucket())
	if err != nil {
		return err
	}

	r := io.Reader(f)

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	_, err = o.Put(ctx, dst.Path(), r, meta, expTime)
	if err != nil {
		return err
	}

	fmt.Printf("Created %s\n", dst.String())

	return nil
}

// download transfers s3 compatible object src to dst on local machine
func download(ctx context.Context, bs buckets.Store, src fpath.FPath, dst fpath.FPath) error {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if !dst.IsLocal() {
		return fmt.Errorf("destination must be local path: %s", dst)
	}

	o, err := bs.GetObjectStore(ctx, src.Bucket())
	if err != nil {
		return err
	}

	rr, _, err := o.Get(ctx, src.Path())
	if err != nil {
		return err
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	if fi, err := os.Stat(dst.Path()); err == nil && fi.IsDir() {
		dst = dst.Join((src.Base()))
	}

	var f *os.File
	if dst.Base() == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(dst.Path())
		if err != nil {
			return err
		}
		defer utils.LogClose(f)
	}

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	if dst.Base() != "-" {
		fmt.Printf("Downloaded %s to %s\n", src.String(), dst.String())
	}

	return nil
}

// copy copies s3 compatible object src to s3 compatible object dst
func copy(ctx context.Context, bs buckets.Store, src fpath.FPath, dst fpath.FPath) error {
	if src.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", src)
	}

	if dst.IsLocal() {
		return fmt.Errorf("destination must be Storj URL: %s", dst)
	}

	o, err := bs.GetObjectStore(ctx, src.Bucket())
	if err != nil {
		return err
	}

	rr, _, err := o.Get(ctx, src.Path())
	if err != nil {
		return err
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	if dst.Bucket() != src.Bucket() {
		o, err = bs.GetObjectStore(ctx, dst.Bucket())
		if err != nil {
			return err
		}
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	// if destination object name not specified, default to source object name
	if strings.HasSuffix(dst.Path(), "/") {
		dst = dst.Join(src.Base())
	}

	_, err = o.Put(ctx, dst.Path(), r, meta, expTime)
	if err != nil {
		return err
	}

	fmt.Printf("%s copied to %s\n", src.String(), dst.String())

	return nil
}

// CP is a test of the CP uplink command
func CP(ctx context.Context, uplink *testplanet.Node, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No object specified for copy")
	}
	if len(args) == 1 {
		return fmt.Errorf("No destination specified")
	}

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	dst, err := fpath.New(args[1])
	if err != nil {
		return err
	}

	bs, err := uplink.Client.GetBucketStore(ctx, uplink.Identity)
	if err != nil {
		return err
	}

	// if both local
	if src.IsLocal() && dst.IsLocal() {
		return errors.New("At least one of the source or the desination must be a Storj URL")
	}

	// if uploading
	if src.IsLocal() {
		return upload(ctx, bs, src, dst)
	}

	// if downloading
	if dst.IsLocal() {
		return download(ctx, bs, src, dst)
	}

	// if copying from one remote location to another
	return copy(ctx, bs, src, dst)
}
