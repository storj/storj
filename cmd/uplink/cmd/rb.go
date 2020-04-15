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

	counter := 0
	defer func() {
		// print number of deleted files no mater if we end with error or without
		if counter > 0 {
			fmt.Printf("Files (%d) from bucket %s deleted\n", counter, dst.Bucket())
		}
		if err == nil {
			fmt.Printf("Bucket %s deleted\n", dst.Bucket())
		}
	}()

	if *rbForceFlag {
		objects := project.ListObjects(ctx, dst.Bucket(), &uplink.ListObjectsOptions{
			Recursive: true,
		})

		for objects.Next() {
			object := objects.Item()
			path := object.Key
			_, err := project.DeleteObject(ctx, dst.Bucket(), path)
			if err != nil {
				return fmt.Errorf("failed to delete encrypted object, cannot empty bucket: %q", path)
			}
			counter++
			if counter%10 == 0 {
				fmt.Printf("Files (%d) from bucket %s deleted\n", counter, dst.Bucket())
			}
		}
		if err := objects.Err(); err != nil {
			return err
		}
	}

	if _, err := project.DeleteBucket(ctx, dst.Bucket()); err != nil {
		return convertError(err, dst)
	}

	return nil
}
