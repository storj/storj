// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/psservice"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "farmer",
		Short: "Farmer",
	}

	cfg struct {
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		Storage  psservice.Config
	}
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the farmer",
		RunE:  cmdRun,
	})
	cfgstruct.Bind(rootCmd.PersistentFlags(), &cfg)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return cfg.Identity.Run(process.Ctx(cmd), cfg.Kademlia, cfg.Storage)
}

func main() {
	process.ExecuteWithConfig(rootCmd, "$HOME/.storj/farmer/config.yaml")
}
