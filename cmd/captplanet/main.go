// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/process"
)

var (
	mon = monkit.Package()

	rootCmd = &cobra.Command{
		Use:   "captplanet",
		Short: "Captain Planet! With our powers combined!",
	}

	defaultConfDir = getDefaultConfDir()
)

func getDefaultConfDir() string {
	switch runtime.GOOS {
	default:
		return "$HOME/.storj/capt"
	case "windows":
		return filepath.Join(os.Getenv("AppData"), "Storj", "capt")
	}
}

func main() {
	// process.Exec will load this for this command.
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	setupCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "setup.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
