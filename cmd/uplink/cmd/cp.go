// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"
)

var (
	cpCfg Config
	cpCmd = &cobra.Command{
		Use:   "cp",
		Short: "Copies a local file or Storj object to another location locally or in Storj",
		RunE:  copy,
	}
)

func init() {
	RootCmd.AddCommand(cpCmd)
	cfgstruct.Bind(cpCmd.Flags(), &cpCfg, cfgstruct.ConfDir(defaultConfDir))
	cpCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func copy(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	identity, err := cpCfg.Load()
	if err != nil {
		return err
	}

	bs, err := cpCfg.GetBucketStore(ctx, identity)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		fmt.Println("No file specified for copy")
		return nil
	}

	if len(args) == 1 {
		fmt.Println("No destination specified")
		return nil
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
		if u1.Scheme == "" {
			fmt.Println("Invalid destination")
			return nil
		}

		// if object name not specified, default to filename
		if u1.Path == "" || u1.Path == "/" {
			u1.Path = filepath.Base(args[0])
		}

		f, err := os.Open(args[0])
		if err != nil {
			return err
		}

		defer utils.LogClose(f)

		o, err := bs.GetObjectStore(ctx, u1.Host)
		if err != nil {
			return err
		}

		meta := objects.SerializableMeta{}
		expTime := time.Time{}

		_, err = o.Put(ctx, paths.New(u1.Path), f, meta, expTime)
		if err != nil {
			return err
		}

		fmt.Printf("Created: %s\n", u1.Path)

		return nil
	}

	o, err := bs.GetObjectStore(ctx, u0.Host)
	if err != nil {
		return err
	}

	// if downloading
	if u1.Scheme == "" {
		f, err := os.Create(args[1])
		if err != nil {
			return err
		}

		defer utils.LogClose(f)

		rr, _, err := o.Get(ctx, paths.New(u0.Path))
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

		fmt.Printf("Downloaded %s to %s\n", u0.Path, args[1])

		return nil
	}

	// if copying from one remote location to another
	rr, _, err := o.Get(ctx, paths.New(u0.Path))
	if err != nil {
		return err
	}
	defer utils.LogClose(rr)

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	o, err = bs.GetObjectStore(ctx, u1.Host)
	if err != nil {
		return err
	}

	meta := objects.SerializableMeta{}
	expTime := time.Time{}

	// if destination object name not specified, default to source object name
	if u1.Path == "" || u1.Path == "/" {
		u1.Path = u0.Path
	}

	_, err = o.Put(ctx, paths.New(u1.Path), r, meta, expTime)
	if err != nil {
		return err
	}

	fmt.Printf("%s copied to %s\n", u0.Host+u0.Path, u1.Host+u1.Path)

	return nil
}
