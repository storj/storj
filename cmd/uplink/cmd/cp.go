// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"
)

var (
	cpCfg Config
	cpCmd = &cobra.Command{
		Use:   "cp",
		Short: "Copies a local file or Storj object to another location locally or in Storj",
		RunE:  copyMain,
	}
)

func init() {
	RootCmd.AddCommand(cpCmd)
	cfgstruct.Bind(cpCmd.Flags(), &cpCfg, cfgstruct.ConfDir(defaultConfDir))
	cpCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

// upload uploads args[0] from local machine to s3 compatible object args[1]
func upload(ctx context.Context, bs buckets.Store, srcFile string, destObj *url.URL) error {
	if destObj.Scheme == "" {
		fmt.Println("Invalid destination")
		return nil
	}

	// if object name not specified, default to filename
	if destObj.Path == "" || destObj.Path == "/" {
		destObj.Path = filepath.Base(srcFile)
	}

	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	defer utils.LogClose(f)

	o, err := bs.GetObjectStore(ctx, destObj.Host)
	if err != nil {
		return err
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	_, err = o.Put(ctx, paths.New(destObj.Path), f, meta, expTime)
	if err != nil {
		return err
	}

	fmt.Printf("Created: %s\n", destObj.Path)

	return nil
}

// download downloads s3 compatible object args[0] to args[1] on local machine
func download(ctx context.Context, bs buckets.Store, srcObj *url.URL, destFile string) error {
	if srcObj.Scheme == "" {
		fmt.Println("Invalid source")
		return nil
	}

	o, err := bs.GetObjectStore(ctx, srcObj.Host)
	if err != nil {
		return err
	}

	f, err := os.Create(destFile)
	if err != nil {
		return err
	}

	defer utils.LogClose(f)

	rr, _, err := o.Get(ctx, paths.New(srcObj.Path))
	if err != nil {
		return err
	}
	defer utils.LogClose(rr)

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded %s to %s\n", srcObj.Path, destFile)

	return nil
}

// copy copies s3 compatible object args[0] to s3 compatible object args[1]
func copy(ctx context.Context, bs buckets.Store, srcObj *url.URL, destObj *url.URL) error {
	o, err := bs.GetObjectStore(ctx, srcObj.Host)
	if err != nil {
		return err
	}

	rr, _, err := o.Get(ctx, paths.New(srcObj.Path))
	if err != nil {
		return err
	}
	defer utils.LogClose(rr)

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	if destObj.Host != srcObj.Host {
		o, err = bs.GetObjectStore(ctx, destObj.Host)
		if err != nil {
			return err
		}
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	// if destination object name not specified, default to source object name
	if destObj.Path == "" || destObj.Path == "/" {
		destObj.Path = srcObj.Path
	}

	_, err = o.Put(ctx, paths.New(destObj.Path), r, meta, expTime)
	if err != nil {
		return err
	}

	fmt.Printf("%s copied to %s\n", srcObj.Host+srcObj.Path, destObj.Host+destObj.Path)

	return nil
}

// copyMain is the function executed when cpCmd is called
func copyMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		fmt.Println("No object specified for copy")
		return nil
	}
	if len(args) == 1 {
		fmt.Println("No destination specified")
		return nil
	}

	ctx := process.Ctx(cmd)

	identity, err := cpCfg.Load()
	if err != nil {
		return err
	}

	bs, err := cpCfg.GetBucketStore(ctx, identity)
	if err != nil {
		return err
	}

	u0, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}

	u1, err := utils.ParseURL(args[1])
	if err != nil {
		return err
	}

	// if uploading
	if u0.Scheme == "" {
		return upload(ctx, bs, args[0], u1)
	}

	// if downloading
	if u1.Scheme == "" {
		return download(ctx, bs, u0, args[1])
	}

	// if copying from one remote location to another
	return copy(ctx, bs, u0, u1)
}
