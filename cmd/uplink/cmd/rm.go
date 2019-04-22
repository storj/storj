// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "rm",
		Short: "Delete an object",
		RunE:  deleteObject,
	}, RootCmd)
}

func deleteObject(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No object specified for deletion")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("No bucket specified, use format sj://bucket/")
	}

	var access libuplink.EncryptionAccess
	copy(access.Key[:], []byte(cfg.Enc.Key))

	project, bucket, err := cfg.GetProjectAndBucket(ctx, dst.Bucket(), access)
	if err != nil {
		return convertError(err, dst)
	}

	defer func() {
		if err := bucket.Close(); err != nil {
			fmt.Printf("error closing bucket: %+v\n", err)
		}

		if err := project.Close(); err != nil {
			fmt.Printf("error closing project: %+v\n", err)
		}
	}()

	err = bucket.DeleteObject(ctx, dst.Path())
	if err != nil {
		return convertError(err, dst)
	}

	fmt.Printf("Deleted %s\n", dst)

	return nil
}
