// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

// runCfg defines configuration for run command.
type runCfg struct {
	StorageNodeFlags
}

// newRunCmd creates a new run command.
func newRunCmd(f *Factory) *cobra.Command {
	var runCfg runCfg

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the storagenode",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdRun(cmd, &runCfg)
		},
	}

	process.Bind(cmd, &runCfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func cmdRun(cmd *cobra.Command, cfg *runCfg) (err error) {
	// inert constructors only ====

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	cfg.Debug.Address = *process.DebugAddrFlag

	mapDeprecatedConfigs(log, &cfg.StorageNodeFlags)

	identity, err := cfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	if err := cfg.Verify(log); err != nil {
		log.Error("Invalid configuration.", zap.Error(err))
		return err
	}

	db, err := storagenodedb.OpenExisting(ctx, log.Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storagenode: %+v", err)
	}

	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, cfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := storagenode.New(log, identity, db, revocationDB, cfg.Config, version.Build, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	// okay, start doing stuff ====

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, log, nil, cfg.Identity.CertPath); err != nil {
		log.Warn("Failed to initialize telemetry batcher.", zap.Error(err))
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for master database on storagenode: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		return errs.New("Error checking version for storagenode database: %+v", err)
	}

	preflightEnabled, err := cmd.Flags().GetBool("preflight.database-check")
	if err != nil {
		return errs.New("Cannot retrieve preflight.database-check flag: %+v", err)
	}
	if preflightEnabled {
		err = db.Preflight(ctx)
		if err != nil {
			return errs.New("Error during preflight check for storagenode databases: %+v", err)
		}
	}

	if err := peer.Storage2.CacheService.Init(ctx); err != nil {
		log.Error("Failed to initialize CacheService.", zap.Error(err))
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()

	return errs.Combine(runError, closeError)
}
