// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/process"
	"storj.io/common/process/eventkitbq"
	_ "storj.io/common/process/googleprofiler" // This attaches google cloud profiler.
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
)

// Config is the configuration for the job queue server.
type Config struct {
	satellite.JobqConfig

	Identity identity.Config
}

var (
	confDir     string
	identityDir string

	runCfg Config

	rootCmd = &cobra.Command{
		Use:   "jobq",
		Short: "job queue server (implements the repair queue)",
		RunE:  runJobQueue,
	}
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "jobq")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "jobq")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for jobq configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for jobq identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(rootCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func runJobQueue(cmd *cobra.Command, args []string) error {
	logger := zap.L()
	ctx, _ := process.Ctx(cmd)

	identity, err := runCfg.Identity.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}
	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.TLS)
	if err != nil {
		return fmt.Errorf("error creating revocation database: %w", err)
	}
	peer, err := satellite.NewJobq(logger, identity, process.AtomicLevel(cmd), &runCfg.JobqConfig, revocationDB)
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, logger, monkit.Default, process.MetricsIDFromHostname(logger), eventkitbq.BQDestination); err != nil {
		logger.Warn("Failed to initialize telemetry batcher on satellite api", zap.Error(err))
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errors.Join(runError, closeError)
}

func main() {
	logger, _, _ := process.NewLogger("jobq")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}
