// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/storage"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "rb",
		Short: "Remove an empty bucket",
		RunE:  deleteBucket,
	})
}

func deleteBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for deletion")
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
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return fmt.Errorf("Bucket not found: %s", dst.Bucket())
		}
		return err
	}

	o, err := bs.GetObjectStore(ctx, dst.Bucket())
	if err != nil {
		return err
	}

	items, _, err := o.List(ctx, nil, nil, nil, true, 1, meta.None)
	if err != nil {
		return err
	}

	if len(items) > 0 {
		return fmt.Errorf("Bucket not empty: %s", dst.Bucket())
	}

	err = bs.Delete(ctx, dst.Bucket())
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s deleted\n", dst.Bucket())

	return nil
}
