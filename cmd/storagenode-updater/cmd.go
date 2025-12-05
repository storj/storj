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

	"storj.io/common/cfgstruct"
	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/version"
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
	shouldUpdateCmd = &cobra.Command{
		Use:   "should-update <service>",
		Short: "Check if service should be updated to suggested version",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdShouldUpdate,
	}

	runCfg struct {
		Identity identity.Config
		Version  checker.Config

		BinaryLocation string `help:"the storage node executable binary location" default:"storagenode"`
		BinaryStoreDir string `help:"dir to backup current binaries. Use it only for setups running the storagenode docker image. Path specified must be a host filesystem mounted destination." default:""`
		ServiceName    string `help:"storage node OS service name" default:"storagenode"`
		RestartMethod  string `help:"Method used to restart services. Default is 'kill'' (good for containers). 'service' is supported on FreeBSD, to use rc.d" default:"kill"`

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
	rootCmd.AddCommand(shouldUpdateCmd)

	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(restartCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(shouldUpdateCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
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

func cmdShouldUpdate(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	service := args[0]

	switch service {
	case "storagenode", "storagenode-updater":
	default: // do not decide if other than above mentioned processes should be updated
		zap.L().Error("Process is not allowed", zap.String("service", service))
		os.Exit(1)
	}

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Error loading identity.", zap.Error(err))
	}
	nodeID = ident.ID
	if nodeID.IsZero() {
		zap.L().Fatal("Empty node ID.")
	}

	ver, err := checker.New(runCfg.Version.ClientConfig).Process(ctx, service)
	if err != nil {
		zap.L().Fatal("Error retrieving version info.", zap.Error(err))
	}

	var shouldUpdate bool

	if runCfg.BinaryLocation != "" && fileExists(runCfg.BinaryLocation) {
		currentVersion, err := binaryVersion(runCfg.BinaryLocation)
		if err != nil {
			zap.L().Fatal("Error retrieving binary version.", zap.Error(err))
		}

		updateVersion, _, err := version.ShouldUpdateVersion(currentVersion, nodeID, ver)
		if err != nil {
			zap.L().Error("Error on should update version",
				zap.String("service", service),
				zap.Error(err))
		}

		shouldUpdate = !updateVersion.IsZero()
	} else {
		shouldUpdate = version.ShouldUpdate(ver.Rollout, nodeID)
	}

	if shouldUpdate {
		zap.L().Info("Service should be updated", zap.String("service", service))
		os.Exit(0)
	} else {
		zap.L().Info("Service should not be updated", zap.String("service", service))
		os.Exit(1)
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

	zap.ReplaceGlobals(logger.With(zap.String("Process", updaterServiceName)))
	return nil
}
