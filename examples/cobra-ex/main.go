// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/pkg/process"
)

func main() {
	root := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			zap.S().Debugf("hello world")
			fmt.Println("hello world was logged to debug")
		},
	}

	root.AddCommand(&cobra.Command{
		Use: "subcommand",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("yay")
		},
	})

	process.Execute(root)
}
