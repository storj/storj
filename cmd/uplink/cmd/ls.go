// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/uplink"
)

var (
	lsRecursiveFlag *bool
	lsEncryptedFlag *bool
)

func init() {
	lsCmd := addCmd(&cobra.Command{
		Use:   "ls [sj://BUCKET[/PREFIX]]",
		Short: "List objects and prefixes or all buckets",
		RunE:  list,
		Args:  cobra.MaximumNArgs(1),
	}, RootCmd)
	lsRecursiveFlag = lsCmd.Flags().Bool("recursive", false, "if true, list recursively")
	lsEncryptedFlag = lsCmd.Flags().Bool("encrypted", false, "if true, show paths as base64-encoded encrypted paths")

	setBasicFlags(lsCmd.Flags(), "recursive", "encrypted")
}

func list(cmd *cobra.Command, args []string) error {
	ctx, _ := withTelemetry(cmd)

	project, err := cfg.getProject(ctx, *lsEncryptedFlag)
	if err != nil {
		return err
	}
	defer closeProject(project)

	// list objects
	if len(args) > 0 {
		src, err := fpath.New(args[0])
		if err != nil {
			return err
		}

		if src.IsLocal() {
			return fmt.Errorf("no bucket specified, use format sj://bucket/")
		}

		err = listFiles(ctx, project, src.Bucket(), src.Path(), false)
		return convertError(err, src)
	}

	noBuckets := true

	buckets := project.ListBuckets(ctx, nil)
	for buckets.Next() {
		bucket := buckets.Item()

		fmt.Println("BKT", formatTime(bucket.Created), bucket.Name)
		if *lsRecursiveFlag {
			if err := listFilesFromBucket(ctx, project, bucket.Name); err != nil {
				return err
			}
		}
		noBuckets = false
	}
	if buckets.Err() != nil {
		return buckets.Err()
	}

	if noBuckets {
		fmt.Println("No buckets")
	}

	return nil
}

func listFilesFromBucket(ctx context.Context, project *uplink.Project, bucket string) error {
	return listFiles(ctx, project, bucket, "", true)
}

func listFiles(ctx context.Context, project *uplink.Project, bucket, prefix string, prependBucket bool) error {
	// TODO force adding slash at the end because fpath is removing it,
	// most probably should be fixed in storj/common
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	objects := project.ListObjects(ctx, bucket, &uplink.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: *lsRecursiveFlag,
		System:    true,
	})

	for objects.Next() {
		object := objects.Item()
		path := object.Key
		if prependBucket {
			path = fmt.Sprintf("%s/%s", bucket, path)
		}
		if object.IsPrefix {
			fmt.Println("PRE", path)
		} else {
			fmt.Printf("%v %v %12v %v\n", "OBJ", formatTime(object.System.Created), object.System.ContentLength, path)
		}
	}
	if objects.Err() != nil {
		return objects.Err()
	}

	return nil
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}
