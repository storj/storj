// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/fpath"
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
		Short: "Run the versioncontrol server",
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
)

const (
	defaultServerAddr = ":8080"
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "versioncontrol")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for versioncontrol configuration")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.SetupMode())
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()
	controlserver, err := versioncontrol.New(log, &runCfg)
	if err != nil {
		return err
	}
	ctx, _ := process.Ctx(cmd)
	err = controlserver.Run(ctx)
	return err
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("versioncontrol configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{}

	serverAddress := cmd.Flag("address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"),
		process.SaveConfigWithOverrides(overrides))
}

func main() {
	process.Exec(rootCmd)
}
