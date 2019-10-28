// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/cmd/satellite/reports"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb"
)

// Satellite defines satellite configuration
type Satellite struct {
	Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	satellite.Config
}

var (
	rootCmd = &cobra.Command{
		Use:   "satellite",
		Short: "Satellite",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the satellite",
		RunE:  cmdRun,
	}
	runAPICmd = &cobra.Command{
		Use:   "api",
		Short: "Run the satellite API",
		RunE:  cmdAPIRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	qdiagCmd = &cobra.Command{
		Use:   "qdiag",
		Short: "Repair Queue Diagnostic Tool support",
		RunE:  cmdQDiag,
	}
	reportsCmd = &cobra.Command{
		Use:   "reports",
		Short: "Generate a report",
	}
	nodeUsageCmd = &cobra.Command{
		Use:   "storagenode-usage [start] [end]",
		Short: "Generate a node usage report for a given period to use for payments",
		Long:  "Generate a node usage report for a given period to use for payments. Format dates using YYYY-MM-DD",
		Args:  cobra.MinimumNArgs(2),
		RunE:  cmdNodeUsage,
	}
	partnerAttributionCmd = &cobra.Command{
		Use:   "partner-attribution [partner ID] [start] [end]",
		Short: "Generate a partner attribution report for a given period to use for payments",
		Long:  "Generate a partner attribution report for a given period to use for payments. Format dates using YYYY-MM-DD",
		Args:  cobra.MinimumNArgs(3),
		RunE:  cmdValueAttribution,
	}
	gracefulExitCmd = &cobra.Command{
		Use:   "graceful-exit [start] [end]",
		Short: "Generate a graceful exit report",
		Long:  "Generate a node usage report for a given period to use for payments. Format dates using YYYY-MM-DD",
		Args:  cobra.MinimumNArgs(2),
		RunE:  cmdGracefulExit,
	}

	runCfg   Satellite
	setupCfg Satellite

	qdiagCfg struct {
		Database   string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		QListLimit int    `help:"maximum segments that can be requested" default:"1000"`
	}
	nodeUsageCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output   string `help:"destination of report output" default:""`
	}
	partnerAttribtionCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output   string `help:"destination of report output" default:""`
	}
	gracefulExitCfg struct {
		Database  string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"sqlite3://$CONFDIR/master.db"`
		Output    string `help:"destination of report output" default:""`
		Completed bool   `help:"whether to output (initiated and completed) or (initiated and not completed)" default:"false"`
	}
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
	runCmd.AddCommand(runAPICmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(qdiagCmd)
	rootCmd.AddCommand(reportsCmd)
	reportsCmd.AddCommand(nodeUsageCmd)
	reportsCmd.AddCommand(partnerAttributionCmd)
	reportsCmd.AddCommand(gracefulExitCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAPICmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(qdiagCmd, &qdiagCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(nodeUsageCmd, &nodeUsageCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(gracefulExitCmd, &gracefulExitCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(partnerAttributionCmd, &partnerAttribtionCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	// inert constructors only ====

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

	db, err := satellitedb.New(log.Named("db"), runCfg.Database)
	if err != nil {
		return errs.New("Error starting master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	pointerDB, err := metainfo.NewStore(log.Named("pointerdb"), runCfg.Config.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	revocationDB, err := revocation.NewDBFromCfg(runCfg.Config.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	liveAccounting, err := live.NewCache(log.Named("live-accounting"), runCfg.LiveAccounting)
	if err != nil {
		return errs.New("Error creating live accounting cache: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, liveAccounting.Close())
	}()

	peer, err := satellite.New(log, identity, db, pointerDB, revocationDB, liveAccounting, version.Build, &runCfg.Config)
	if err != nil {
		return err
	}

	// okay, start doing stuff ====

	err = peer.Version.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, log, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Warn("Failed to initialize telemetry batcher: ", err)
	}

	err = db.CreateTables()
	if err != nil {
		return errs.New("Error creating tables for master database on satellite: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}

func cmdAPIRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

	db, err := satellitedb.New(log.Named("db"), runCfg.Database)
	if err != nil {
		return errs.New("Error starting master database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	pointerDB, err := metainfo.NewStore(log.Named("pointerdb"), runCfg.Config.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating revocation database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	revocationDB, err := revocation.NewDBFromCfg(runCfg.Config.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	accountingCache, err := live.NewCache(log.Named("live-accounting"), runCfg.LiveAccounting)
	if err != nil {
		return errs.New("Error creating live accounting cache on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, accountingCache.Close())
	}()

	peer, err := satellite.NewAPI(log, identity, db, pointerDB, revocationDB, accountingCache, &runCfg.Config, version.Build)
	if err != nil {
		return err
	}

	err = peer.Version.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, log, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Warn("Failed to initialize telemetry batcher on satellite api: ", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("satellite configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	return process.SaveConfig(cmd, filepath.Join(setupDir, "config.yaml"))
}

func cmdQDiag(cmd *cobra.Command, args []string) (err error) {

	// open the master db
	database, err := satellitedb.New(zap.L().Named("db"), qdiagCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err := database.Close()
		if err != nil {
			fmt.Printf("error closing connection to master database on satellite: %+v\n", err)
		}
	}()

	list, err := database.RepairQueue().SelectN(context.Background(), qdiagCfg.QListLimit)
	if err != nil {
		return err
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "Path\tLost Pieces\t")

	// populate the row fields
	for _, v := range list {
		fmt.Fprint(w, v.GetPath(), "\t", v.GetLostPieces(), "\t")
	}

	// display the data
	return w.Flush()
}

func cmdGracefulExit(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	layout := "2006-01-02"
	start, err := time.Parse(layout, args[0])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}
	end, err := time.Parse(layout, args[1])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}

	// adding one day to properly account for the entire end day
	end = end.AddDate(0, 0, 1)

	// ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	// send output to stdout
	if gracefulExitCfg.Output == "" {
		return generateGracefulExitCSV(ctx, gracefulExitCfg.Completed, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(gracefulExitCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	return generateGracefulExitCSV(ctx, gracefulExitCfg.Completed, start, end, file)
}

func cmdNodeUsage(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	layout := "2006-01-02"
	start, err := time.Parse(layout, args[0])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}
	end, err := time.Parse(layout, args[1])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}

	//Adding one day to properly account for the entire end day
	end = end.Add(time.Hour * 24)

	// Ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	// send output to stdout
	if nodeUsageCfg.Output == "" {
		return generateNodeUsageCSV(ctx, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(nodeUsageCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	return generateNodeUsageCSV(ctx, start, end, file)
}

func cmdValueAttribution(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L().Named("satellite-cli")
	// Parse the UUID
	partnerID, err := uuid.Parse(args[0])
	if err != nil {
		return errs.Combine(errs.New("Invalid Partner ID format. %s", args[0]), err)
	}

	layout := "2006-01-02"
	start, err := time.Parse(layout, args[1])
	if err != nil {
		return errs.New("Invalid start date format. Please use YYYY-MM-DD")
	}
	end, err := time.Parse(layout, args[2])
	if err != nil {
		return errs.New("Invalid end date format. Please use YYYY-MM-DD")
	}

	// Ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	// send output to stdout
	if partnerAttribtionCfg.Output == "" {
		return reports.GenerateAttributionCSV(ctx, partnerAttribtionCfg.Database, *partnerID, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(partnerAttribtionCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
		if err != nil {
			log.Sugar().Errorf("error closing the file %v after retrieving partner value attribution data: %+v", partnerAttribtionCfg.Output, err)
		}
	}()

	return reports.GenerateAttributionCSV(ctx, partnerAttribtionCfg.Database, *partnerID, start, end, file)
}

func main() {
	process.Exec(rootCmd)
}
