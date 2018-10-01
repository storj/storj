// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package config

import (
	"fmt"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var Cmd = &cobra.Command{
	Use:   "config",
	Short: "Mirroring options setup",
	Long: ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("config called")
	},
}

func init() {
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// configCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// configCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
