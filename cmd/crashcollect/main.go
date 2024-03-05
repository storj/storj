// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/process"
	"storj.io/storj/crashcollect"
)

// Config defines storj crash collect service configuration.
type Config struct {
	crashcollect.Config
}

func main() {
	logger, _, _ := process.NewLogger("crashcollect")
	zap.ReplaceGlobals(logger)

	rootCmd := &cobra.Command{
		Use:   "crashcollect",
		Short: "Crash collect service",
	}

	var runCfg Config
	var setupCfg Config
	var confDir string
	var identityDir string

	defaultConfDir := fpath.ApplicationDir("storj", "crashcollect")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "crashcollect")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for storj crash collect service configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for storj crash collect service identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	runCmd := RunCommand(&runCfg)
	setupCmd := SetupCommand(confDir)

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))

	process.ExecCustomDebug(rootCmd)
}

// RunCommand creates command for running crash collect.
func RunCommand(runCfg *Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the storj crash collect service",
	}

	runCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, _ := process.Ctx(cmd)
		log := zap.L()

		identity, err := runCfg.Identity.Load()
		if err != nil {
			log.Error("failed to load identity.", zap.Error(err))
			return errs.New("failed to load identity: %+v", err)
		}

		peer, err := crashcollect.New(log, identity, runCfg.Config)
		if err != nil {
			return err
		}

		runError := peer.Run(ctx)
		closeError := peer.Close()
		return errs.Combine(runError, closeError)
	}

	return runCmd
}

// SetupCommand creates command for creating config file for crash collect service.
func SetupCommand(confDir string) *cobra.Command {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		Annotations: map[string]string{"type": "setup"},
	}

	setupCmd.RunE = func(cmd *cobra.Command, args []string) error {
		setupDir, err := filepath.Abs(confDir)
		if err != nil {
			return err
		}

		valid, _ := fpath.IsValidSetupDir(setupDir)
		if !valid {
			return fmt.Errorf("storj crash collect service configuration already exists (%v)", setupDir)
		}

		err = os.MkdirAll(setupDir, 0700)
		if err != nil {
			return err
		}

		return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"))
	}

	return setupCmd
}
