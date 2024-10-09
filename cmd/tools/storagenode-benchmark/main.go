// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/common/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "",
		Short: "",
	}
)

func main() {
	process.Exec(rootCmd)
}
