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
	"storj.io/common/peertls/extensions"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/metainfo"
)

// Admin is the satellite core process that runs chores
//
// architecture: Peer
type Admin struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

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

	Admin struct {
		Listener net.Listener
		Server   *admin.Server
	}
}

// NewAdmin creates a new satellite admin peer.
func NewAdmin(log *zap.Logger, full *identity.FullIdentity, db DB,
	pointerDB metainfo.PointerDB,
	revocationDB extensions.RevocationDB,
	versionInfo version.Info, config *Config) (*Admin, error) {
	peer := &Admin{
		Log:      log,
		Identity: full,
		DB:       db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup debug
		var err error
		if config.Debug.Address != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Address)
			if err != nil {
				withoutStack := errors.New(err.Error())
				peer.Log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
				err = nil
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "Admin"
		peer.Debug.Server = debug.NewServer(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig)
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

	{ // setup debug
		var err error
		peer.Admin.Listener, err = net.Listen("tcp", config.Admin.Address)
		if err != nil {
			return nil, err
		}

		adminConfig := config.Admin
		adminConfig.AuthorizationToken = config.Console.AuthToken

		peer.Admin.Server = admin.NewServer(log.Named("admin"), peer.Admin.Listener, peer.DB, adminConfig)
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
