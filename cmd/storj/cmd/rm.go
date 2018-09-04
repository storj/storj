// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
)

var (
	rmCfg Config
	rmCmd = &cobra.Command{
		Use:   "rm",
		Short: "Delete an object",
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

	identity, err := rmCfg.Load()
	if err != nil {
		return err
	}

	bs, err := rmCfg.GetBucketStore(ctx, identity)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		fmt.Println("No object specified for deletion")
		return nil
	}

	u, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}

	o, err := bs.GetObjectStore(ctx, u.Host)
	if err != nil {
		return err
	}

	err = o.Delete(ctx, paths.New(u.Path))
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %s from %s\n", u.Path, u.Host)

	return nil
}
