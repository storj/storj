// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mb sj://BUCKET",
		Short: "Create a new bucket",
		RunE:  makeBucket,
		Args:  cobra.ExactArgs(1),
	}, RootCmd)
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx, _ := withTelemetry(cmd)

	if len(args) == 0 {
		return fmt.Errorf("no bucket specified for creation")
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

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	if _, err := project.CreateBucket(ctx, dst.Bucket()); err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", dst.Bucket())

	return nil
}
