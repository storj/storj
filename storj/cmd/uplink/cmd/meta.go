// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"
)

var metaCmd *cobra.Command

func init() {
	metaCmd = addCmd(&cobra.Command{
		Use:   "meta",
		Short: "Metadata related commands",
	}, RootCmd)
}
