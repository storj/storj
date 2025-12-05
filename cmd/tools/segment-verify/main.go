// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/uuid"
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
		RunE:  verifySegments,
	}

	bucketsCmd = &cobra.Command{
		Use:   "buckets",
		Short: "runs the command on segments from specified buckets",
		RunE:  verifySegments,
	}

	readCSVCmd = &cobra.Command{
		Use:   "read-csv",
		Short: "runs the command on segments from an input CSV file",
		RunE:  verifySegments,
	}

	summarizeCmd = &cobra.Command{
		Use:   "summarize-log",
		Short: "summarizes verification log",
		Args:  cobra.ExactArgs(1),
		RunE:  summarizeVerificationLog,
	}

	nodeCheckCmd = &cobra.Command{
		Use:   "node-check",
		Short: "checks segments for too many duplicate or unvetted nodes",
		RunE:  verifySegmentsNodeCheck,
	}

	satelliteCfg Satellite
	rangeCfg     RangeConfig
	bucketsCfg   BucketConfig
	readCSVCfg   ReadCSVConfig
	nodeCheckCfg NodeCheckConfig

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
	rootCmd.AddCommand(summarizeCmd)
	rootCmd.AddCommand(nodeCheckCmd)
	runCmd.AddCommand(rangeCmd)
	runCmd.AddCommand(bucketsCmd)
	runCmd.AddCommand(readCSVCmd)

	process.Bind(runCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))

	process.Bind(rangeCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(rangeCmd, &rangeCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(bucketsCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(bucketsCmd, &bucketsCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(readCSVCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(readCSVCmd, &readCSVCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))

	process.Bind(nodeCheckCmd, &satelliteCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(nodeCheckCmd, &nodeCheckCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

// RangeConfig defines configuration for verifying segment existence.
type RangeConfig struct {
	Service ServiceConfig
	Verify  VerifierConfig

	Low  string `help:"hex lowest segment id prefix to verify"`
	High string `help:"hex highest segment id prefix to verify (excluded)"`
}

// BucketConfig defines configuration for verifying segment existence within a list of buckets.
type BucketConfig struct {
	Service ServiceConfig
	Verify  VerifierConfig

	BucketsCSV string `help:"csv file of project_id,bucket_name of buckets to verify" default:""`
}

// ReadCSVConfig defines configuration for verifying existence of specific segments.
type ReadCSVConfig struct {
	Service ServiceConfig
	Verify  VerifierConfig

	InputFile string `help:"csv file of segment_id,position for segments to verify"`
}

func verifySegments(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	return verifySegmentsInContext(ctx, log, cmd, satelliteCfg, rangeCfg)
}

func verifySegmentsInContext(ctx context.Context, log *zap.Logger, cmd *cobra.Command, satelliteCfg Satellite, rangeCfg RangeConfig) error {
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
	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), satelliteCfg.Metainfo.DatabaseURL,
		satelliteCfg.Config.Metainfo.Metabase("segment-verify"))
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

	placements, err := satelliteCfg.Placement.Parse(satelliteCfg.Overlay.Node.CreateDefaultPlacement, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	// setup dependencies for verification
	overlayService, err := overlay.NewService(log.Named("overlay"), db.OverlayCache(), db.NodeEvents(), placements, "", "", satelliteCfg.Overlay)
	if err != nil {
		return Error.Wrap(err)
	}

	ordersService, err := orders.NewService(log.Named("orders"), signing.SignerFromFullIdentity(identity), overlayService, orders.NewNoopDB(), placements.CreateFilters, satelliteCfg.Orders)
	if err != nil {
		return Error.Wrap(err)
	}

	var (
		verifyConfig  VerifierConfig
		serviceConfig ServiceConfig
		commandFunc   func(ctx context.Context, service *Service) error
	)
	switch cmd.Name() {
	case "range":
		verifyConfig = rangeCfg.Verify
		serviceConfig = rangeCfg.Service
		commandFunc = func(ctx context.Context, service *Service) error {
			return verifySegmentsRange(ctx, service, rangeCfg)
		}
	case "buckets":
		verifyConfig = bucketsCfg.Verify
		serviceConfig = bucketsCfg.Service
		commandFunc = verifySegmentsBuckets
	case "read-csv":
		verifyConfig = readCSVCfg.Verify
		serviceConfig = readCSVCfg.Service
		commandFunc = func(ctx context.Context, service *Service) error {
			return verifySegmentsCSV(ctx, service, readCSVCfg)
		}
	default:
		return errors.New("unknown command: " + cmd.Name())
	}

	// setup verifier
	verifier := NewVerifier(log.Named("verifier"), dialer, ordersService, verifyConfig)
	service, err := NewService(log.Named("service"), metabaseDB, verifier, overlayService, serviceConfig)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, service.Close()) }()

	log.Debug("starting", zap.Any("config", service.config), zap.String("command", cmd.Name()))
	return commandFunc(ctx, service)
}

func verifySegmentsRange(ctx context.Context, service *Service, rangeCfg RangeConfig) error {
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
	copy(high[:], highBytes)

	if high.IsZero() {
		return Error.New("high argument not specified")
	}

	return service.ProcessRange(ctx, low, high)
}

func verifySegmentsBuckets(ctx context.Context, service *Service) error {
	if bucketsCfg.BucketsCSV == "" {
		return Error.New("bucket list file path not provided")
	}

	bucketList, err := service.ParseBucketFile(bucketsCfg.BucketsCSV)
	if err != nil {
		return Error.Wrap(err)
	}
	return service.ProcessBuckets(ctx, bucketList.Buckets)
}

func verifySegmentsCSV(ctx context.Context, service *Service, readCSVCfg ReadCSVConfig) (err error) {
	if readCSVCfg.InputFile == "" {
		return Error.New("input CSV file not provided")
	}

	segmentSource, err := OpenSegmentCSVFile(readCSVCfg.InputFile)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, segmentSource.Close()) }()
	return service.ProcessSegmentsFromCSV(ctx, segmentSource)
}

func main() {
	logger, _, _ := process.NewLogger("segment-verify")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}

// ParseBucketFile parses a csv file containing project_id and bucket names.
func (service *Service) ParseBucketFile(path string) (_ BucketList, err error) {
	csvFile, err := os.Open(path)
	if err != nil {
		return BucketList{}, err
	}
	defer func() {
		err = errs.Combine(err, csvFile.Close())
	}()

	csvReader := csv.NewReader(csvFile)
	allEntries, err := csvReader.ReadAll()
	if err != nil {
		return BucketList{}, err
	}

	bucketList := BucketList{}
	for _, entry := range allEntries {
		if len(entry) < 2 {
			return BucketList{}, Error.New("unable to parse buckets file: %w", err)
		}

		projectId, err := projectIdFromCompactString(strings.TrimSpace(entry[0]))
		if err != nil {
			return BucketList{}, Error.New("unable to parse buckets file: %w", err)
		}
		bucketList.Add(projectId, metabase.BucketName(strings.TrimSpace(entry[1])))
	}
	return bucketList, nil
}

func projectIdFromCompactString(s string) (uuid.UUID, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return uuid.UUID{}, Error.New("invalid string")
	}

	return uuid.FromBytes(decoded)
}
