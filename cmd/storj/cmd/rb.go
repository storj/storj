// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	rbCfg Config
	rbCmd = &cobra.Command{
		Use:   "rb",
		Short: "Remove an empty bucket",
		RunE:  deleteBucket,
	}
)

func init() {
	RootCmd.AddCommand(rbCmd)
	cfgstruct.Bind(rbCmd.Flags(), &rbCfg, cfgstruct.ConfDir(defaultConfDir))
	rbCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func deleteBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return errs.New("No bucket specified for deletion")
	}

	so, err := getStorjObjects(ctx, rmCfg)
	if err != nil {
		return err
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	err = so.DeleteBucket(ctx, u.Host)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %s", u.Host)

	return nil
}
