// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/private/version"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
)

// Repairer is the repairer process.
//
// architecture: Peer
type Repairer struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity

	Dialer  rpc.Dialer
	Version *version_checker.Service

	Metainfo        *metainfo.Service
	Overlay         *overlay.Service
	Orders          *orders.Service
	SegmentRepairer *repairer.SegmentRepairer
	Repairer        *repairer.Service
}

// NewRepairer creates a new repairer peer.
func NewRepairer(log *zap.Logger, full *identity.FullIdentity, pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, repairQueue queue.RepairQueue,
	bucketsDB metainfo.BucketsDB, overlayCache overlay.DB, ordersDB orders.DB, versionInfo version.Info, config *Config) (*Repairer, error) {
	peer := &Repairer{
		Log:      log,
		Identity: full,
	}

	{
		if !versionInfo.IsZero() {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version_checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
	}

	{ // setup dialer
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)
	}

	{ // setup metainfo
		log.Debug("Setting up metainfo")
		peer.Metainfo = metainfo.NewService(log.Named("metainfo"), pointerDB, bucketsDB)
	}

	{ // setup overlay
		log.Debug("Setting up overlay")
		peer.Overlay = overlay.NewService(log.Named("overlay"), overlayCache, config.Overlay)
	}

	{ // setup orders
		log.Debug("Setting up orders")
		peer.Orders = orders.NewService(
			log.Named("orders"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay,
			ordersDB,
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.Contact.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
		)
	}

	{ // setup repairer
		log.Debug("Setting up repairer")
		peer.SegmentRepairer = repairer.NewSegmentRepairer(
			log.Named("segment repairer"),
			peer.Metainfo,
			peer.Orders,
			peer.Overlay,
			peer.Dialer,
			config.Repairer.Timeout,
			config.Repairer.MaxExcessRateOptimalThreshold,
			config.Checker.RepairOverride,
			config.Repairer.DownloadTimeout,
			signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity()),
		)
		peer.Repairer = repairer.NewService(log.Named("repairer"), repairQueue, &config.Repairer, peer.SegmentRepairer)
	}

	return peer, nil
}

// Run runs the repair process until it's either closed or it errors.
func (peer *Repairer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		peer.Log.Info("Repairer started")
		return errs2.IgnoreCanceled(peer.Repairer.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Repairer) Close() error {
	var errlist errs.Group

	// close services in reverse initialization order

	if peer.Overlay != nil {
		errlist.Add(peer.Overlay.Close())
	}
	if peer.Repairer != nil {
		errlist.Add(peer.Repairer.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Repairer) ID() storj.NodeID { return peer.Identity.ID }
