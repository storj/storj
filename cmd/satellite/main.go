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

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
)

// Satellite defines satellite configuration
type Satellite struct {
	Database string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`

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
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	diagCmd = &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE:  cmdDiag,
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

	runCfg   Satellite
	setupCfg Satellite

	diagCfg struct {
		Database string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
	}
	qdiagCfg struct {
		Database   string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
		QListLimit int    `help:"maximum segments that can be requested" default:"1000"`
	}
	nodeUsageCfg struct {
		Database string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
		Output   string `help:"destination of report output" default:""`
	}
	confDir     string
	identityDir string
	isDev       bool
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "satellite")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "satellite")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for satellite configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for satellite identity credentials")
	cfgstruct.DevFlag(rootCmd, &isDev, true, "use development and test configuration settings")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(qdiagCmd)
	rootCmd.AddCommand(reportsCmd)
	reportsCmd.AddCommand(nodeUsageCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.Bind(qdiagCmd.Flags(), &qdiagCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.Bind(nodeUsageCmd.Flags(), &nodeUsageCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	// inert constructors only ====

	ctx := process.Ctx(cmd)
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

	peer, err := satellite.New(log, identity, db, &runCfg.Config, version.Build)
	if err != nil {
		return err
	}

	// okay, start doing stuff ====

	err = peer.Version.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	err = db.CreateTables()
	if err != nil {
		return errs.New("Error creating tables for master database on satellite: %+v", err)
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

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), nil)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	database, err := satellitedb.New(zap.L().Named("db"), diagCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err := database.Close()
		if err != nil {
			fmt.Printf("error closing connection to master database on satellite: %+v\n", err)
		}
	}()

	//get all bandwidth agreements rows already ordered
	stats, err := database.BandwidthAgreement().GetUplinkStats(context.Background(), time.Time{}, time.Now())
	if err != nil {
		fmt.Printf("error reading satellite database %v: %v\n", diagCfg.Database, err)
		return err
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "UplinkID\tTotal\t# Of Transactions\tPUT Action\tGET Action\t")

	// populate the row fields
	for _, s := range stats {
		fmt.Fprint(w, s.NodeID, "\t", s.TotalBytes, "\t", s.TotalTransactions, "\t", s.PutActionCount, "\t", s.GetActionCount, "\t\n")
	}

	// display the data
	return w.Flush()
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

	list, err := database.RepairQueue().Peekqueue(context.Background(), qdiagCfg.QListLimit)
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

func cmdNodeUsage(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	layout := "2006-01-02"
	start, err := time.Parse(layout, args[0])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}
	end, err := time.Parse(layout, args[1])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}

	// Ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	// send output to stdout
	if nodeUsageCfg.Output == "" {
		return generateCSV(ctx, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(nodeUsageCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	return generateCSV(ctx, start, end, file)
}

func main() {
	process.Exec(rootCmd)
}
