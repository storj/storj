// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "gw",
		Short: "Gateway",
	}

	cfg miniogw.Config

	defaultConfDir = "$HOME/.storj/gw"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the gateway",
		RunE:  cmdRun,
	})
	cfgstruct.Bind(rootCmd.PersistentFlags(), &cfg,
		cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return cfg.Run(process.Ctx(cmd))
}

func main() {
	process.ExecuteWithConfig(rootCmd,
		filepath.Join(defaultConfDir, "config.yaml"))
}
