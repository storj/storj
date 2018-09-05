// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/miniogw"
)

const defaultConfDir = "$HOME/.storj/cli"

// Config is miniogw.Config configuration
type Config struct {
	miniogw.Config
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "storj",
	Short: "A command-line interface for accessing the Storj network",
}
