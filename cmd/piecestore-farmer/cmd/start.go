// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	_ "github.com/mattn/go-sqlite3" // sqlite driver
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/piecestore/rpc/server"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a farmer node by ID",
	Long:  "Start farmer node by ID using farmer node config values",
	RunE:  startNode,
}

func init() {
	RootCmd.AddCommand(startCmd)
}

// startNode starts a farmer node by ID
func startNode(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return errs.New("No ID specified")
	}

	_, _, err := SetConfigPath(args[0])
	if err != nil {
		return err
	}

	err = viper.ReadInConfig()
	if err != nil {
		return err
	}

	config := GetConfigValues()

	s := server.New(config)

	err = s.Start()
	if err != nil {
		return err
	}

	return nil
}
