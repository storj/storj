// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/piecelist"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repaircsv"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/repair/repairer/manual"
	"storj.io/storj/satellite/reputation"
	srevocation "storj.io/storj/satellite/revocation"
	sndebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/modular/eventkit"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	{
		config.RegisterConfig[debug.Config](ball, "debug")
		sndebug.Module(ball)
	}

	eventkit.Module(ball)

	mud.Provide[signing.Signer](ball, signing.SignerFromFullIdentity)
	consoleweb.Module(ball)
	{
		mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
		mud.Provide[rpc.Dialer](ball, rpc.NewDefaultDialer)
		mud.Provide[*tlsopts.Options](ball, tlsopts.NewOptions)
		config.RegisterConfig[tlsopts.Config](ball, "server")
	}

	{
		overlay.Module(ball)
		mud.View[DB, overlay.DB](ball, DB.OverlayCache)

		// TODO: we must keep it here as it uses consoleweb.Config from overlay package.
		mud.Provide[*overlay.Service](ball, func(log *zap.Logger, db overlay.DB, nodeEvents nodeevents.DB, placements nodeselection.PlacementDefinitions, consoleConfig consoleweb.Config, config overlay.Config) (*overlay.Service, error) {
			return overlay.NewService(log, db, nodeEvents, placements, consoleConfig.ExternalAddress, consoleConfig.SatelliteName, config)
		})
	}

	{
		// TODO: fix reversed dependency (nodeselection -> overlay).
		mud.Provide[nodeselection.PlacementDefinitions](ball, func(config nodeselection.PlacementConfig, selectionConfig overlay.NodeSelectionConfig, env nodeselection.PlacementConfigEnvironment) (nodeselection.PlacementDefinitions, error) {
			return config.Placement.Parse(selectionConfig.CreateDefaultPlacement, env)
		})
		nodeselection.Module(ball)
	}
	rangedloop.Module(ball)
	bloomfilter.Module(ball)
	metainfo.Module(ball)
	metabase.Module(ball)
	eventingconfig.Module(ball)

	{
		orders.Module(ball)
		mud.View[DB, orders.DB](ball, DB.Orders)
	}
	audit.Module(ball)

	mud.View[DB, nodeevents.DB](ball, DB.NodeEvents)

	piecelist.Module(ball)

	buckets.Module(ball)

	mud.View[DB, buckets.DB](ball, DB.Buckets)
	mud.View[DB, attribution.DB](ball, DB.Attribution)
	mud.View[DB, overlay.PeerIdentities](ball, DB.PeerIdentities)
	mud.View[DB, srevocation.DB](ball, DB.Revocation)
	mud.View[DB, console.DB](ball, DB.Console)
	mud.View[overlay.DB, bloomfilter.Overlay](ball, func(db overlay.DB) bloomfilter.Overlay {
		return db
	})

	console.Module(ball)
	mud.RegisterInterfaceImplementation[metainfo.APIKeys, console.APIKeys](ball)

	// should be defined here due to circular dependencies (accounting vs live/console config)
	mud.Provide[*accounting.Service](ball, func(log *zap.Logger, projectAccountingDB accounting.ProjectAccounting, liveAccounting accounting.Cache, metabaseDB metabase.DB, cc console.Config, config, lc live.Config) *accounting.Service {
		return accounting.NewService(log, projectAccountingDB, liveAccounting, metabaseDB, lc.BandwidthCacheTTL, cc.UsageLimits.Storage.Free, cc.UsageLimits.Bandwidth.Free, cc.UsageLimits.Segment.Free, lc.AsOfSystemInterval)
	})
	accounting.Module(ball)
	mud.View[DB, accounting.ProjectAccounting](ball, DB.ProjectAccounting)

	live.Module(ball)

	{
		mud.Provide[*server.Server](ball, server.New)
		config.RegisterConfig[server.Config](ball, "server2")
	}

	{
		mud.View[DB, entitlements.DB](ball, func(db DB) entitlements.DB {
			return db.Console().Entitlements()
		})
		mud.Provide[*entitlements.Service](ball, entitlements.NewService)
		config.RegisterConfig[entitlements.Config](ball, "entitlements")
	}

	mud.Provide[*metainfo.MigrationModeFlagExtension](ball, metainfo.NewMigrationModeFlagExtension)
	mud.Provide[eventingconfig.BucketLocationTopicIDMap](ball, func(config eventingconfig.Config) eventingconfig.BucketLocationTopicIDMap {
		return config.Buckets
	})
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server, metainfoEndpoint *metainfo.Endpoint) (*EndpointRegistration, error) {
		err := pb.DRPCRegisterMetainfo(srv.DRPC(), metainfoEndpoint)
		if err != nil {
			return nil, err
		}
		return &EndpointRegistration{}, nil
	})

	mud.View[DB, audit.ReverifyQueue](ball, DB.ReverifyQueue)
	mud.View[DB, audit.VerifyQueue](ball, DB.VerifyQueue)
	mud.View[DB, audit.WrappedContainment](ball, func(db DB) audit.WrappedContainment {
		return audit.WrappedContainment{
			Containment: db.Containment(),
		}
	})
	mud.Provide[reputation.DB](ball, func(log *zap.Logger, db DB, cfg reputation.Config) reputation.DB {
		reputationDB := db.Reputation()
		if cfg.FlushInterval > 0 {
			reputationDB = reputation.NewCachingDB(log, reputationDB, cfg)
		}
		return reputationDB
	})

	mud.View[*identity.FullIdentity, signing.Signee](ball, func(fullIdentity *identity.FullIdentity) signing.Signee {
		return signing.SigneeFromPeerIdentity(fullIdentity.PeerIdentity())
	})
	checker.Module(ball)
	repairer.Module(ball)
	manual.Module(ball)
	repaircsv.Module(ball)
	reputation.Module(ball)
	jobq.Module(ball)
	mud.RegisterInterfaceImplementation[queue.RepairQueue, *jobq.RepairJobQueue](ball)
	eventing.Module(ball)

	mud.View[metabase.Adapter, changestream.Adapter](ball, func(adapter metabase.Adapter) changestream.Adapter {
		csAdapter, ok := adapter.(changestream.Adapter)
		if !ok {
			panic("changestream service requires spanner adapter")
		}
		return csAdapter
	})

}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct{}
