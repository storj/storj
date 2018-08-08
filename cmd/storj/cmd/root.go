// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/miniogw"
)

const defaultConfDir = "$HOME/.storj/cli"

type Config struct {
	miniogw.Config
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "storj",
	Short: "A brief description of your application",
}
