// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/uplink"
	"storj.io/uplink/private/multipart"
)

var (
	lsRecursiveFlag *bool
	lsEncryptedFlag *bool
	lsPendingFlag   *bool
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
	lsPendingFlag = lsCmd.Flags().Bool("pending", false, "if true, list pending objects")

	setBasicFlags(lsCmd.Flags(), "recursive", "encrypted", "pending")
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

		if !strings.HasSuffix(args[0], "/") && src.Path() != "" {
			err = listObject(ctx, project, src.Bucket(), src.Path())
			if err != nil && !errors.Is(err, uplink.ErrObjectNotFound) {
				return convertError(err, src)
			}
		}
		err = listObjects(ctx, project, src.Bucket(), src.Path(), false)
		return convertError(err, src)
	}
	noBuckets := true

	buckets := project.ListBuckets(ctx, nil)
	for buckets.Next() {
		bucket := buckets.Item()

		if !*lsPendingFlag {
			fmt.Println("BKT", formatTime(bucket.Created), bucket.Name)
		}
		if *lsRecursiveFlag {
			if err := listObjectsFromBucket(ctx, project, bucket.Name); err != nil {
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

func listObjectsFromBucket(ctx context.Context, project *uplink.Project, bucket string) error {
	return listObjects(ctx, project, bucket, "", true)
}

func listObject(ctx context.Context, project *uplink.Project, bucket, path string) error {
	if *lsPendingFlag {
		return listPendingObject(ctx, project, bucket, path)
	}
	object, err := project.StatObject(ctx, bucket, path)
	if err != nil {
		return err
	}
	fmt.Printf("%v %v %12v %v\n", "OBJ", formatTime(object.System.Created), object.System.ContentLength, path)
	return nil
}

func listObjects(ctx context.Context, project *uplink.Project, bucket, prefix string, prependBucket bool) error {
	// TODO force adding slash at the end because fpath is removing it,
	// most probably should be fixed in storj/common
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var objects *uplink.ObjectIterator
	if *lsPendingFlag {
		return listPendingObjects(ctx, project, bucket, prefix, prependBucket)
	}

	objects = project.ListObjects(ctx, bucket, &uplink.ListObjectsOptions{
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

func listPendingObject(ctx context.Context, project *uplink.Project, bucket, path string) error {
	objects := multipart.ListPendingObjectStreams(ctx, project, bucket, path, &multipart.ListMultipartUploadsOptions{
		System: true,
		Custom: true,
	})

	for objects.Next() {
		object := objects.Item()
		path := object.Key
		fmt.Printf("%v %v %12v %v\n", "OBJ", formatTime(object.System.Created), object.System.ContentLength, path)
	}
	return objects.Err()
}

func listPendingObjects(ctx context.Context, project *uplink.Project, bucket, prefix string, prependBucket bool) error {
	// TODO force adding slash at the end because fpath is removing it,
	// most probably should be fixed in storj/common
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	objects := multipart.ListMultipartUploads(ctx, project, bucket, &multipart.ListMultipartUploadsOptions{
		Prefix:    prefix,
		Cursor:    "",
		Recursive: *lsRecursiveFlag,

		System: true,
		Custom: true,
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

	return objects.Err()
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}
