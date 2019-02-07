// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/metainfo"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

func init() {
	statCmd := addCmd(&cobra.Command{
		Use:   "stat",
		Short: "stat a Storj object",
		RunE:  cmdStat,
	}, RootCmd)
}

// copyMain is the function executed when cpCmd is called
func statObject(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No object specified for stat")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("No bucket specified, use format sj://bucket/")
	}

	identity, err := cfg.Identity.Load()
	if err != nil {
		return err
	}

	metainfoClient := New(transport.NewClient(identity), setupCfg.SatelliteAddr)
	
	// Stat will return the health of a specific path
	resp, err := metainfo.Stat(ctx, src.Bucket(), src.Path()) {
	if err != nil {
		return convertError(err, src)
	}

	fmt.Printf("%+v\n", resp)

	return nil
}
