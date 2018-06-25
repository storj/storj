// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "piecestore-farmer",
	Short: "Piecestore-Farmer CLI",
	Long:  "Piecestore-Farmer command line utility for creating, starting, and deleting farmer nodes",
}
