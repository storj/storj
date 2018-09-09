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

	so, err := getStorjObjects(ctx, lsCfg)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		bi, err := so.ListBuckets(ctx)
		if err != nil {
			return err
		}

		for _, bucket := range bi {
			fmt.Println(bucket.Created, bucket.Name)
		}

		return nil
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	marker := ""

	for {
		oi, err := so.ListObjects(ctx, u.Host, u.Path, marker, "", 1000)
		if err != nil {
			return err
		}

		for _, object := range oi.Objects {
			fmt.Println(object.Name)
		}

		for _, prefix := range oi.Prefixes {
			fmt.Println(prefix)
		}

		if !oi.IsTruncated {
			break
		}
		marker = oi.NextMarker
	}

	return nil
}
