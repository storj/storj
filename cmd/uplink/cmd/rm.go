// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
)

var (
	rmEncryptedFlag *bool
)

func init() {
	rmCmd := addCmd(&cobra.Command{
		Use:   "rm",
		Short: "Delete an object",
		RunE:  deleteObject,
	}, RootCmd)
	rmEncryptedFlag = rmCmd.Flags().Bool("encrypted", false, "if true, treat paths as base64-encoded encrypted paths")
}

func deleteObject(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

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

	project, err := cfg.GetProject(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := project.Close(); err != nil {
			fmt.Printf("error closing project: %+v\n", err)
		}
	}()

	scope, err := cfg.GetScope()
	if err != nil {
		return err
	}

	access := scope.EncryptionAccess
	if *rmEncryptedFlag {
		access = libuplink.NewEncryptionAccessWithDefaultKey(storj.Key{})
		access.Store().EncryptionBypass = true
	}

	bucket, err := project.OpenBucket(ctx, dst.Bucket(), access)
	if err != nil {
		return err
	}
	defer func() {
		if err := bucket.Close(); err != nil {
			fmt.Printf("error closing bucket: %+v\n", err)
		}
	}()

	if err = bucket.DeleteObject(ctx, dst.Path()); err != nil {
		return convertError(err, dst)
	}

	if err := project.Close(); err != nil {
		return err
	}

	fmt.Printf("Deleted %s\n", dst)

	return nil
}
