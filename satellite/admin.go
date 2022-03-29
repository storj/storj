// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console/accountmanagementapikeys"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Admin is the satellite core process that runs chores.
//
// architecture: Peer
type Admin struct {
	// core dependencies
	Log        *zap.Logger
	Identity   *identity.FullIdentity
	DB         DB
	MetabaseDB *metabase.DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Version struct {
		Chore   *checker.Chore
		Service *checker.Service
	}

	Payments struct {
		Accounts payments.Accounts
		Service  *stripecoinpayments.Service
		Stripe   stripecoinpayments.StripeClient
	}

	Admin struct {
		Listener net.Listener
		Server   *admin.Server
	}

	Buckets struct {
		Service *buckets.Service
	}

	AccountManagementAPIKeys struct {
		Service *accountmanagementapikeys.Service
	}
}

// NewAdmin creates a new satellite admin peer.
func NewAdmin(log *zap.Logger, full *identity.FullIdentity, db DB, metabaseDB *metabase.DB,
	versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel) (*Admin, error) {
	peer := &Admin{
		Log:        log,
		Identity:   full,
		DB:         db,
		MetabaseDB: metabaseDB,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{
		peer.Buckets.Service = buckets.NewService(db.Buckets(), metabaseDB)
	}

	{ // setup account management api keys
		peer.AccountManagementAPIKeys.Service = accountmanagementapikeys.NewService(db.OIDC().OAuthTokens(), config.AccountManagementAPIKeys)
	}

	{ // setup debug
		var err error
		if config.Debug.Address != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Address)
			if err != nil {
				withoutStack := errors.New(err.Error())
				peer.Log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "Admin"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{
		if !versionInfo.IsZero() {
			peer.Log.Debug("Version info",
				zap.Stringer("Version", versionInfo.Version.Version),
				zap.String("Commit Hash", versionInfo.CommitHash),
				zap.Stringer("Build Timestamp", versionInfo.Timestamp),
				zap.Bool("Release Build", versionInfo.Release),
			)
		}
		peer.Version.Service = checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
		peer.Version.Chore = checker.NewChore(peer.Version.Service, config.Version.CheckInterval)

		peer.Services.Add(lifecycle.Item{
			Name: "version",
			Run:  peer.Version.Chore.Run,
		})
	}

	{ // setup payments
		pc := config.Payments

		var stripeClient stripecoinpayments.StripeClient
		var err error
		switch pc.Provider {
		default:
			stripeClient = stripecoinpayments.NewStripeMock(
				peer.ID(),
				peer.DB.StripeCoinPayments().Customers(),
				peer.DB.Console().Users(),
			)
		case "stripecoinpayments":
			stripeClient = stripecoinpayments.NewStripeClient(log, pc.StripeCoinPayments)
		}

		peer.Payments.Service, err = stripecoinpayments.NewService(
			peer.Log.Named("payments.stripe:service"),
			stripeClient,
			pc.StripeCoinPayments,
			peer.DB.StripeCoinPayments(),
			peer.DB.Console().Projects(),
			peer.DB.ProjectAccounting(),
			pc.StorageTBPrice,
			pc.EgressTBPrice,
			pc.SegmentPrice,
			pc.BonusRate)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.Stripe = stripeClient
		peer.Payments.Accounts = peer.Payments.Service.Accounts()
	}
	{ // setup admin endpoint
		var err error
		peer.Admin.Listener, err = net.Listen("tcp", config.Admin.Address)
		if err != nil {
			return nil, err
		}

		adminConfig := config.Admin
		adminConfig.AuthorizationToken = config.Console.AuthToken

		peer.Admin.Server = admin.NewServer(log.Named("admin"), peer.Admin.Listener, peer.DB, peer.Buckets.Service, peer.AccountManagementAPIKeys.Service, peer.Payments.Accounts, adminConfig)
		peer.Servers.Add(lifecycle.Item{
			Name:  "admin",
			Run:   peer.Admin.Server.Run,
			Close: peer.Admin.Server.Close,
		})
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Admin) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *Admin) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *Admin) ID() storj.NodeID { return peer.Identity.ID }
