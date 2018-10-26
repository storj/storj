// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
)

var (
	recursiveFlag *bool
)

func init() {
	lsCmd := addCmd(&cobra.Command{
		Use:   "ls",
		Short: "List objects and prefixes or all buckets",
		RunE:  list,
	}, CLICmd)
	recursiveFlag = lsCmd.Flags().Bool("recursive", false, "if true, list recursively")
}

func list(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		src, err := fpath.New(args[0])
		if err != nil {
			return err
		}

		if src.IsLocal() {
			return fmt.Errorf("No bucket specified, use format sj://bucket/")
		}

		return listFiles(ctx, bs, src, false)
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
					prefix, err := fpath.New(fmt.Sprintf("sj://%s/", bucket.Bucket))
					if err != nil {
						return err
					}
					err = listFiles(ctx, bs, prefix, true)
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
		fmt.Println("No buckets")
	}

	return nil
}

func listFiles(ctx context.Context, bs buckets.Store, prefix fpath.FPath, prependBucket bool) error {
	o, err := bs.GetObjectStore(ctx, prefix.Bucket())
	if err != nil {
		return err
	}

	startAfter := ""

	for {
		items, more, err := o.List(ctx, prefix.Path(), startAfter, "", *recursiveFlag, 0, meta.Modified|meta.Size)
		if err != nil {
			return err
		}

		for _, object := range items {
			path := object.Path
			if prependBucket {
				path = fmt.Sprintf("%s/%s", prefix.Bucket(), path)
			}
			if object.IsPrefix {
				fmt.Println("PRE", path)
			} else {
				fmt.Printf("%v %v %12v %v\n", "OBJ", formatTime(object.Meta.Modified), object.Meta.Size, path)
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
