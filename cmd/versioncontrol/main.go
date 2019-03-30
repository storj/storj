// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"path/filepath"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/versioncontrol"
)

var (
	rootCmd = &cobra.Command{
		Use:   "versioncontrol",
		Short: "versioncontrol",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the bootstrap server",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	runCfg   versioncontrol.Config
	setupCfg versioncontrol.Config

	confDir string
	isDev   bool
)

const (
	defaultServerAddr = ":8080"
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "versioncontrol")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for versioncontrol configuration")
	cfgstruct.DevFlag(rootCmd, &isDev, false, "use development and test configuration settings")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, isDev, cfgstruct.ConfDir(confDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, isDev, cfgstruct.ConfDir(confDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()

	controlserver, err := versioncontrol.New(log, runCfg)
	if err != nil {
		return
	}
	err = controlserver.Run()

	return
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("bootstrap configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{}

	serverAddress := cmd.Flag("server.address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}

func main() {
	process.Exec(rootCmd)
}
