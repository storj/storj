// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "put",
		Short: "Copies data from standard in to a Storj object",
		RunE:  putMain,
	}, RootCmd)
}

// putMain is the function executed when putCmd is called
func putMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no object specified for copy")
	}

	ctx, _ := process.Ctx(cmd)

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	src, err := fpath.New("-")
	if err != nil {
		return err
	}

	return upload(ctx, src, dst, false)
}
