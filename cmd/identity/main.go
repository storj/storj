// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "identity",
		Short: "Identity management",
	}

	defaultConfDir = fpath.ApplicationDir("storj", "identity")
)

func main() {
	process.Exec(rootCmd)
}
