// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"
)

var (
	ctx = context.Background()

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generate",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		Run:   generateCSV,
	}
)

func main() {
}

func generateCSV(cmd *cobra.Command, args []string) {

}