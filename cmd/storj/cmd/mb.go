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
	mbCfg Config
	mbCmd = &cobra.Command{
		Use:   "mb",
		Short: "Create a bucket",
		RunE:  makeBucket,
	}
)

func init() {
	RootCmd.AddCommand(mbCmd)
	cfgstruct.Bind(mbCmd.Flags(), &mbCfg, cfgstruct.ConfDir(defaultConfDir))
	mbCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return errs.New("No bucket specified for creation")
	}

	so, err := getStorjObjects(ctx, mbCfg)
	if err != nil {
		return err
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	err = so.MakeBucketWithLocation(ctx, u.Host, "")
	if err != nil {
		return err
	}

	fmt.Printf("Created %s", u.Host)

	return nil
}
