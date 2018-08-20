// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/minio/minio/pkg/hash"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	cpCfg Config
	cpCmd = &cobra.Command{
		Use:   "cp",
		Short: "A brief description of your command",
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

	if len(args) == 0 {
		return errs.New("No file specified for copy")
	}

	if len(args) == 1 {
		return errs.New("No destination specified")
	}

	so, err := getStorjObjects(ctx, cpCfg)
	if err != nil {
		return err
	}

	u0, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	u1, err := url.Parse(args[1])
	if err != nil {
		return err
	}

	// if uploading
	if u0.Scheme == "" {
		f, err := os.Open(args[0])

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		fr, err := hash.NewReader(f, fi.Size(), "", "")
		if err != nil {
			return err
		}

		defer f.Close()

		oi, err := so.PutObject(ctx, u1.Host, u1.Path, fr, nil)
		if err != nil {
			return err
		}

		fmt.Println("Bucket:", oi.Bucket)
		fmt.Println("Object:", oi.Name)

		return nil
	}

	srcInfo, err := so.GetObjectInfo(ctx, u0.Host, u0.Path)
	if err != nil {
		return err
	}

	// if downloading
	if u1.Scheme == "" {
		f, err := os.Create(args[1])
		if err != nil {
			return err
		}

		defer f.Close()

		err = so.GetObject(ctx, srcInfo.Bucket, srcInfo.Name, 0, srcInfo.Size, f, srcInfo.ETag)
		if err != nil {
			return err
		}

		fmt.Printf("Downloaded %s to %s", srcInfo.Bucket+srcInfo.Name, args[1])

		return nil
	}

	// if copying from one remote location to another
	objInfo, err := so.CopyObject(ctx, u0.Host, u0.Path, u1.Host, u1.Path, srcInfo)
	if err != nil {
		return err
	}

	fmt.Println(objInfo.Bucket)
	fmt.Println(objInfo.Name)

	return nil
}
