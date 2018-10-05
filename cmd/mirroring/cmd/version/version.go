// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Shows version of application",
	Long: ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("version called")
	},
}

func init() {
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
