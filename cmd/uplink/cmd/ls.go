// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/utils"
)

const (
	pagination = 1000
)

var (
	recursiveFlag *bool
)

func init() {
	lsCmd := addCmd(&cobra.Command{
		Use:   "ls",
		Short: "List objects and prefixes or all buckets",
		RunE:  list,
	})
	recursiveFlag = lsCmd.Flags().Bool("recursive", false, "if true, list recursively")
}

func list(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		u, err := utils.ParseURL(args[0])
		if err != nil {
			return err
		}
		if u.Host == "" {
			return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
		}

		return listFiles(ctx, bs, u)
	}

	startAfter := ""
	noBuckets := true

	for {
		items, more, err := bs.List(ctx, startAfter, "", pagination)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			noBuckets = false
			for _, bucket := range items {
				name := bucket.Bucket
				if *recursiveFlag {
					name = fmt.Sprintf("sj://%s", bucket.Bucket)
				}
				fmt.Println("BKT", bucket.Meta.Created, name)
				if *recursiveFlag {
					err := listFiles(ctx, bs, &url.URL{Host: bucket.Bucket, Path: "/"})
					if err != nil {
						return err
					}
				}
			}
		}
		if !more {
			break
		}
		startAfter = items[len(items)-1].Bucket
	}

	if noBuckets {
		return fmt.Errorf("No buckets")
	}

	return nil
}

func listFiles(ctx context.Context, bs buckets.Store, u *url.URL) error {
	o, err := bs.GetObjectStore(ctx, u.Host)
	if err != nil {
		return err
	}

	startAfter := paths.New("")

	for {
		items, more, err := o.List(ctx, paths.New(u.Path), startAfter, nil, *recursiveFlag, pagination, meta.Modified)
		if err != nil {
			return err
		}

		for _, object := range items {
			// TODO: should list be doing this for us?
			path := object.Path.String()
			if *recursiveFlag {
				path = "sj:/" + filepath.Join(fmt.Sprintf("/%s", u.Host), path)
			} else {
				path = filepath.Base(object.Path.String())
			}
			if object.IsPrefix {
				fmt.Println("PRE", path+"/")
			} else {
				fmt.Println("OBJ", object.Meta.Modified, path)
			}
		}

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path
	}

	return nil
}
