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
		Use:   "cat",
		Short: "Copies a Storj object to standard out",
		RunE:  catMain,
	}, RootCmd)
}

// catMain is the function executed when catCmd is called
func catMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no object specified for copy")
	}

	ctx, _ := process.Ctx(cmd)

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if src.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	dst, err := fpath.New("-")
	if err != nil {
		return err
	}

	return download(ctx, src, dst, false)
}
