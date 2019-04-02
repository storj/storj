// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "release",
		Short: "release",
	}
	installCmd = &cobra.Command{
		Use:   "install",
		Short: "install the binary",
		RunE:  cmdInstall,
	}
	buildCmd = &cobra.Command{
		Use:   "build",
		Short: "build the binary",
		RunE:  cmdBuild,
	}
)

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(buildCmd)
}

func cmdInstall(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()
	log.Error("Install Called")

	return nil
}

func cmdBuild(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()

	log.Error("Build Called", zap.Strings("args", args))
	return nil
}

func main() {
	process.Exec(rootCmd)
}
