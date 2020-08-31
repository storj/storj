// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"

	"storj.io/private/process"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
)

var (
	mon = monkit.Package()

	rootCmd = &cobra.Command{
		Use:   "segment-reaper",
		Short: "A tool for detecting and deleting zombie segments",
	}
)

func main() {
	process.Exec(rootCmd)
}
