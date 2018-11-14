// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mb",
		Short: "Create a new bucket",
		RunE:  makeBucket,
	}, CLICmd)
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for creation")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("No bucket specified, use format sj://bucket/")
	}

	if dst.Path() != "" {
		return fmt.Errorf("Nested buckets not supported, use format sj://bucket/")
	}

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	_, err = bs.Get(ctx, dst.Bucket())
	if err == nil {
		return fmt.Errorf("Bucket already exists")
	}
	if !storage.ErrKeyNotFound.Has(err) {
		return err
	}
	_, err = bs.Put(ctx, dst.Bucket(), storj.Cipher(cfg.PathEncType))
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", dst.Bucket())

	return nil
}
