// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/coordinator"
)

// RepairCoordinator is the satellite repair coordinator process.
//
// architecture: Peer
type RepairCoordinator struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer
	Server *server.Server

	Version struct {
		Chore   *checker.Chore
		Service *checker.Service
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Metainfo *metainfo.Service
	Overlay  *overlay.Service

	Repair struct {
		Coordinator *coordinator.Endpoint
	}
}

// NewRepairCoordinator creates a new satellite repair coordinator process
func NewRepairCoordinator(log *zap.Logger, full *identity.FullIdentity, db DB, pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, bucketsDB metainfo.BucketsDB, overlayCache overlay.DB, versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel) (*RepairCoordinator, error) {
	peer := &RepairCoordinator{
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
		debugConfig.ControlTitle = "RepairCoordinator"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{
		peer.Log.Info("Version info",
			zap.Stringer("Version", versionInfo.Version.Version),
			zap.String("Commit Hash", versionInfo.CommitHash),
			zap.Stringer("Build Timestamp", versionInfo.Timestamp),
			zap.Bool("Release Build", versionInfo.Release),
		)

		peer.Version.Service = checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
		peer.Version.Chore = checker.NewChore(peer.Version.Service, config.Version.CheckInterval)

		peer.Services.Add(lifecycle.Item{
			Name: "version",
			Run:  peer.Version.Chore.Run,
		})
	}

	{ // setup listener and server
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Servers.Add(lifecycle.Item{
			Name: "server",
			Run: func(ctx context.Context) error {
				// Don't change the format of this comment, it is used to figure out the node id.
				peer.Log.Info(fmt.Sprintf("Node %s started", peer.Identity.ID))
				peer.Log.Info(fmt.Sprintf("Public server started on %s", peer.Addr()))
				peer.Log.Info(fmt.Sprintf("Private server started on %s", peer.PrivateAddr()))
				return peer.Server.Run(ctx)
			},
			Close: peer.Server.Close,
		})
	}

	{ // setup metainfo
		peer.Metainfo = metainfo.NewService(log.Named("metainfo"), pointerDB, bucketsDB)
	}

	{ // setup overlay
		var err error
		peer.Overlay, err = overlay.NewService(peer.Log.Named("overlay"), overlayCache, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Close: peer.Overlay.Close,
		})
	}

	{ // setup repair coordinator service
		var err error
		peer.Repair.Coordinator, err = coordinator.NewEndpoint(peer.Log.Named("repair-coordinator"), config.Coordinator, peer.DB.RepairQueue(), peer.DB.RepairJobList())
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err := internalpb.DRPCRegisterRepairCoordinator(peer.Server.DRPC(), peer.Repair.Coordinator); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}
	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *RepairCoordinator) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *RepairCoordinator) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *RepairCoordinator) ID() storj.NodeID { return peer.Identity.ID }

// Addr returns the public address.
func (peer *RepairCoordinator) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *RepairCoordinator) URL() storj.NodeURL {
	return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()}
}

// PrivateAddr returns the private address.
func (peer *RepairCoordinator) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
