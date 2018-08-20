// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	rmCfg Config
	rmCmd = &cobra.Command{
		Use:   "rm",
		Short: "A brief description of your command",
		RunE:  delete,
	}
)

func init() {
	RootCmd.AddCommand(rmCmd)
	cfgstruct.Bind(rmCmd.Flags(), &rmCfg, cfgstruct.ConfDir(defaultConfDir))
	rmCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func delete(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return errs.New("No object specified for deletion")
	}

	so, err := getStorjObjects(ctx, rmCfg)
	if err != nil {
		return err
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	err = so.DeleteObject(ctx, u.Host, u.Path)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %s from %s", u.Path, u.Host)

	return nil
}
