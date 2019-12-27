// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/common/storj"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "rb",
		Short: "Remove an empty bucket",
		RunE:  deleteBucket,
	}, RootCmd)
}

func deleteBucket(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("no bucket specified for deletion")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	if dst.Path() != "" {
		return fmt.Errorf("nested buckets not supported, use format sj://bucket/")
	}

	project, bucket, err := cfg.GetProjectAndBucket(ctx, dst.Bucket())
	if err != nil {
		return convertError(err, dst)
	}
	defer closeProjectAndBucket(project, bucket)

	list, err := bucket.ListObjects(ctx, &storj.ListOptions{Direction: storj.After, Recursive: true, Limit: 1})
	if err != nil {
		return convertError(err, dst)
	}

	if len(list.Items) > 0 {
		return fmt.Errorf("bucket not empty: %s", dst.Bucket())
	}

	err = project.DeleteBucket(ctx, dst.Bucket())
	if err != nil {
		return convertError(err, dst)
	}

	fmt.Printf("Bucket %s deleted\n", dst.Bucket())

	return nil
}
