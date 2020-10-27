// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
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

	defer func() {
		if err != nil {
			fmt.Printf("Bucket %s has NOT been deleted\n %+v", dst.Bucket(), err.Error())
		} else {
			fmt.Printf("Bucket %s has been deleted\n", dst.Bucket())
		}
	}()

	if *rbForceFlag {
		// TODO: Do we need to have retry here?
		if _, err := project.DeleteBucketWithObjects(ctx, dst.Bucket()); err != nil {
			return convertError(err, dst)
		}

		return nil
	}

	if _, err := project.DeleteBucket(ctx, dst.Bucket()); err != nil {
		return convertError(err, dst)
	}

	return nil
}
