// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "stat",
		Short: "stat a Storj object",
		RunE:  statObject,
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

	metainfo, _, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	obj, err := metainfo.GetObject(ctx, dst.Bucket(), dst.Path())
	// identity, err := cfg.Identity.Load()
	if err != nil {
		return err
	}

	// metainfoClient := metainfo.New(transport.NewClient(identity), setupCfg.SatelliteAddr)

	// // Stat will return the health of a specific path
	// resp, err := metainfoClient.Stat(ctx, []byte(dst.Path()), []byte(dst.Bucket()))
	// if err != nil {
	// 	return convertError(err, dst)
	// }

	fmt.Printf("%+v\n", obj)

	return nil
}
