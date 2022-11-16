// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"
	"runtime/pprof"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
)

// RangedLoop is the satellite ranged loop process.
//
// architecture: Peer
type RangedLoop struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	RangedLoop struct {
		Service *rangedloop.Service
	}
}

// NewRangedLoop creates a new satellite ranged loop process.
func NewRangedLoop(log *zap.Logger, full *identity.FullIdentity, db DB, metabaseDB *metabase.DB, config *Config, atomicLogLevel *zap.AtomicLevel) (*RangedLoop, error) {
	peer := &RangedLoop{
		Log:      log,
		Identity: full, // TODO: figure out if we need Identity here
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
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "RangedLoop"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{ // setup ranged loop
		// TODO: replace with real segment provider
		segments := &rangedlooptest.MockProvider{}
		peer.RangedLoop.Service = rangedloop.NewService(log.Named("rangedloop"), config.RangedLoop, segments, nil)

		peer.Services.Add(lifecycle.Item{
			Name: "rangeloop",
			Run:  peer.RangedLoop.Service.Run,
		})
	}

	return peer, nil
}

// Run runs satellite ranged loop until it's either closed or it errors.
func (peer *RangedLoop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "rangedloop"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *RangedLoop) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *RangedLoop) ID() storj.NodeID { return peer.Identity.ID }
