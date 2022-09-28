// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/hex"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/uuid"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

// Satellite defines satellite configuration.
type Satellite struct {
	Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	satellite.Config
}

var (
	rootCmd = &cobra.Command{
		Use:   "segment-verify",
		Short: "segment-verify",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "commands to process segments",
	}

	rangeCmd = &cobra.Command{
		Use:   "range",
		Short: "runs the command on a range of segments",
		RunE:  verifySegmentsRange,
	}

	satelliteCfg Satellite
	rangeCfg     RangeConfig

	confDir     string
	identityDir string
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "satellite")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "satellite")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for satellite configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for satellite identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(rangeCmd)

	process.Bind(runCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(rangeCmd, &rangeCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

// RangeConfig defines configuration for verifying segment existence.
type RangeConfig struct {
	Service ServiceConfig
	Verify  VerifierConfig

	Low  string `help:"hex lowest segment id prefix to verify"`
	High string `help:"hex highest segment id prefix to verify (excluded)"`
}

func verifySegmentsRange(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// open default satellite database
	db, err := satellitedb.Open(ctx, log.Named("db"), satelliteCfg.Database, satellitedb.Options{
		ApplicationName:     "segment-verify",
		SaveRollupBatchSize: satelliteCfg.Tally.SaveRollupBatchSize,
		ReadRollupBatchSize: satelliteCfg.Tally.ReadRollupBatchSize,
	})
	if err != nil {
		return errs.New("Error starting master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// open metabase
	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), satelliteCfg.Metainfo.DatabaseURL, metabase.Config{
		ApplicationName:  "satellite-core",
		MinPartSize:      satelliteCfg.Config.Metainfo.MinPartSize,
		MaxNumberOfParts: satelliteCfg.Config.Metainfo.MaxNumberOfParts,
		ServerSideCopy:   satelliteCfg.Config.Metainfo.ServerSideCopy,
	})
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { _ = metabaseDB.Close() }()

	// check whether satellite and metabase versions match
	versionErr := db.CheckVersion(ctx)
	if versionErr != nil {
		log.Error("versions skewed", zap.Error(versionErr))
		return Error.Wrap(versionErr)
	}

	versionErr = metabaseDB.CheckVersion(ctx)
	if versionErr != nil {
		log.Error("versions skewed", zap.Error(versionErr))
		return Error.Wrap(versionErr)
	}

	// setup dialer
	identity, err := satelliteCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	revocationDB, err := revocation.OpenDBFromCfg(ctx, satelliteCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	tlsOptions, err := tlsopts.NewOptions(identity, satelliteCfg.Server.Config, revocationDB)
	if err != nil {
		return Error.Wrap(err)
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	// setup dependencies for verification
	overlay, err := overlay.NewService(log.Named("overlay"), db.OverlayCache(), satelliteCfg.Overlay)
	if err != nil {
		return Error.Wrap(err)
	}

	ordersService, err := orders.NewService(log.Named("orders"), signing.SignerFromFullIdentity(identity), overlay, db.Orders(), satelliteCfg.Orders)
	if err != nil {
		return Error.Wrap(err)
	}

	// setup verifier
	verifier := NewVerifier(log.Named("verifier"), dialer, ordersService, rangeCfg.Verify)
	service, err := NewService(log.Named("service"), metabaseDB, verifier, overlay, rangeCfg.Service)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, service.Close()) }()

	// parse arguments
	var low, high uuid.UUID

	lowBytes, err := hex.DecodeString(rangeCfg.Low)
	if err != nil {
		return Error.Wrap(err)
	}
	highBytes, err := hex.DecodeString(rangeCfg.High)
	if err != nil {
		return Error.Wrap(err)
	}

	copy(low[:], lowBytes)
	copy(low[:], highBytes)

	if high.IsZero() {
		return Error.New("high argument not specified")
	}

	return service.ProcessRange(ctx, low, high)
}

func main() {
	process.Exec(rootCmd)
}
