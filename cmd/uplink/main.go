// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/pkg/process"
)

func main() {
	process.ExecWithCustomConfig(cmd.RootCmd, func(cmd *cobra.Command, vip *viper.Viper) error {
		accessFlag := cmd.Flags().Lookup("access")
		if accessFlag == nil || accessFlag.Value.String() == "" {
			return process.LoadConfig(cmd, vip)
		}
		return nil
	})
}
