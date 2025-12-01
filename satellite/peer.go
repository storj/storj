// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"net"
	"net/mail"
	"net/smtp"

	hw "github.com/jtolds/monkit-hw/v2"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/private/migrate"
	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
	"storj.io/storj/private/server"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accountfreeze"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/bucketmigrations"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/dbcleanup"
	"storj.io/storj/satellite/console/dbcleanup/pendingdelete"
	"storj.io/storj/satellite/console/emailreminders"
	"storj.io/storj/satellite/console/userinfo"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/durability"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/gc/piecetracker"
	"storj.io/storj/satellite/gc/sender"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/nodeselection/tracker"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/offlinenodes"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/snopayouts"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/flightrecorder"
	"storj.io/storj/shared/tagsql"
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

	// PeerIdentities returns a storage for peer identities
	PeerIdentities() overlay.PeerIdentities
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// NodeEvents returns a database for node event information
	NodeEvents() nodeevents.DB
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
	// VerifyQueue returns queue for segments chosen for verification
	VerifyQueue() audit.VerifyQueue
	// ReverifyQueue returns queue for pieces that need audit reverification
	ReverifyQueue() audit.ReverifyQueue
	// Console returns database for satellite console
	Console() console.DB
	// AdminChangeHistory returns the database for storing admin change history.
	AdminChangeHistory() changehistory.DB
	// OIDC returns the database for OIDC resources.
	OIDC() oidc.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Containment returns database for containment
	Containment() audit.Containment
	// Buckets returns the database to interact with buckets
	Buckets() buckets.DB
	// BucketMigrations returns the database to interact with bucket migrations
	BucketMigrations() bucketmigrations.DB
	// StripeCoinPayments returns stripecoinpayments database.
	StripeCoinPayments() stripe.DB
	// Billing returns storjscan transactions database.
	Billing() billing.TransactionsDB
	// Wallets returns storjscan wallets database.
	Wallets() storjscan.WalletsDB
	// SNOPayouts returns database for payouts.
	SNOPayouts() snopayouts.DB
	// Compensation tracks storage node compensation
	Compensation() compensation.DB
	// Revocation tracks revoked macaroons
	Revocation() revocation.DB
	// NodeAPIVersion tracks nodes observed api usage
	NodeAPIVersion() nodeapiversion.DB
	// StorjscanPayments stores payments retrieved from storjscan.
	StorjscanPayments() storjscan.PaymentsDB

	// Testing provides access to testing facilities. These should not be used in production code.
	Testing() TestingDB
}

// TestingDB defines access to database testing facilities.
type TestingDB interface {
	// Implementation returns the implementations of the databases.
	Implementation() []dbutil.Implementation
	// Rebind adapts a query's syntax for a database dialect.
	Rebind(query string) string
	// RawDB returns the underlying database connection to the primary database.
	RawDB() tagsql.DB
	// Schema returns the full schema for the database.
	Schema() []string
	// TestMigrateToLatest initializes the database for testplanet.
	TestMigrateToLatest(ctx context.Context) error
	// ProductionMigration returns the primary migration.
	ProductionMigration() *migrate.Migration
	// TestMigration returns the migration used for tests.
	TestMigration() *migrate.Migration
}

