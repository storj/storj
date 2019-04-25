// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mb",
		Short: "Create a new bucket",
		RunE:  makeBucket,
	}, RootCmd)
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

	metainfo, _, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	_, err = metainfo.GetBucket(ctx, dst.Bucket())
	if err == nil {
		return fmt.Errorf("Bucket already exists")
	}
	if !storj.ErrBucketNotFound.Has(err) {
		return err
	}
	_, err = metainfo.CreateBucket(ctx, dst.Bucket(), &storj.Bucket{PathCipher: storj.Cipher(cfg.Enc.PathType)})
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", dst.Bucket())

	return nil
}
