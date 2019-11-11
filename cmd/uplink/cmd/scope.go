// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"github.com/spf13/cobra"
)

var scopeCmd *cobra.Command

func init() {
	scopeCmd = addCmd(&cobra.Command{
		Use:   "scope",
		Short: "Scope related commands",
	}, RootCmd)
}
