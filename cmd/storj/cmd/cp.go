// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

	storjCtx, cancel := context.WithCancel(ctx)
	/* create a signal of type os.Signal */
	c := make(chan os.Signal, 0x01)

	/* register for the os signals */
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		signal.Stop(c)
		cancel()
	}()

	go func() error {
		select {
		case <-c:
			// Handle any storj cli specific clean up here...
			cancel()
			return storjCtx.Err()
		}
	}()

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

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	if u.Scheme == "" {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		fr, err := hash.NewReader(f, fi.Size(), "", "")
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		u, err = url.Parse(args[1])
		if err != nil {
			return err
		}

		oi, err := so.PutObject(ctx, u.Host, u.Path, fr, nil)
		if err != nil {
			return err
		}

		fmt.Println("Bucket:", oi.Bucket)
		fmt.Println("Object:", oi.Name)

		return nil
	}

	oi, err := so.GetObjectInfo(ctx, u.Host, u.Path)
	if err != nil {
		return err
	}

	f, err := os.Create(args[1])
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	err = so.GetObject(ctx, oi.Bucket, oi.Name, 0, oi.Size, f, oi.ETag)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded %s to %s", oi.Bucket+oi.Name, args[1])

	return nil
}
