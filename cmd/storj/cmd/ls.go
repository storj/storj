// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/utils"
)

var (
	lsCfg Config
	lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List objects and prefixes or all buckets",
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

	identity, err := lsCfg.Load()
	if err != nil {
		return err
	}

	bs, err := lsCfg.GetBucketStore(ctx, identity)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		startAfter := ""
		var items []buckets.ListItem
		for {
			moreItems, more, err := bs.List(ctx, startAfter, "", 0)
			if err != nil {
				return err
			}
			items = append(items, moreItems...)
			if !more {
				break
			}
			startAfter = moreItems[len(moreItems)-1].Bucket
		}

		for _, bucket := range items {
			fmt.Println(bucket.Meta.Created, bucket.Bucket)
		}

		return nil
	}

	u, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}

	o, err := bs.GetObjectStore(ctx, u.Host)
	if err != nil {
		return err
	}

	startAfter := paths.New("")

	for {
		items, more, err := o.List(ctx, paths.New(u.Path), startAfter, nil, true, 1000, meta.All)
		if err != nil {
			return err
		}

		for _, object := range items {
			fmt.Println(object.Meta.Modified, object.Path)
		}

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path[len(paths.New(u.Path)):]
	}

	return nil
}
