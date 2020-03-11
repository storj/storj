// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

var (
	rmEncryptedFlag *bool
)

func init() {
	rmCmd := addCmd(&cobra.Command{
		Use:   "rm sj://BUCKET/KEY",
		Short: "Delete an object",
		RunE:  deleteObject,
		Args:  cobra.ExactArgs(1),
	}, RootCmd)
	rmEncryptedFlag = rmCmd.Flags().Bool("encrypted", false, "if true, treat paths as base64-encoded encrypted paths")
	setBasicFlags(rmCmd.Flags(), "encrypted")
}

func deleteObject(cmd *cobra.Command, args []string) error {
	ctx, _ := withTelemetry(cmd)

	if len(args) == 0 {
		return fmt.Errorf("no object specified for deletion")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	project, err := cfg.getProject(ctx, *rmEncryptedFlag)
	if err != nil {
		return err
	}
	defer closeProject(project)

	if _, err = project.DeleteObject(ctx, dst.Bucket(), dst.Path()); err != nil {
		return convertError(err, dst)
	}

	fmt.Printf("Deleted %s\n", dst)

	return nil
}
