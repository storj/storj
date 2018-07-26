// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/process"
)

var (
	mon = monkit.Package()

	rootCmd = &cobra.Command{
		Use:   "captplanet",
		Short: "Captain Planet! With our powers combined!",
	}

	defaultConfDir = "$HOME/.storj/capt"
)

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	process.ExecuteWithConfig(rootCmd,
		filepath.Join(defaultConfDir, "config.yaml"))
}
