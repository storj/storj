// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/uplink"
)

var (
	rbForceFlag *bool
)

func init() {
	rbCmd := addCmd(&cobra.Command{
		Use:   "rb sj://BUCKET",
		Short: "Remove an empty bucket",
		RunE:  deleteBucket,
		Args:  cobra.ExactArgs(1),
	}, RootCmd)
	rbForceFlag = rbCmd.Flags().Bool("force", false, "if true, empties the bucket of objects first")
	setBasicFlags(rbCmd.Flags(), "force")
}

func deleteBucket(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := withTelemetry(cmd)

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

	project, err := cfg.getProject(ctx, true)
	if err != nil {
		return convertError(err, dst)
	}
	defer closeProject(project)

	successes := 0
	failures := 0
	defer func() {
		if successes > 0 {
			fmt.Printf("(%d) files from bucket %s have been deleted\n", successes, dst.Bucket())
		}
		if failures > 0 {
			fmt.Printf("(%d) files from bucket %s have NOT been deleted\n", failures, dst.Bucket())
		}
		if err == nil && failures == 0 {
			fmt.Printf("Bucket %s have been deleted\n", dst.Bucket())
		} else {
			fmt.Printf("Bucket %s have NOT been deleted\n", dst.Bucket())
		}
	}()

	if *rbForceFlag {
		// TODO add retry in case of failures
		objects := project.ListObjects(ctx, dst.Bucket(), &uplink.ListObjectsOptions{
			Recursive: true,
		})

		for objects.Next() {
			object := objects.Item()
			path := object.Key
			_, err := project.DeleteObject(ctx, dst.Bucket(), path)
			if err != nil {
				fmt.Printf("failed to delete encrypted object, cannot empty bucket %q: %+v\n", dst.Bucket(), err)
				failures++
				continue
			}
			successes++
			if successes%10 == 0 {
				fmt.Printf("(%d) files from bucket %s have been deleted\n", successes, dst.Bucket())
			}
		}
		if err := objects.Err(); err != nil {
			return err
		}
	}

	if failures == 0 {
		if _, err := project.DeleteBucket(ctx, dst.Bucket()); err != nil {
			return convertError(err, dst)
		}
	}

	return nil
}
