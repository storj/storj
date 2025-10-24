// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	_ "storj.io/common/process/googleprofiler" // This attaches google cloud profiler.
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/version"
	"storj.io/storj/cmd/satellite/reports"
	"storj.io/storj/private/revocation"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/lrucache"
)

// Satellite defines satellite configuration.
type Satellite struct {
	Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	DatabaseOptions struct {
		APIKeysCache struct {
			Expiration time.Duration `help:"satellite database api key expiration" default:"60s"`
			Capacity   int           `help:"satellite database api key lru capacity" default:"10000"`
		}
		RevocationsCache struct {
			Expiration time.Duration `help:"macaroon revocation cache expiration" default:"5m"`
			Capacity   int           `help:"macaroon revocation cache capacity" default:"10000"`
		}
		MigrationUnsafe string `help:"comma separated migration types to run during every startup (none: no migration, snapshot: creating db from latest test snapshot (for testing only), testdata: create testuser in addition to a migration, full: do the normal migration (equals to 'satellite run migration'" default:"none" hidden:"true"`
	}

	satellite.Config

	UnsafeSkipDBVersionCheck bool `help:"skip database (satellite/metabase) version check, use with caution" default:"false" advanced:"true"`
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
	valueAttributionCmd = &cobra.Command{
		Use:   "populate-placement [batch-size]",
		Short: "Populate placement constraints in value_attributions from bucket_metainfos",
		Args:  cobra.MaximumNArgs(1),
		RunE:  cmdPopulatePlacementFromBucketMetainfos,
		Long:  "Populate placement constraints in value_attribution from bucket_metainfos. Optional batch-size parameter controls how many rows to process at once (default: 1000).",
	}
	runAPICmd = &cobra.Command{
		Use:   "api",
		Short: "Run the satellite API",
		RunE:  cmdAPIRun,
	}
	runConsoleAPICmd = &cobra.Command{
		Use:   "console-api",
		Short: "Run the satellite console API",
		RunE:  cmdConsoleAPIRun,
	}
	runUICmd = &cobra.Command{
		Use:   "ui",
		Short: "Run the satellite UI",
		RunE:  cmdUIRun,
	}
	runRepairerCmd = &cobra.Command{
		Use:   "repair",
		Short: "Run the repair service",
		RunE:  cmdRepairerRun,
	}
	runAuditorCmd = &cobra.Command{
		Use:   "auditor",
		Short: "Run the auditor service",
		RunE:  cmdAuditorRun,
	}
	runAdminCmd = &cobra.Command{
		Use:   "admin",
		Short: "Run the satellite Admin",
		RunE:  cmdAdminRun,
	}
	runGCCmd = &cobra.Command{
		Use:   "garbage-collection",
		Short: "Run the satellite garbage collection process to send generated bloom filters to storage nodes",
		RunE:  cmdGCRun,
	}
	runGCBloomFilterCmd = &cobra.Command{
		Use:   "garbage-collection-bloom-filters",
		Short: "Run the satellite garbage collection process which creates a bloom filter for each node",
		RunE:  cmdGCBloomFilterRun,
	}
	runRangedLoopCmd = &cobra.Command{
		Use:   "ranged-loop",
		Short: "Run the satellite segments ranged loop",
		RunE:  cmdRangedLoopRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	qdiagCmd = &cobra.Command{
		Use:   "qdiag",
		Short: "Repair queue Diagnostic Tool support",
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
		Use:   "partner-attribution [start] [end] [user-agent,...]",
		Short: "Generate a partner attribution report for a given period to use for payments",
		Long:  "Generate a partner attribution report for a given period to use for payments. Format dates using YYYY-MM-DD. The end date is exclusive. Optionally filter using a comma-separated list of user agents.",
		Args:  cobra.MinimumNArgs(2),
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
	applyFreeTierCouponsCmd = &cobra.Command{
		Use:   "apply-free-coupons",
		Short: "Applies free tier coupon to Stripe customers",
		Long:  "Applies free tier coupon to Stripe customers without a coupon",
		RunE:  cmdApplyFreeTierCoupons,
	}
	setInvoiceStatusCmd = &cobra.Command{
		Use:   "set-invoice-status [start-period] [end-period] [status]",
		Short: "set all open invoices status",
		Long:  "set all open invoices in the specified date ranges to the provided status. Period is a UTC date formatted like YYYY-MM.",
		Args:  cobra.ExactArgs(3),
		RunE:  cmdSetInvoiceStatus,
	}
	createCustomerBalanceInvoiceItemsCmd = &cobra.Command{
		Use:   "create-balance-invoice-items",
		Short: "Creates stripe invoice line items for stripe customer balance",
		Long:  "Creates stripe invoice line items for stripe customer balances obtained from past invoices and other miscellaneous charges.",
		RunE:  cmdCreateCustomerBalanceInvoiceItems,
	}

	prepareCustomerInvoiceRecordsCmd = &cobra.Command{
		Use:   "prepare-invoice-records [period]",
		Short: "Prepares invoice project records",
		Long:  "Prepares invoice project records that will be used during invoice line items creation.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdPrepareCustomerInvoiceRecords,
	}
	createCustomerProjectInvoiceItemsGroupedCmd = &cobra.Command{
		Use:   "create-project-invoice-items-grouped [period]",
		Short: "Creates stripe invoice line items for project charges grouped by project",
		Long:  "Creates stripe invoice line items for not consumed project records grouped by project",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCreateCustomerProjectInvoiceItemsGrouped,
	}
	createCustomerInvoicesCmd = &cobra.Command{
		Use:   "create-invoices [period]",
		Short: "Creates stripe invoices from pending invoice items",
		Long:  "Creates stripe invoices for all stripe customers known to satellite",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCreateCustomerInvoices,
	}
	generateCustomerInvoicesCmd = &cobra.Command{
		Use:   "generate-invoices [period]",
		Short: "Performs all tasks necessary to generate Stripe invoices",
		Long:  "Performs all tasks necessary to generate Stripe invoices. Equivalent to running apply-free-coupons, prepare-invoice-records, create-project-invoice-items, and create-invoices in order. Does not finalize invoices.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdGenerateCustomerInvoices,
	}
	finalizeCustomerInvoicesCmd = &cobra.Command{
		Use:   "finalize-invoices",
		Short: "Finalizes all draft stripe invoices",
		Long:  "Finalizes all draft stripe invoices known to satellite's stripe account.",
		RunE:  cmdFinalizeCustomerInvoices,
	}
	payInvoicesWithTokenCmd = &cobra.Command{
		Use:   "pay-customer-invoices",
		Short: "pay open finalized invoices for customer",
		Long:  "attempts payment on any open finalized invoices for a specific user.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdPayCustomerInvoices,
	}
	payAllInvoicesCmd = &cobra.Command{
		Use:   "pay-invoices",
		Short: "pay finalized invoices",
		Long:  "attempts payment on all open finalized invoices according to subscriptions settings.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdPayAllInvoices,
	}
	failPendingInvoiceTokenPaymentCmd = &cobra.Command{
		Use:   "fail-token-payment",
		Short: "fail pending invoice token payment",
		Long:  "attempts to transition the token invoice payments that are stuck in a pending state to failed.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdFailPendingInvoiceTokenPayments,
	}
	completePendingInvoiceTokenPaymentCmd = &cobra.Command{
		Use:   "complete-token-payment",
		Short: "complete pending invoice token payment",
		Long:  "attempts to transition the token invoice payments that are stuck in a pending state to complete.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCompletePendingInvoiceTokenPayments,
	}
	stripeCustomerCmd = &cobra.Command{
		Use:   "ensure-stripe-customer",
		Short: "Ensures that we have a stripe customer for every user",
		Long:  "Ensures that we have a stripe customer for every satellite user.",
		RunE:  cmdStripeCustomer,
	}
	generateListOfReusedCardFingerprints = &cobra.Command{
		Use:   "list-reused-card-fingerprints [min-customers] [csv-path]",
		Short: "List reused card fingerprints",
		Long:  "List reused card fingerprints for all the customers.",
		Args:  cobra.ExactArgs(2),
		RunE:  cmdGenerateListOfReusedCardFingerprints,
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
	registerLostSegments = &cobra.Command{
		Use:   "register-lost-segments [number_of_segments_lost]",
		Short: "Register permanently lost segments for our statistics",
		Long:  "Send metric information through monkit indicating the (permanent) loss of some number of segments. Temporarily unavailable segments are reported automatically by the repair checker and do not need to be reported here.",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdRegisterLostSegments,
	}
	fetchPiecesCmd = &cobra.Command{
		Use:   "fetch-pieces <stream-id> <position> <output-dir>",
		Short: "Retrieve pieces of a segment from all responding nodes",
		Args:  cobra.ExactArgs(3),
		RunE:  cmdFetchPieces,
	}
	repairSegmentCmd = &cobra.Command{
		Use:   "repair-segment <csv-file> or <stream-id> <position>",
		Short: "Repair segment and verify all downloadable pieces",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  cmdRepairSegment,
	}
	fixLastNetsCmd = &cobra.Command{
		Use:   "fix-last-nets",
		Short: "Fix last_net entries in the database for satellites with DistinctIP=false",
		RunE:  cmdFixLastNets,
	}

	entitlementsCmd = &cobra.Command{
		Use:   "entitlements",
		Short: "Entitlements administration",
		Long:  "Operations to administrate satellite entitlements",
	}
	projectEntitlementsCmd = &cobra.Command{
		Use:   "projects",
		Short: "Project entitlements administration",
		Long:  "Operations to administrate satellite project entitlements",
	}
	setNewBucketPlacementsCmd = &cobra.Command{
		Use:   "set-new-bucket-placements",
		Short: "Set NewBucketPlacements entitlement for projects",
		Long:  "Set NewBucketPlacements entitlement for projects to be reused during new bucket creation",
		RunE:  cmdSetNewBucketPlacements,
	}
	setPlacementProductMapCmd = &cobra.Command{
		Use:   "set-placement-product-mappings",
		Short: "Set PlacementProductMappings entitlement for projects",
		Long:  "Set PlacementProductMappings entitlement for projects to override global configs",
		RunE:  cmdSetPlacementProductMap,
	}

	usersCmd = &cobra.Command{
		Use:   "user-accounts",
		Short: "User accounts administration",
		Long:  "Operations to administrate satellite users accounts",
	}

	batchSizeDeleteObjects           = 100
	useDeleteAllObjectsUncoordinated = false
	deleteObjectsCmd                 = &cobra.Command{
		Use:   "delete-objects",
		Short: "Delete objects and their segments",
		Long: "Delete from a list of users accounts their objects and segments.\nAccounts must be on " +
			"'pending deletion' status, when not they are logged with an info message and skipped. " +
			"Unexisting accounts are logged with an debug message and skipped.\nSystem errors exit the " +
			"process with an error message.\nThe command can operate on one account or on multiple " +
			"accounts; when the passed possitional argmuent is a string that contains '@', the command " +
			"considers that's the email address of the user to delete its data, otherwise it considers " +
			"a path to a CSV file where the first column contains the email of the user's account and " +
			"if the first row doesn't contain '@' is considered the header and skipped.",
		Args: cobra.ExactArgs(1),
		RunE: cmdDeleteObjects,
	}

	executeDeleteAllObjectsUncoordinated = false
	deleteAllObjectsUncoordinatedCmd     = &cobra.Command{
		Use:   "delete-all-objects-uncoordinated <public-project-id> <bucket-name> <owner-email>",
		Short: "Delete all the objects in a bucket",
		Long: "Deletes the objects in a given bucket, but does not guarantee consistency, while the " +
			"delete is in progress. There must be no uploads, downloads or deletes happening while this is being run." +
			"On failure, the system should check whether there are any undeleted streams or objects in the system. " +
			"The owner-email argument is used to verify that the provided project and bucket are owned by the user with this email address.",
		Args: cobra.ExactArgs(3),
		RunE: cmdDeleteAllObjectsUncoordinated,
	}

	deleteNonExistingBucketObjectsCmd = &cobra.Command{
		Use:   "delete-non-existing-bucket-objects <project-id> <bucket-name>",
		Short: "Delete all the objects of an unexisting bucket",
		Long: "Deletes the objects in a given bucket, but does not guarantee consistency, while the " +
			"delete is in progress. Bucket must not exists at the moment when command is executed otherwise " +
			"command will return error",
		Args: cobra.ExactArgs(2),
		RunE: cmdDeleteNonExistingBucketObjects,
	}

	deleteAccountsCmd = &cobra.Command{
		Use:   "delete-accounts",
		Short: "Delete accounts and their associated entities",
		Long: "From the list of users accounts it redacts the users' personal information and marks " +
			"their accounts as deleted, deactivate their projects, and delete their API keys.\n The " +
			"accounts must be on 'pending deletion' status, otherwise they are logged with an info " +
			"message and skipped. The accounts must not have data, otherwise they are logged as an " +
			"error message and skipped. Unexisting accounts are logged with a debug  message and " +
			"skipped.\nSystem errors exit the process with an error message.\nThe command can operate " +
			"on one account or on multiple accounts; when the passed positional argument is a string " +
			"that contains '@', the command considers that's the email address of the user to delete " +
			"its data, otherwise it considers a path to a CSV file where the first column contains the " +
			"email of the user's account and if the first row doesn't contain '@' is considered the " +
			"header and skipped.",
		Args: cobra.ExactArgs(1),
		RunE: cmdDeleteAccounts,
	}

	setAccountsStatusPendingDeletionCmd = &cobra.Command{
		Use:   "set-status-pending-deletion",
		Short: "Safely change accounts to pending deletion status for accounts with trial expiration freeze",
		Long: "Changes the status of user accounts to 'pending deletion', but only if certain safety criteria are met:\n" +
			"1. The account is currently in 'active' status\n" +
			"2. The account is NOT in the paid tier\n" +
			"3. The account has an active 'trial expiration freeze'\n" +
			"4. The active 'trial expiration freeze' days till escalation is over'\n" +
			"5. The account is NOT a member of any third party project\n\n" +
			"Accounts that don't meet these criteria are logged and skipped.\n" +
			"The command can operate on one account or on multiple accounts; when the passed positional argument " +
			"is a string that contains '@', the command considers that's the email address of the user to change its " +
			"status, otherwise it considers a path to a CSV file where the first column contains the " +
			"email of the user's account and if the first row doesn't contain '@' is considered the " +
			"header and skipped.",
		Args: cobra.ExactArgs(1),
		RunE: cmdSetAccountsStatusPendingDeletion,
	}

	runCfg   Satellite
	setupCfg Satellite

	qdiagCfg struct {
		Database   string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		JobQueue   jobq.Config
		Identity   identity.Config
		QListLimit int `help:"maximum segments that can be requested" default:"1000"`
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
	partnerAttributionCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output   string `help:"destination of report output" default:""`
	}
	reportsGracefulExitCfg struct {
		Database  string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Output    string `help:"destination of report output" default:""`
		Completed bool   `help:"whether to output (initiated and completed) or (initiated and not completed)" default:"false"`
		TimeBased bool   `help:"whether the satellite is using time-based graceful exit (and thus, whether to include piece transfer progress in output)" default:"true" hidden:"true"`
	}
	reportsVerifyGracefulExitReceiptCfg struct {
	}
	consistencyGECleanupCfg struct {
		Database string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
		Before   string `help:"select only exited nodes before this UTC date formatted like YYYY-MM. Date cannot be newer than the current time (required)"`
	}
	setInvoiceStatusCfg struct {
		DryRun bool `help:"do not update stripe" default:"false"`
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
	runCmd.AddCommand(runConsoleAPICmd)
	runCmd.AddCommand(runUICmd)
	runCmd.AddCommand(runAdminCmd)
	runCmd.AddCommand(runRepairerCmd)
	runCmd.AddCommand(runAuditorCmd)
	runCmd.AddCommand(runGCCmd)
	runCmd.AddCommand(runGCBloomFilterCmd)
	runCmd.AddCommand(runRangedLoopCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(qdiagCmd)
	rootCmd.AddCommand(reportsCmd)
	rootCmd.AddCommand(compensationCmd)
	rootCmd.AddCommand(billingCmd)
	rootCmd.AddCommand(consistencyCmd)
	rootCmd.AddCommand(restoreTrashCmd)
	rootCmd.AddCommand(registerLostSegments)
	rootCmd.AddCommand(fetchPiecesCmd)
	rootCmd.AddCommand(repairSegmentCmd)
	rootCmd.AddCommand(fixLastNetsCmd)
	rootCmd.AddCommand(usersCmd)
	rootCmd.AddCommand(valueAttributionCmd)
	rootCmd.AddCommand(entitlementsCmd)
	reportsCmd.AddCommand(nodeUsageCmd)
	reportsCmd.AddCommand(partnerAttributionCmd)
	reportsCmd.AddCommand(reportsGracefulExitCmd)
	reportsCmd.AddCommand(reportsVerifyGEReceiptCmd)
	compensationCmd.AddCommand(generateInvoicesCmd)
	compensationCmd.AddCommand(recordPeriodCmd)
	compensationCmd.AddCommand(recordOneOffPaymentsCmd)
	billingCmd.AddCommand(applyFreeTierCouponsCmd)
	billingCmd.AddCommand(setInvoiceStatusCmd)
	billingCmd.AddCommand(createCustomerBalanceInvoiceItemsCmd)
	billingCmd.AddCommand(prepareCustomerInvoiceRecordsCmd)
	billingCmd.AddCommand(createCustomerProjectInvoiceItemsGroupedCmd)
	billingCmd.AddCommand(createCustomerInvoicesCmd)
	billingCmd.AddCommand(generateCustomerInvoicesCmd)
	billingCmd.AddCommand(finalizeCustomerInvoicesCmd)
	billingCmd.AddCommand(payInvoicesWithTokenCmd)
	billingCmd.AddCommand(payAllInvoicesCmd)
	billingCmd.AddCommand(failPendingInvoiceTokenPaymentCmd)
	billingCmd.AddCommand(completePendingInvoiceTokenPaymentCmd)
	billingCmd.AddCommand(stripeCustomerCmd)
	billingCmd.AddCommand(generateListOfReusedCardFingerprints)
	entitlementsCmd.AddCommand(projectEntitlementsCmd)
	projectEntitlementsCmd.AddCommand(setNewBucketPlacementsCmd)
	setNewBucketPlacementsCmd.Flags().StringVar(&entitlementUserEmail, "email", "", "Set bucket placements for all active projects of a specific user by email")
	setNewBucketPlacementsCmd.Flags().StringVar(&entitlementUserEmailCSV, "csv", "", "Set bucket placements for all active projects of users listed in CSV file")
	setNewBucketPlacementsCmd.Flags().StringVar(&entitlementJSON, "placements", "", "JSON array of placement IDs to set (e.g., \"[0, 12]\"). If not provided, uses satellite config defaults")
	setNewBucketPlacementsCmd.Flags().BoolVar(&entitlementSkipConfirm, "skip-confirmation", false, "Skip confirmation prompt for bulk operations")
	setNewBucketPlacementsCmd.Flags().BoolVar(&entitlementVerbose, "verbose", false, "Whether to log info about each processed project")
	projectEntitlementsCmd.AddCommand(setPlacementProductMapCmd)
	setPlacementProductMapCmd.Flags().StringVar(&entitlementUserEmail, "email", "", "Set placement-product mapping for all active projects of a specific user by email")
	setPlacementProductMapCmd.Flags().StringVar(&entitlementUserEmailCSV, "csv", "", "Set placement-product mapping for all active projects of users listed in CSV file")
	setPlacementProductMapCmd.Flags().StringVar(&entitlementJSON, "placements", "", "1:1 JSON mapping of placement to product ID to set (e.g., \"{0:3,12:2}\"). If not provided, uses satellite config defaults")
	setPlacementProductMapCmd.Flags().BoolVar(&entitlementSkipConfirm, "skip-confirmation", false, "Skip confirmation prompt for bulk operations")
	setPlacementProductMapCmd.Flags().BoolVar(&entitlementVerbose, "verbose", false, "Whether to log info about each processed project")
	consistencyCmd.AddCommand(consistencyGECleanupCmd)
	usersCmd.AddCommand(deleteObjectsCmd)
	deleteObjectsCmd.Flags().IntVar(&batchSizeDeleteObjects, "batch-size", 100, "Number of objects/segments to delete in a single batch")
	deleteObjectsCmd.Flags().BoolVar(&useDeleteAllObjectsUncoordinated, "use-delete-all-objects-uncoordinated", false, "Delete all objects with a more performant way at the expense of no consistency guarantee")
	usersCmd.AddCommand(deleteAccountsCmd)
	usersCmd.AddCommand(deleteAllObjectsUncoordinatedCmd)
	deleteAllObjectsUncoordinatedCmd.Flags().IntVar(&batchSizeDeleteObjects, "batch-size", 100, "Number of objects/segments to delete in a single batch")
	deleteAllObjectsUncoordinatedCmd.Flags().BoolVar(&executeDeleteAllObjectsUncoordinated, "really-run-this-dangerous-command-without-any-confirmation", false, "This disables bucket reconfirmation.")
	usersCmd.AddCommand(deleteNonExistingBucketObjectsCmd)
	usersCmd.AddCommand(setAccountsStatusPendingDeletionCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runMigrationCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAPICmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runConsoleAPICmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runUICmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAdminCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runRepairerCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runAuditorCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runGCCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runGCBloomFilterCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(runRangedLoopCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(restoreTrashCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(registerLostSegments, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(fetchPiecesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(repairSegmentCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(qdiagCmd, &qdiagCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(nodeUsageCmd, &nodeUsageCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(generateInvoicesCmd, &generateInvoicesCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(recordPeriodCmd, &recordPeriodCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(recordOneOffPaymentsCmd, &recordOneOffPaymentsCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(reportsGracefulExitCmd, &reportsGracefulExitCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(reportsVerifyGEReceiptCmd, &reportsVerifyGracefulExitReceiptCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(partnerAttributionCmd, &partnerAttributionCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(applyFreeTierCouponsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setInvoiceStatusCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setInvoiceStatusCmd, &setInvoiceStatusCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerBalanceInvoiceItemsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(prepareCustomerInvoiceRecordsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerProjectInvoiceItemsGroupedCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(createCustomerInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(generateCustomerInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(finalizeCustomerInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(payInvoicesWithTokenCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(payAllInvoicesCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(failPendingInvoiceTokenPaymentCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(completePendingInvoiceTokenPaymentCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(stripeCustomerCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(generateListOfReusedCardFingerprints, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(consistencyGECleanupCmd, &consistencyGECleanupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(fixLastNetsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(deleteObjectsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(deleteAllObjectsUncoordinatedCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(deleteNonExistingBucketObjectsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(deleteAccountsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setAccountsStatusPendingDeletionCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(valueAttributionCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setNewBucketPlacementsCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setPlacementProductMapCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))

	if err := consistencyGECleanupCmd.MarkFlagRequired("before"); err != nil {
		panic(err)
	}
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	// inert constructors only ====

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

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

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-core"))
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

	var repairQueue queue.RepairQueue
	if !runCfg.JobQueue.ServerNodeURL.IsZero() {
		repairQueue, err = jobq.OpenJobQueue(ctx, identity, runCfg.JobQueue)
		if err != nil {
			return errs.New("opening jobq connection: %+v", err)
		}
	} else {
		repairQueue = db.RepairQueue()
	}

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

	peer, err := satellite.New(log, identity, db, metabaseDB, revocationDB, repairQueue, liveAccounting, version.Build, &runCfg.Config, process.AtomicLevel(cmd))
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

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-migration"))
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

	var repairQueue queue.RepairQueue
	if !qdiagCfg.JobQueue.ServerNodeURL.IsZero() {
		identity, err := qdiagCfg.Identity.Load()
		if err != nil {
			return errs.New("could not load identity: %+v", err)
		}
		repairQueue, err = jobq.OpenJobQueue(ctx, identity, qdiagCfg.JobQueue)
		if err != nil {
			return errs.Wrap(err)
		}
	} else {
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

		repairQueue = database.RepairQueue()
	}

	list, err := repairQueue.SelectN(context.Background(), qdiagCfg.QListLimit)
	if err != nil {
		return err
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	_, _ = fmt.Fprintln(w, "Segment StreamID\tSegment Position\tSegment Health\t")

	// populate the row fields
	for _, v := range list {
		_, _ = fmt.Fprintln(w, v.StreamID.String(), "\t", v.Position.Encode(), "\t", v.SegmentHealth, "\t")
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

	start, end, err := reports.ParseRange(args[0], args[1])
	if err != nil {
		return err
	}

	var userAgents []string
	if len(args) > 2 {
		userAgents = strings.Split(args[2], ",")
	}

	// send output to stdout
	if partnerAttributionCfg.Output == "" {
		return reports.GenerateAttributionCSV(ctx, partnerAttributionCfg.Database, start, end, userAgents, os.Stdout)
	}

	// send output to file
	file, err := os.Create(partnerAttributionCfg.Output)
	if err != nil {
		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
		if err != nil {
			log.Error("Error closing the output file after retrieving partner value attribution data.",
				zap.String("Output File", partnerAttributionCfg.Output),
				zap.Error(err),
			)
		}
	}()

	return reports.GenerateAttributionCSV(ctx, partnerAttributionCfg.Database, start, end, userAgents, file)
}

// cmdSetInvoiceStatus sets the status of all open invoices within the provided period to the provided status.
// args[0] is the start of the period in YYYY-MM format.
// args[1] is the end of the period in YYYY-MM format.
// args[2] is the status to set the invoices to.
func cmdSetInvoiceStatus(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	periodEnd, err := parseYearMonth(args[1])
	if err != nil {
		return err
	}
	// parseYearMonth returns the first day of the month, but we want the period end to be the last day of the month
	periodEnd = periodEnd.AddDate(0, 1, -1)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.SetInvoiceStatus(ctx, periodStart, periodEnd, args[2], setInvoiceStatusCfg.DryRun)
	})
}

func cmdCreateCustomerBalanceInvoiceItems(cmd *cobra.Command, _ []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.CreateBalanceInvoiceItems(ctx)
	})
}

func cmdPrepareCustomerInvoiceRecords(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.PrepareInvoiceProjectRecords(ctx, periodStart)
	})
}

func cmdCreateCustomerProjectInvoiceItemsGrouped(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.InvoiceApplyProjectRecordsGrouped(ctx, periodStart)
	})
}

func cmdCreateCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.CreateInvoices(ctx, periodStart)
	})
}

func cmdGenerateCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.GenerateInvoices(ctx, periodStart)
	})
}

func cmdFinalizeCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.FinalizeInvoices(ctx)
	})
}

func cmdPayCustomerInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		err := payments.InvoiceApplyCustomerTokenBalance(ctx, args[0])
		if err != nil {
			return errs.New("error applying native token payments to invoice for customer: %v", err)
		}
		return payments.PayCustomerInvoices(ctx, args[0])
	})
}

func cmdPayAllInvoices(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	periodStart, err := parseYearMonth(args[0])
	if err != nil {
		return err
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		err := payments.InvoiceApplyTokenBalance(ctx, periodStart)
		if err != nil {
			return errs.New("error applying native token payments: %v", err)
		}
		return payments.PayInvoices(ctx, periodStart)
	})
}

func cmdFailPendingInvoiceTokenPayments(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.FailPendingInvoiceTokenPayments(ctx, strings.Split(args[0], ","))
	})
}

func cmdCompletePendingInvoiceTokenPayments(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.CompletePendingInvoiceTokenPayments(ctx, strings.Split(args[0], ","))
	})
}

func cmdStripeCustomer(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return generateStripeCustomers(ctx)
}

func cmdGenerateListOfReusedCardFingerprints(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	minCustomers, err := strconv.Atoi(args[0])
	if err != nil {
		return errs.New("invalid numeric argument for minimum customers: %v", err)
	}

	return getListOfReusedCardFingerprints(ctx, minCustomers, args[1])
}

func cmdConsistencyGECleanup(cmd *cobra.Command, args []string) error {
	return errs.New("this command is not supported with time-based graceful exit")
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
	nonexistent := new(int64)

	undelete := func(node *nodeselection.SelectedNode) {
		log.Info("starting restore trash", zap.String("Node ID", node.ID.String()))

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

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

	var nodes []*nodeselection.SelectedNode
	if len(args) == 0 {
		err = db.OverlayCache().IterateAllContactedNodes(ctx, func(ctx context.Context, node *nodeselection.SelectedNode) error {
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
				log.Error("unable to parse node id", zap.String("Node ID", nodeid), zap.Error(err))
				atomic.AddInt64(nonexistent, 1)
				continue
			}
			dossier, err := db.OverlayCache().Get(ctx, parsedNodeID)
			if err != nil {
				log.Error("unable to find node id", zap.String("Node ID", nodeid), zap.Error(err))
				atomic.AddInt64(nonexistent, 1)
				continue
			}
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID:         dossier.Id,
				Address:    dossier.Address,
				LastNet:    dossier.LastNet,
				LastIPPort: dossier.LastIPPort,
			})
		}
	}

	mathrand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	limiter := sync2.NewLimiter(100)
	for _, node := range nodes {
		node := node
		limiter.Go(ctx, func() { undelete(node) })
	}
	limiter.Wait()

	log.Sugar().Infof("restore trash complete. %d successes, %d failures, %d nonexistent", *successes, *failures, *nonexistent)
	return nil
}

func cmdRegisterLostSegments(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	numLostSegments, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatal("invalid numeric argument", zap.String("argument", args[0]))
	}
	if err := process.InitMetricsWithCertPath(ctx, log, nil, runCfg.Identity.CertPath); err != nil {
		log.Fatal("Failed to initialize telemetry batcher", zap.Error(err))
	}

	scope := monkit.Default.ScopeNamed("segment_durability")
	scope.Meter("lost_segments").Mark(numLostSegments)

	if err := process.Report(ctx); err != nil {
		log.Fatal("could not send telemetry", zap.Error(err))
	}
	// we can't actually tell whether metrics is really enabled at this point;
	// process.InitMetrics...() can return a nil error while disabling metrics
	// entirely. make sure that's clear to the user.
	log.Info("lost segment event(s) sent (if metrics are actually enabled)", zap.Int("lost-segments", numLostSegments))

	return nil
}

func cmdFixLastNets(cmd *cobra.Command, _ []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	if runCfg.Overlay.Node.DistinctIP {
		log.Info("No fix necessary; DistinctIP=true")
		return nil
	}
	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName: "satellite-fix-last-nets",
	})
	if err != nil {
		return fmt.Errorf("error opening master database: %w", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	return db.OverlayCache().OneTimeFixLastNets(ctx)
}

func main() {
	logger, _, _ := process.NewLogger("satellite")
	zap.ReplaceGlobals(logger)

	process.ExecCustomDebug(rootCmd)
}
