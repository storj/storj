// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/process"
)

var (
	ctx = context.Background()

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generateCSV",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  generateCSV,
	}
)

func main() {
	rootCmd.AddCommand(cmdGenerate)
	process.Exec(rootCmd)
}

func generateCSV(cmd *cobra.Command, args []string) error {
	return query(args[0], args[1])
}

func query(a, b string) error {
	fmt.Printf("a: %v, b: %v \n", a, b)
	return nil
}
