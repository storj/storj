// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/utils"
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

		return listFiles(ctx, bs, u, false)
	}

	startAfter := ""
	noBuckets := true

	for {
		items, more, err := bs.List(ctx, startAfter, "", 0)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			noBuckets = false
			for _, bucket := range items {
				fmt.Println("BKT", formatTime(bucket.Meta.Created), bucket.Bucket)
				if *recursiveFlag {
					err := listFiles(ctx, bs, &url.URL{Host: bucket.Bucket, Path: "/"}, true)
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

func listFiles(ctx context.Context, bs buckets.Store, u *url.URL, prependBucket bool) error {
	o, err := bs.GetObjectStore(ctx, u.Host)
	if err != nil {
		return err
	}

	startAfter := paths.New("")

	for {
		items, more, err := o.List(ctx, paths.New(u.Path), startAfter, nil, *recursiveFlag, 0, meta.Modified)
		if err != nil {
			return err
		}

		for _, object := range items {
			path := object.Path.String()
			if prependBucket {
				path = fmt.Sprintf("%s/%s", u.Host, path)
			}
			if object.IsPrefix {
				fmt.Println("PRE", path+"/")
			} else {
				fmt.Println("OBJ", formatTime(object.Meta.Modified), path)
			}
		}

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path
	}

	return nil
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}
