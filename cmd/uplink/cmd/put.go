// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "put",
		Short: "Copies data from standard in to a Storj object",
		RunE:  putMain,
	})
}

// putMain is the function executed when putCmd is called
func putMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No object specified for copy")
	}

	ctx := process.Ctx(cmd)

	u0, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	// if uploading
	if u0.Host == "" {
		return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
	}

	return upload(ctx, bs, "-", u0)
}
