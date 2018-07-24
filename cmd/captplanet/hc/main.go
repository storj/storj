// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "hc",
		Short: "Heavy client",
	}

	cfg struct {
		Identity  provider.IdentityConfig
		Kademlia  kademlia.Config
		PointerDB pointerdb.Config
		Overlay   overlay.Config
	}
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the heavy client",
		RunE:  cmdRun,
	})
	cfgstruct.Bind(rootCmd.PersistentFlags(), &cfg)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return cfg.Identity.Run(process.Ctx(cmd), cfg.Kademlia, cfg.PointerDB, cfg.Overlay)
}

func main() {
	process.ExecuteWithConfig(rootCmd, "$HOME/.storj/hc/config.yaml")
}
