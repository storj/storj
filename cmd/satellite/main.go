// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/fpath"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	_ "storj.io/private/process/googleprofiler" // This attaches google cloud profiler.
	"storj.io/private/version"
	"storj.io/storj/cmd/satellite/reports"
	"storj.io/storj/private/lrucache"
	"storj.io/storj/private/revocation"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
)

// Satellite defines satellite configuration.
type Satellite struct {
	Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	DatabaseOptions struct {
		APIKeysCache struct {
			Expiration time.Duration `help:"satellite database api key expiration" default:"60s"`
			Capacity   int           `help:"satellite database api key lru capacity" default:"1000"`
		}
		RevocationsCache struct {
			Expiration time.Duration `help:"macaroon revocation cache expiration" default:"5m"`
			Capacity   int           `help:"macaroon revocation cache capacity" default:"10000"`
		}
	}

	satellite.Config
}

// APIKeysLRUOptions returns a cache.Options based on the APIKeys LRU config.
func (s *Satellite) APIKeysLRUOptions() lrucache.Options {
	return lrucache.Options{
		Expiration: s.DatabaseOptions.APIKeysCache.Expiration,
		Capacity:   s.DatabaseOptions.APIKeysCache.Capacity,
	}
}

// RevocationLRUOptions returns a cache.Options based on the Revocations LRU config.
func (s *Satellite) RevocationLRUOptions() lrucache.Options {
	return lrucache.Options{
		Expiration: s.DatabaseOptions.RevocationsCache.Expiration,
		Capacity:   s.DatabaseOptions.RevocationsCache.Capacity,
	}
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
	runMigrationCmd = &cobra.Command{
		Use:   "migration",
		Short: "Run the satellite database migration",
		RunE:  cmdMigrationRun,
	}
	runAPICmd = &cobra.Command{
		Use:   "api",
		Short: "Run the satellite API",
		RunE:  cmdAPIRun,
	}
	runRepairerCmd = &cobra.Command{
		Use:   "repair",
		Short: "Run the repair service",
		RunE:  cmdRepairerRun,
	}
	runAdminCmd = &cobra.Command{
		Use:   "admin",
		Short: "Run the satellite Admin",
		RunE:  cmdAdminRun,
	}
	runGCCmd = &cobra.Command{
		Use:   "garbage-collection",
		Short: "Run the satellite garbage collection process",
		RunE:  cmdGCRun,
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
		Long:  "Generate a node usage report for a given period to use for payments. Format dates using YYYY-MM-DD. The end date is exclusive.",
		Args:  cobra.MinimumNArgs(2),
		RunE:  cmdNodeUsage,
	}
	partnerAttributionCmd = &cobra.Command{
		Use:   "partner-attribution [partner ID] [start] [end]",
		Short: "Generate a partner attribution report for a given period to use for payments",
		Long:  "Generate a partner attribution report for a given period to use for payments. Format dates using YYYY-MM-DD. The end date is exclusive.",
		Args:  cobra.MinimumNArgs(3),
		RunE:  cmdValueAttribution,
	}
	reportsGracefulExitCmd = &cobra.Command{
		Use:   "graceful-exit [start] [end]",
		Short: "Generate a graceful exit report",
		Long:  "Generate a node usage report for a given period to use for payments. Format dates using YYYY-MM-DD. The end date is exclusive.",
		Args:  cobra.MinimumNArgs(2),
		RunE:  cmdReportsGracefulExit,
	}
	reportsVerifyGEReceiptCmd = &cobra.Command{
		Use:   "verify-exit-receipt [storage node ID] [receipt]",
		Short: "Verify a graceful exit receipt",
		Long:  "Verify a graceful exit receipt is valid.",
		Args:  cobra.MinimumNArgs(2),
		RunE:  reportsVerifyGEReceipt,
	}
	compensationCmd = &cobra.Command{
		Use:   "compensation",
		Short: "Storage Node Compensation commands",
	}
	generateInvoicesCmd = &cobra.Command{
		Use:   "generate-invoices [period]",
		Short: "Generate storage node invoices",
		Long:  "Generate storage node invoices for a pay period. Period is a UTC date formatted like YYYY-MM.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdGenerateInvoices,
	}
	recordPeriodCmd = &cobra.Command{
		Use:   "record-period [paystubs-csv] [payments-csv]",
		Short: "Record storage node pay period",
		Long:  "Record storage node paystubs and payments for a pay period",
		Args:  cobra.ExactArgs(2),
		RunE:  cmdRecordPeriod,
	}
	recordOneOffPaymentsCmd = &cobra.Command{
		Use:   "record-one-off-payments [payments-csv]",
		Short: "Record one-off storage node payments",
		Long:  "Record one-off storage node payments outside of a pay period",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdRecordOneOffPayments,
	}
	billingCmd = &cobra.Command{
		Use:   "billing",
		Short: "Customer billing commands",
	}
	prepareCustomerInvoiceRecordsCmd = &cobra.Command{
		Use:   "prepare-invoice-records [period]",
		Short: "Prepares invoice project records",
		Long:  "Prepares invoice project records that will be used during invoice line items creation.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdPrepareCustomerInvoiceRecords,
	}
	createCustomerInvoiceItemsCmd = &cobra.Command{
		Use:   "create-invoice-items [period]",
		Short: "Creates stripe invoice line items",
		Long:  "Creates stripe invoice line items for not consumed project records.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCreateCustomerInvoiceItems,
	}
	createCustomerInvoiceCouponsCmd = &cobra.Command{
		Use:   "create-invoice-coupons [period]",
		Short: "Adds coupons to stripe invoices",
		Long:  "Creates stripe invoice line items for not consumed coupons.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCreateCustomerInvoiceCoupons,
	}
	createCustomerInvoicesCmd = &cobra.Command{
		Use:   "create-invoices [period]",
		Short: "Creates stripe invoices from pending invoice items",
		Long:  "Creates stripe invoices for all stripe customers known to satellite",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCreateCustomerInvoices,
	}
	finalizeCustomerInvoicesCmd = &cobra.Command{
		Use:   "finalize-invoices",
		Short: "Finalizes all draft stripe invoices",
		Long:  "Finalizes all draft stripe invoices known to satellite's stripe account.",
		RunE:  cmdFinalizeCustomerInvoices,
	}
	stripeCustomerCmd = &cobra.Command{
		Use:   "ensure-stripe-customer",
		Short: "Ensures that we have a stripe customer for every user",
		Long:  "Ensures that we have a stripe customer for every satellite user.",
		RunE:  cmdStripeCustomer,
	}
	consistencyCmd = &cobra.Command{
		Use:   "consistency",
		Short: "Readdress DB consistency issues",
		Long:  "Readdress DB consistency issues and perform data cleanups for improving the DB performance.",
	}
	consistencyGECleanupCmd = &cobra.Command{
		Use:   "ge-cleanup-orphaned-data",
		Short: "Cleanup Graceful Exit orphaned data",
		Long:  "Cleanup Graceful Exit data which is lingering in the transfer queue DB table on nodes which has finished the exit.",
		RunE:  cmdConsistencyGECleanup,
	}
	restoreTrashCmd = &cobra.Command{
		Use:   "restore-trash [node-id-1 node-id-2 node-id-3 ...]",
		Short: "Restore trash",
		Long: "Tell storage nodes to undo garbage collection. " +
			"If node ids aren't provided, *all* nodes are used.",
		RunE: cmdRestoreTrash,
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
	generateInvoicesCfg struct {
		Database     string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output       string `help:"destination of report output" default:""`
		Compensation compensation.Config
		SurgePercent int64 `help:"surge percent for payments" default:"0"`
	}
	recordPeriodCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
	}
	recordOneOffPaymentsCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
	}
	partnerAttribtionCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output   string `help:"destination of report output" default:""`
	}
	reportsGracefulExitCfg struct {
		Database  string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output    string `help:"destination of report output" default:""`
		Completed bool   `help:"whether to output (initiated and completed) or (initiated and not completed)" default:"false"`
	}
	reportsVerifyGracefulExitReceiptCfg struct {
	}
	consistencyGECleanupCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Before   string `help:"select only exited nodes before this UTC date formatted like YYYY-MM. Date cannot be newer than the current time (required)"`
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
	runCmd.AddCommand(runMigrationCmd)
	runCmd.AddCommand(runAPICmd)
	runCmd.AddCommand(runAdminCmd)
	runCmd.AddCommand(runRepairerCmd)
	runCmd.AddCommand(runGCCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(qdiagCmd)
	rootCmd.AddCommand(reportsCmd)
	rootCmd.AddCommand(compensationCmd)
	rootCmd.AddCommand(billingCmd)
	rootCmd.AddCommand(consistencyCmd)
	rootCmd.AddCommand(restoreTrashCmd)
	reportsCmd.AddCommand(nodeUsageCmd)
	reportsCmd.AddCommand(partnerAttributionCmd)
	reportsCmd.AddCommand(reportsGracefulExitCmd)
	reportsCmd.AddCommand(reportsVerifyGEReceiptCmd)
	compensationCmd.AddCommand(generateInvoicesCmd)
	compensationCmd.AddCommand(recordPeriodCmd)
	compensationCmd.AddCommand(recordOneOffPaymentsCmd)
	billingCmd.AddCommand(prepareCustomerInvoiceRecordsCmd)
	billingCmd.AddCommand(createCustomerInvoiceItemsCmd)
	billingCmd.AddCommand(createCustomerInvoiceCouponsCmd)
	billingCmd.AddCommand(createCustomerInvoicesCmd)
	billingCmd.AddCommand(finalizeCustomerInvoicesCmd)
	billingCmd.AddCommand(stripeCustomerCmd)
	consistencyCmd.AddCommand(consistencyGECleanupCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runMigrationCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAPICmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAdminCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runRepairerCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runGCCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(restoreTrashCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(qdiagCmd, &qdiagCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(nodeUsageCmd, &nodeUsageCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(generateInvoicesCmd, &generateInvoicesCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(recordPeriodCmd, &recordPeriodCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(recordOneOffPaymentsCmd, &recordOneOffPaymentsCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(reportsGracefulExitCmd, &reportsGracefulExitCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(reportsVerifyGEReceiptCmd, &reportsVerifyGracefulExitReceiptCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(partnerAttributionCmd, &partnerAttribtionCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(prepareCustomerInvoiceRecordsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerInvoiceItemsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerInvoiceCouponsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(finalizeCustomerInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(stripeCustomerCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(consistencyGECleanupCmd, &consistencyGECleanupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))

	if err := consistencyGECleanupCmd.MarkFlagRequired("before"); err != nil {
		panic(err)
	}
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	// inert constructors only ====

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:     "satellite-core",
		SaveRollupBatchSize: runCfg.Tally.SaveRollupBatchSize,
		ReadRollupBatchSize: runCfg.Tally.ReadRollupBatchSize,
	})
	if err != nil {
		return errs.New("Error starting master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	liveAccounting, err := live.OpenCache(ctx, log.Named("live-accounting"), runCfg.LiveAccounting)
	if err != nil {
		if !accounting.ErrSystemOrNetError.Has(err) || liveAccounting == nil {
			return errs.New("Error instantiating live accounting cache: %w", err)
		}

		log.Warn("Unable to connect to live accounting cache. Verify connection",
			zap.Error(err),
		)
	}
	defer func() {
		err = errs.Combine(err, liveAccounting.Close())
	}()

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
	defer func() {
		err = errs.Combine(err, rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx)))
	}()

	peer, err := satellite.New(log, identity, db, metabaseDB, revocationDB, liveAccounting, rollupsWriteCache, version.Build, &runCfg.Config, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	// okay, start doing stuff ====
	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, log, nil, runCfg.Identity.CertPath); err != nil {
		log.Warn("Failed to initialize telemetry batcher", zap.Error(err))
	}

	err = metabaseDB.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed metabase database version check.", zap.Error(err))
		return errs.New("failed metabase version check: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}

func cmdMigrationRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := satellitedb.Open(ctx, log.Named("migration"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-migration"})
	if err != nil {
		return errs.New("Error creating new master database connection for satellitedb migration: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for master database on satellite: %+v", err)
	}

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()
	err = metabaseDB.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating metabase tables: %+v", err)
	}

	return nil
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
	ctx, _ := process.Ctx(cmd)

	// open the master db
	database, err := satellitedb.Open(ctx, zap.L().Named("db"), qdiagCfg.Database, satellitedb.Options{ApplicationName: "satellite-qdiag"})
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
	fmt.Fprintln(w, "Segment StreamID\tSegment Position\tSegment Health\t")

	// populate the row fields
	for _, v := range list {
		fmt.Fprint(w, v.StreamID.String(), "\t", v.Position.Encode(), "\t", v.SegmentHealth, "\t")
	}

	// display the data
	return w.Flush()
}

func reportsVerifyGEReceipt(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	}

	// Check the node ID is valid
	nodeID, err := storj.NodeIDFromString(args[0])
	if err != nil {
		return errs.Combine(err, errs.New("Invalid node ID."))
	}

	return verifyGracefulExitReceipt(ctx, identity, nodeID, args[1])
}

func cmdReportsGracefulExit(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	start, end, err := reports.ParseRange(args[0], args[1])
	if err != nil {
		return err
	}

	// send output to stdout
	if reportsGracefulExitCfg.Output == "" {
		return generateGracefulExitCSV(ctx, reportsGracefulExitCfg.Completed, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(reportsGracefulExitCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	return generateGracefulExitCSV(ctx, reportsGracefulExitCfg.Completed, start, end, file)
}

func cmdNodeUsage(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	start, end, err := reports.ParseRange(args[0], args[1])
	if err != nil {
		return err
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

func cmdGenerateInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	period, err := compensation.PeriodFromString(args[0])
	if err != nil {
		return err
	}

	if err := runWithOutput(generateInvoicesCfg.Output, func(out io.Writer) error {
		return generateInvoicesCSV(ctx, period, out)
	}); err != nil {
		return err
	}

	if generateInvoicesCfg.Output != "" {
		fmt.Println("Generated invoices")
	}
	return nil
}

func cmdRecordPeriod(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	paystubsCount, paymentsCount, err := recordPeriod(ctx, args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Println(paystubsCount, "paystubs recorded")
	fmt.Println(paymentsCount, "payments recorded")
	return nil
}

func cmdRecordOneOffPayments(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	count, err := recordOneOffPayments(ctx, args[0])
	if err != nil {
		return err
	}
	fmt.Println(count, "payments recorded")
	return nil
}

func cmdValueAttribution(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L().Named("satellite-cli")

	partnerID, err := uuid.FromString(args[0])
	if err != nil {
		return errs.Combine(errs.New("Invalid Partner ID format. %s", args[0]), err)
	}

	start, end, err := reports.ParseRange(args[1], args[2])
	if err != nil {
		return err
	}

	// send output to stdout
	if partnerAttribtionCfg.Output == "" {
		return reports.GenerateAttributionCSV(ctx, partnerAttribtionCfg.Database, partnerID, start, end, os.Stdout)
	}

	// send output to file
	file, err := os.Create(partnerAttribtionCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
		if err != nil {
			log.Error("Error closing the output file after retrieving partner value attribution data.",
				zap.String("Output File", partnerAttribtionCfg.Output),
				zap.Error(err),
			)
		}
	}()

	return reports.GenerateAttributionCSV(ctx, partnerAttribtionCfg.Database, partnerID, start, end, file)
}

func cmdPrepareCustomerInvoiceRecords(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	period, err := parseBillingPeriod(args[0])
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, _ satellite.DB) error {
		return payments.PrepareInvoiceProjectRecords(ctx, period)
	})
}

func cmdCreateCustomerInvoiceItems(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	period, err := parseBillingPeriod(args[0])
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, _ satellite.DB) error {
		return payments.InvoiceApplyProjectRecords(ctx, period)
	})
}

func cmdCreateCustomerInvoiceCoupons(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	period, err := parseBillingPeriod(args[0])
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, _ satellite.DB) error {
		return payments.InvoiceApplyCoupons(ctx, period)
	})
}

func cmdCreateCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	period, err := parseBillingPeriod(args[0])
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, _ satellite.DB) error {
		return payments.CreateInvoices(ctx, period)
	})
}

func cmdFinalizeCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, _ satellite.DB) error {
		return payments.FinalizeInvoices(ctx)
	})
}

func cmdStripeCustomer(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return generateStripeCustomers(ctx)
}

func cmdConsistencyGECleanup(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	before, err := time.Parse("2006-01-02", consistencyGECleanupCfg.Before)
	if err != nil {
		return errs.New("before flag value isn't of the expected format. %+v", err)
	}

	if before.After(time.Now()) {
		return errs.New("before flag value cannot be newer than the current time.")
	}
	return cleanupGEOrphanedData(ctx, before.UTC(), runCfg.GracefulExit)
}

func cmdRestoreTrash(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := satellitedb.Open(ctx, log.Named("restore-trash"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-restore-trash"})
	if err != nil {
		return errs.New("Error creating new master database connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	tlsOptions, err := tlsopts.NewOptions(identity, runCfg.Server.Config, revocationDB)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	successes := new(int64)
	failures := new(int64)

	undelete := func(node *overlay.SelectedNode) {
		log.Info("starting restore trash", zap.String("Node ID", node.ID.String()))

		conn, err := dialer.DialNodeURL(ctx, storj.NodeURL{
			ID:      node.ID,
			Address: node.Address.Address,
		})
		if err != nil {
			atomic.AddInt64(failures, 1)
			log.Error("unable to connect", zap.String("Node ID", node.ID.String()), zap.Error(err))
			return
		}
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Error("close failure", zap.String("Node ID", node.ID.String()), zap.Error(err))
			}
		}()

		client := pb.NewDRPCPiecestoreClient(conn)
		_, err = client.RestoreTrash(ctx, &pb.RestoreTrashRequest{})
		if err != nil {
			atomic.AddInt64(failures, 1)
			log.Error("unable to restore trash", zap.String("Node ID", node.ID.String()), zap.Error(err))
			return
		}

		atomic.AddInt64(successes, 1)
		log.Info("successful restore trash", zap.String("Node ID", node.ID.String()))
	}

	var nodes []*overlay.SelectedNode
	if len(args) == 0 {
		err = db.OverlayCache().IterateAllNodes(ctx, func(ctx context.Context, node *overlay.SelectedNode) error {
			nodes = append(nodes, node)
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		for _, nodeid := range args {
			parsedNodeID, err := storj.NodeIDFromString(nodeid)
			if err != nil {
				return err
			}
			dossier, err := db.OverlayCache().Get(ctx, parsedNodeID)
			if err != nil {
				return err
			}
			nodes = append(nodes, &overlay.SelectedNode{
				ID:         dossier.Id,
				Address:    dossier.Address,
				LastNet:    dossier.LastNet,
				LastIPPort: dossier.LastIPPort,
			})
		}
	}

	limiter := sync2.NewLimiter(100)
	for _, node := range nodes {
		node := node
		limiter.Go(ctx, func() { undelete(node) })
	}
	limiter.Wait()

	log.Sugar().Infof("restore trash complete. %d successes, %d failures", *successes, *failures)
	return nil
}

func main() {
	process.ExecCustomDebug(rootCmd)
}