// Config is the global config satellite.
type Config struct {
	Identity identity.Config
	Server   server.Config
	Debug    debug.Config

	Placement nodeselection.ConfigurablePlacementRule `help:"detailed placement rules in the form 'id:definition;id:definition;...' where id is a 16 bytes integer (use >10 for backward compatibility), definition is a combination of the following functions:country(2 letter country codes,...), tag(nodeId, key, bytes(value)) all(...,...)."`

	Admin admin.Config

	Contact      contact.Config
	Overlay      overlay.Config
	OfflineNodes offlinenodes.Config
	NodeEvents   nodeevents.Config
	StrayNodes   straynodes.Config

	BucketEventing eventingconfig.Config
	Metainfo       metainfo.Config
	Orders         orders.Config

	Userinfo userinfo.Config

	Reputation reputation.Config

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	GarbageCollection   sender.Config
	GarbageCollectionBF bloomfilter.Config

	RepairQueueCheck repairer.QueueStatConfig
	JobQueue         jobq.Config

	RangedLoop rangedloop.Config
	Durability durability.Config

	ExpiredDeletion expireddeletion.Config
	ZombieDeletion  zombiedeletion.Config

	Tally            tally.Config
	NodeTally        nodetally.Config
	Rollup           rollup.Config
	RollupArchive    rolluparchive.Config
	LiveAccounting   live.Config
	ProjectBWCleanup projectbwcleanup.Config

	Mail         mailservice.Config
	HubspotMails hubspotmails.Config

	Payments paymentsconfig.Config

	Console          consoleweb.Config
	Entitlements     entitlements.Config
	Valdi            valdi.Config
	ConsoleAuth      consoleauth.Config
	EmailReminders   emailreminders.Config
	ConsoleDBCleanup dbcleanup.Config

	PendingDeleteCleanup pendingdelete.Config

	Emission emission.Config

	AccountFreeze accountfreeze.Config

	Version version_checker.Config

	GracefulExit gracefulexit.Config

	Compensation compensation.Config

	Analytics analytics.Config

	PieceTracker piecetracker.Config

	DurabilityReport durability.ReportConfig

	KeyManagement kms.Config

	SSO sso.Config

	HealthCheck healthcheck.Config

	FlightRecorder flightrecorder.Config

	TagAuthorities string `help:"comma-separated paths of additional cert files, used to validate signed node tags"`

	PrometheusTracker tracker.PrometheusTrackerConfig

	DisableConsoleFromSatelliteAPI bool `help:"indicates whether the console API should not be served along with satellite API" default:"false"`

	StandaloneConsoleAPIEnabled bool `help:"indicates whether the console API should be served as a standalone service" default:"false"`
}

func setupMailService(log *zap.Logger, mailConfig mailservice.Config) (*mailservice.Service, error) {
	fromAndHost := func(cfg mailservice.Config) (*mail.Address, string, error) {
		// validate from mail address
		from, err := mail.ParseAddress(cfg.From)
		if err != nil {
			return nil, "", errs.New("SMTP from address '%s' couldn't be parsed: %v", cfg.From, err)
		}

		// validate smtp server address
		host, _, err := net.SplitHostPort(cfg.SMTPServerAddress)
		if err != nil && cfg.AuthType != "simulate" && cfg.AuthType != "nologin" {
			return nil, "", errs.New("SMTP server address '%s' couldn't be parsed: %v", cfg.SMTPServerAddress, err)
		}
		return from, host, err
	}

	// TODO(yar): test multiple satellites using same OAUTH credentials

	var sender mailservice.Sender
	switch mailConfig.AuthType {
	case "oauth2":
		creds := oauth2.Credentials{
			ClientID:     mailConfig.ClientID,
			ClientSecret: mailConfig.ClientSecret,
			TokenURI:     mailConfig.TokenURI,
		}
		token, err := oauth2.RefreshToken(context.TODO(), creds, mailConfig.RefreshToken)
		if err != nil {
			return nil, err
		}

		from, _, err := fromAndHost(mailConfig)
		if err != nil {
			return nil, err
		}

		sender = &post.SMTPSender{
			From: *from,
			Auth: &oauth2.Auth{
				UserEmail: from.Address,
				Storage:   oauth2.NewTokenStore(creds, *token),
			},
			ServerAddress: mailConfig.SMTPServerAddress,
		}
	case "plain":
		from, host, err := fromAndHost(mailConfig)
		if err != nil {
			return nil, err
		}

		sender = &post.SMTPSender{
			From:          *from,
			Auth:          smtp.PlainAuth("", mailConfig.Login, mailConfig.Password, host),
			ServerAddress: mailConfig.SMTPServerAddress,
		}
	case "login":
		from, _, err := fromAndHost(mailConfig)
		if err != nil {
			return nil, err
		}

		sender = &post.SMTPSender{
			From: *from,
			Auth: post.LoginAuth{
				Username: mailConfig.Login,
				Password: mailConfig.Password,
			},
			ServerAddress: mailConfig.SMTPServerAddress,
		}
	case "insecure":
		from, _, err := fromAndHost(mailConfig)
		if err != nil {
			return nil, err
		}
		sender = &post.SMTPSender{
			From:          *from,
			ServerAddress: mailConfig.SMTPServerAddress,
		}
	case "nomail":
		sender = simulate.NoMail{}
	default:
		sender = simulate.NewDefaultLinkClicker(log.Named("mail:linkclicker"))
	}

	return mailservice.New(
		log.Named("mail:service"),
		sender,
		mailConfig.TemplatePath,
	)
}
