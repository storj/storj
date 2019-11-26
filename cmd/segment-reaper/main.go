// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "segment-reaper",
		Short: "A tool for detecting and deleting zombie segments",
	}
)

func main() {
	process.Exec(rootCmd)
}
