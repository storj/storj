// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	"storj.io/storj/private/version/checker"
)

const (
	updaterServiceName = "storagenode-updater"
	minCheckInterval   = time.Minute
)

var (
	// TODO: replace with config value of random bytes in storagenode config.
	nodeID storj.NodeID

	rootCmd = &cobra.Command{
		Use:   "storagenode-updater",
		Short: "Version updater for storage node",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the storagenode-updater for storage node",
		Args:  cobra.OnlyValidArgs,
		RunE:  cmdRun,
	}
	restartCmd = &cobra.Command{
		Use:   "restart-service <new binary path>",
		Short: "Restart service with the new binary",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdRestart,
	}

	runCfg struct {
		checker.Config
		Identity identity.Config

		BinaryLocation string `help:"the storage node executable binary location" default:"storagenode.exe"`
		ServiceName    string `help:"storage node OS service name" default:"storagenode"`
		// deprecated
		Log string `help:"deprecated, use --log.output" default:""`
	}

	confDir     string
	identityDir string
)

func init() {
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(restartCmd)

	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(restartCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	if runCfg.Log != "" {
		if err = openLog(runCfg.Log); err != nil {
			zap.L().Error("Error creating new logger.", zap.Error(err))
		}
	}

	if !fileExists(runCfg.BinaryLocation) {
		zap.L().Fatal("Unable to find storage node executable binary.")
	}

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Error loading identity.", zap.Error(err))
	}
	nodeID = ident.ID
	if nodeID.IsZero() {
		zap.L().Fatal("Empty node ID.")
	}

	ctx, _ := process.Ctx(cmd)

	switch {
	case runCfg.CheckInterval <= 0:
		err = loopFunc(ctx)
	case runCfg.CheckInterval < minCheckInterval:
		zap.L().Error("Check interval below minimum. Overriding it minimum.",
			zap.Stringer("Check Interval", runCfg.CheckInterval),
			zap.Stringer("Minimum Check Interval", minCheckInterval),
		)
		runCfg.CheckInterval = minCheckInterval
		fallthrough
	default:
		loop := sync2.NewCycle(runCfg.CheckInterval)
		err = loop.Run(ctx, loopFunc)
	}
	if err != nil && !errs2.IsCanceled(err) {
		log.Fatal(err)
	}

	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.Mode().IsRegular()
}

func openLog(logPath string) error {
	if runtime.GOOS == "windows" && !strings.HasPrefix(logPath, "winfile:///") {
		logPath = "winfile:///" + logPath
	}

	logger, err := process.NewLoggerWithOutputPaths(logPath)
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
