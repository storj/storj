// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	lsCfg Config
	lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "A brief description of your command",
		RunE:  list,
	}
)

func init() {
	RootCmd.AddCommand(lsCmd)
	cfgstruct.Bind(lsCmd.Flags(), &lsCfg, cfgstruct.ConfDir(defaultConfDir))
	lsCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func list(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	storjObjects, err := getStorjObjects(ctx, lsCfg)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		bucketInfo, err := storjObjects.ListBuckets(ctx)
		if err != nil {
			return err
		}

		for _, bucket := range bucketInfo {
			fmt.Println(bucket.Created, bucket.Name)
		}

		return nil
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	objInfo, err := storjObjects.ListObjects(ctx, u.Host, u.Path, "", "", 1000)
	if err != nil {
		return err
	}

	for _, object := range objInfo.Objects {
		fmt.Println(object.Name)
	}

	for _, prefix := range objInfo.Prefixes {
		fmt.Println(prefix)
	}

	return nil
}
