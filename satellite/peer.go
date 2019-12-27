// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/identity"
	"storj.io/storj/pkg/server"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/referrals"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/rewards"
)

var mon = monkit.Package()

// DB is the master database for the satellite
//
// architecture: Master Database
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// CheckVersion checks the database is the correct version
	CheckVersion() error
	// Close closes the database
	Close() error

	// PeerIdentities returns a storage for peer identities
	PeerIdentities() overlay.PeerIdentities
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// Attribution returns database for partner keys information
	Attribution() attribution.DB
	// StoragenodeAccounting returns database for storing information about storagenode use
	StoragenodeAccounting() accounting.StoragenodeAccounting
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
	// Console returns database for satellite console
	Console() console.DB
	// Rewards returns database for marketing admin GUI
	Rewards() rewards.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Containment returns database for containment
	Containment() audit.Containment
	// Buckets returns the database to interact with buckets
	Buckets() metainfo.BucketsDB
	// GracefulExit returns database for graceful exit
	GracefulExit() gracefulexit.DB
	// StripeCoinPayments returns stripecoinpayments database.
	StripeCoinPayments() stripecoinpayments.DB
}

// Config is the global config satellite
type Config struct {
	Identity identity.Config
	Server   server.Config

	Contact contact.Config
	Overlay overlay.Config

	Metainfo metainfo.Config
	Orders   orders.Config

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	GarbageCollection gc.Config

	DBCleanup dbcleanup.Config

	Tally          tally.Config
	Rollup         rollup.Config
	LiveAccounting live.Config

	Mail mailservice.Config

	Payments paymentsconfig.Config

	Referrals referrals.Config

	Console consoleweb.Config

	Marketing marketingweb.Config

	Version version_checker.Config

	GracefulExit gracefulexit.Config

	Metrics metrics.Config
}
