// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
)

var (
	lsRecursiveFlag *bool
	lsEncryptedFlag *bool
)

func init() {
	lsCmd := addCmd(&cobra.Command{
		Use:   "ls",
		Short: "List objects and prefixes or all buckets",
		RunE:  list,
	}, RootCmd)
	lsRecursiveFlag = lsCmd.Flags().Bool("recursive", false, "if true, list recursively")
	lsEncryptedFlag = lsCmd.Flags().Bool("encrypted", false, "if true, show paths as base64-encoded encrypted paths")
}

func list(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	project, err := cfg.GetProject(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := project.Close(); err != nil {
			fmt.Printf("error closing project: %+v\n", err)
		}
	}()

	scope, err := cfg.GetScope()
	if err != nil {
		return err
	}

	access := scope.EncryptionAccess
	if *lsEncryptedFlag {
		access = libuplink.NewEncryptionAccessWithDefaultKey(storj.Key{})
		access.Store().EncryptionBypass = true
	}

	// list objects
	if len(args) > 0 {
		src, err := fpath.New(args[0])
		if err != nil {
			return err
		}

		if src.IsLocal() {
			return fmt.Errorf("no bucket specified, use format sj://bucket/")
		}

		bucket, err := project.OpenBucket(ctx, src.Bucket(), access)
		if err != nil {
			return err
		}
		defer func() {
			if err := bucket.Close(); err != nil {
				fmt.Printf("error closing bucket: %+v\n", err)
			}
		}()

		err = listFiles(ctx, bucket, src, false)
		return convertError(err, src)
	}

	noBuckets := true

	// list buckets
	listOpts := storj.BucketListOptions{
		Direction: storj.Forward,
		Cursor:    "",
	}
	for {
		list, err := project.ListBuckets(ctx, &listOpts)
		if err != nil {
			return err
		}
		if len(list.Items) > 0 {
			noBuckets = false
			for _, bucket := range list.Items {
				fmt.Println("BKT", formatTime(bucket.Created), bucket.Name)
				if *lsRecursiveFlag {
					if err := listFilesFromBucket(ctx, project, bucket.Name, access); err != nil {
						return err
					}
				}
			}
		}
		if !list.More {
			break
		}

		listOpts = listOpts.NextPage(list)
	}

	if noBuckets {
		fmt.Println("No buckets")
	}

	return nil
}

func listFilesFromBucket(ctx context.Context, project *libuplink.Project, bucketName string, access *libuplink.EncryptionAccess) error {
	prefix, err := fpath.New(fmt.Sprintf("sj://%s/", bucketName))
	if err != nil {
		return err
	}

	bucket, err := project.OpenBucket(ctx, bucketName, access)
	if err != nil {
		return err
	}
	defer func() {
		if err := bucket.Close(); err != nil {
			fmt.Printf("error closing bucket: %+v\n", err)
		}
	}()

	err = listFiles(ctx, bucket, prefix, true)
	if err != nil {
		return err
	}

	return nil
}

func listFiles(ctx context.Context, bucket *libuplink.Bucket, prefix fpath.FPath, prependBucket bool) error {
	startAfter := ""

	for {
		list, err := bucket.ListObjects(ctx, &storj.ListOptions{
			Direction: storj.After,
			Cursor:    startAfter,
			Prefix:    prefix.Path(),
			Recursive: *lsRecursiveFlag,
		})
		if err != nil {
			return err
		}

		for _, object := range list.Items {
			path := object.Path
			if prependBucket {
				path = fmt.Sprintf("%s/%s", prefix.Bucket(), path)
			}
			if object.IsPrefix {
				fmt.Println("PRE", path)
			} else {
				fmt.Printf("%v %v %12v %v\n", "OBJ", formatTime(object.Modified), object.Size, path)
			}
		}

		if !list.More {
			break
		}

		startAfter = list.Items[len(list.Items)-1].Path
	}

	return nil
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}
