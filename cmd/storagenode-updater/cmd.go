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
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/private/version"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	"storj.io/storj/storagenode"
)

const (
	updaterServiceName = "storagenode-updater"
	minCheckInterval   = time.Minute
)

var (
	// TODO: replace with config value of random bytes in storagenode config.
	nodeID storj.NodeID

	updaterBinaryPath string

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
		storagenode.Config

		BinaryLocation string `help:"the storage node executable binary location" default:"storagenode"`
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

	updaterBinaryPath, err = os.Executable()
	if err != nil {
		zap.L().Fatal("Unable to find storage node updater binary path.")
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

	zap.L().Info("Running on version",
		zap.String("Service", updaterServiceName),
		zap.String("Version", version.Build.Version.String()),
	)

	ctx, _ := process.Ctx(cmd)

	switch {
	case runCfg.Version.CheckInterval <= 0:
		err = loopFunc(ctx)
	case runCfg.Version.CheckInterval < minCheckInterval:
		zap.L().Error("Check interval below minimum. Overriding it minimum.",
			zap.Stringer("Check Interval", runCfg.Version.CheckInterval),
			zap.Stringer("Minimum Check Interval", minCheckInterval),
		)
		runCfg.Version.CheckInterval = minCheckInterval
		fallthrough
	default:
		loop := sync2.NewCycle(runCfg.Version.CheckInterval)
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

	logger, err := process.NewLoggerWithOutputPaths("storagenode-updater", logPath)
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
