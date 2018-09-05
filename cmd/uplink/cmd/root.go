// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/miniogw"
)

const defaultConfDir = "$HOME/.storj/uplink"

// Config is miniogw.Config configuration
type Config struct {
	miniogw.Config
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "uplink",
	Short: "The Storj client-side S3 gateway and CLI",
}
