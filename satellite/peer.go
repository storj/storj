// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	hw "github.com/jtolds/monkit-hw/v2"
	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/identity"
	"storj.io/private/debug"
	"storj.io/storj/private/server"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/accountmanagementapikeys"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/snopayouts"
)

var mon = monkit.Package()

func init() {
	hw.Register(monkit.Default)
}

// DB is the master database for the satellite.
//
// architecture: Master Database
type DB interface {
	// MigrateToLatest initializes the database
	MigrateToLatest(ctx context.Context) error
	// CheckVersion checks the database is the correct version
	CheckVersion(ctx context.Context) error
	// Close closes the database
	Close() error

	// TestingMigrateToLatest initializes the database for testplanet.
	TestingMigrateToLatest(ctx context.Context) error

	// PeerIdentities returns a storage for peer identities
	PeerIdentities() overlay.PeerIdentities
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// Reputation returns database for audit reputation information
	Reputation() reputation.DB
	// Attribution returns database for partner keys information
	Attribution() attribution.DB
	// StoragenodeAccounting returns database for storing information about storagenode use
	StoragenodeAccounting() accounting.StoragenodeAccounting
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Console returns database for satellite console
	Console() console.DB
	// OIDC returns the database for OIDC resources.
	OIDC() oidc.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Containment returns database for containment
	Containment() audit.Containment
	// Buckets returns the database to interact with buckets
	Buckets() buckets.DB
	// GracefulExit returns database for graceful exit
	GracefulExit() gracefulexit.DB
	// StripeCoinPayments returns stripecoinpayments database.
	StripeCoinPayments() stripecoinpayments.DB
	// SnoPayout returns database for payouts.
	SNOPayouts() snopayouts.DB
	// Compoensation tracks storage node compensation
	Compensation() compensation.DB
	// Revocation tracks revoked macaroons
	Revocation() revocation.DB
	// NodeAPIVersion tracks nodes observed api usage
	NodeAPIVersion() nodeapiversion.DB
}

// Config is the global config satellite.
type Config struct {
	Identity identity.Config
	Server   server.Config
	Debug    debug.Config

	Admin admin.Config

	Contact    contact.Config
	Overlay    overlay.Config
	StrayNodes straynodes.Config

	Metainfo metainfo.Config
	Orders   orders.Config

	Reputation reputation.Config

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	GarbageCollection gc.Config

	ExpiredDeletion expireddeletion.Config
	ZombieDeletion  zombiedeletion.Config

	Tally            tally.Config
	Rollup           rollup.Config
	RollupArchive    rolluparchive.Config
	LiveAccounting   live.Config
	ProjectBWCleanup projectbwcleanup.Config

	Mail mailservice.Config

	Payments paymentsconfig.Config

	AccountManagementAPIKeys accountmanagementapikeys.Config
	Console                  consoleweb.Config

	Version version_checker.Config

	GracefulExit gracefulexit.Config

	Metrics metrics.Config

	Compensation compensation.Config

	ProjectLimit accounting.ProjectLimitConfig

	Analytics analytics.Config
}
