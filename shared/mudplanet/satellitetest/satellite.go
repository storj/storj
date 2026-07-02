// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitetest

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	trustmud "storj.io/storj/satellite/trust/mud"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/modular/opentelemetry"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/sntest"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/piecestore"
)

// Satellite is a configuration. For db support, Wrap it with WithDB.
var Satellite = mudplanet.Customization{
	Modules: mudplanet.Modules{
		dbModule,
		opentelemetry.Module,
		satellite.Module,
		trustmud.Module,
	},
	PreInit: []any{
		func(options *live.Config) {
			options.StorageBackend = "noop://"
		},
		func(options *orders.Config) error {
			key, err := orders.NewEncryptionKeys(orders.EncryptionKey{
				ID:  orders.EncryptionKeyID{1},
				Key: storj.Key{1},
			})
			if err != nil {
				return err
			}
			options.EncryptionKeys = *key
			return nil
		},
	},
}

// WithoutDB is a configuration for running satellite without database support, but the SatelliteDatabases is required by the dependency graph.
func WithoutDB(ball *mud.Ball) {
	mud.Supply[satellitedbtest.SatelliteDatabases](ball, satellitedbtest.SatelliteDatabases{})
}

// WithDB is a configuration for running satellite with database support.
func WithDB(components ...mudplanet.Component) mudplanet.Config {
	return mudplanet.Config{
		Components: components,
		RunWrapper: runWithDatabases,
	}
}

// WithStorageNodes returns a mudplanet.Config with a satellite and n storage nodes ready
// for upload tests. Pass it directly to mudplanet.Run.
//
// The RS scheme is derived from n using testplanet's ratios (min=n/5, repair=2n/5,
// success=3n/5, total=4n/5, each at least 1). Optional reconfigure functions can override
// individual metainfo.Config fields.
//
// Example:
//
//	mudplanet.Run(t, satellitetest.WithStorageNodes(t, 4),
//	    func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
//	        // satellite and nodes are ready; upload / download here
//	    }
//	)
//
//	// With RS override:
//	mudplanet.Run(t, satellitetest.WithStorageNodes(t, 4,
//	    func(cfg *metainfo.Config) { cfg.RS.Total = 3 }),
//	    func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
//	        // satellite and nodes are ready; upload / download here
//	    }
//	)
func WithStorageNodes(t *testing.T, n int, reconfigure ...func(*metainfo.Config)) mudplanet.Config {
	t.Helper()

	snNodes := make([]mudplanet.Component, n)
	snNames := make([]string, n)
	for i := range n {
		name := fmt.Sprintf("storagenode%d", i)
		snNames[i] = name
		snNodes[i] = mudplanet.NewComponent(name, sntest.StoragenodeForSatellite(0),
			mudplanet.WithRunning[*storagenode.EndpointRegistration](),
			mudplanet.WithConfig(func(cfg *monitor.Config) {
				cfg.MinimumDiskSpace = 100 * memory.MB
			}),
			mudplanet.WithConfig(func(cfg *piecestore.OldConfig) {
				cfg.AllocatedDiskSpace = 100 * memory.MB
			}),
		)
	}

	components := append([]mudplanet.Component{
		mudplanet.NewComponent("satellite", Satellite,
			mudplanet.WithRunning[*satellite.EndpointRegistration](),
			mudplanet.WithConfig(func(cfg *metainfo.Config) {
				cfg.RS.Min = atLeastOne(n * 1 / 5)
				cfg.RS.Repair = atLeastOne(n * 2 / 5)
				cfg.RS.Success = atLeastOne(n * 3 / 5)
				cfg.RS.Total = atLeastOne(n * 4 / 5)
				for _, fn := range reconfigure {
					fn(cfg)
				}
			}),
		),
	}, snNodes...)

	cfg := WithDB(components...)
	cfg.Setup = func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		satDB := mudplanet.FindFirst[satellite.DB](t, run, "satellite", 0)
		satOverlay := mudplanet.FindFirst[*overlay.Service](t, run, "satellite", 0)
		uploadCache := mudplanet.FindFirst[*overlay.UploadSelectionCache](t, run, "satellite", 0)
		eg := new(errgroup.Group)
		for i, name := range snNames {
			// FindFirst uses require.* internally, so call it on the test goroutine
			// before handing off to the errgroup worker.
			snServer := mudplanet.FindFirst[*server.Server](t, run, name, i+1)
			snID := mudplanet.FindFirst[*identity.FullIdentity](t, run, name, i+1)
			eg.Go(func() error {
				return registerWithSatellite(ctx, satDB, satOverlay, snServer, snID)
			})
		}
		require.NoError(t, eg.Wait())
		require.NoError(t, uploadCache.Refresh(ctx))
	}
	return cfg
}

func dbModule(ball *mud.Ball) {
	mud.Provide[satellite.DB](ball, func(ctx context.Context, log *zap.Logger, database satellitedbtest.SatelliteDatabases) (satellite.DB, error) {
		db, err := satellitedbtest.CreateMasterDB(ctx, log.Named("db"), "satellite", "S", 1, database.MasterDB, satellitedb.Options{
			ApplicationName: "mudplanet",
		})
		if err != nil {
			return nil, err
		}
		err = satellitedb.MigrateSatelliteDB(ctx, log, db, "snapshot,testdata")
		return db, err
	})
	mud.Provide[*metabase.DB](ball, func(ctx context.Context, log *zap.Logger, database satellitedbtest.SatelliteDatabases) (*metabase.DB, error) {
		db, err := satellitedbtest.CreateMetabaseDB(ctx, log.Named("metabase"), "metabase", "M", 1, database.MetabaseDB, metabase.Config{
			ApplicationName:  "mudplanet",
			MaxNumberOfParts: 100,
		})
		if err != nil {
			return nil, err
		}
		err = db.TestMigrateToLatest(ctx)
		return db, err
	})
}

func runWithDatabases(t *testing.T, fn func(t *testing.T, module func(*mud.Ball))) {
	databases := satellitedbtest.Databases(t)
	if len(databases) == 0 {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + dbtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + dbtest.DefaultCockroach + "\n" +
			"-spanner-test-db=" + dbtest.DefaultSpanner)
	}

	for _, satelliteDB := range databases {
		// TODO(tidb): skip TiDB tests for mudplanet for the time being.
		if satelliteDB.Name == "TiDB" {
			continue
		}
		t.Run(satelliteDB.Name, func(t *testing.T) {
			fn(t, func(ball *mud.Ball) {
				mud.Supply[satellitedbtest.SatelliteDatabases](ball, satelliteDB)
			})
		})
	}
}

// registerWithSatellite checks a storage node into the satellite overlay so it can be
// selected for uploads. LastNet is set to ip:port (matching MaskOffLastNet with
// DistinctIP=false) so the clumping invariant treats each node as a distinct network.
func registerWithSatellite(ctx context.Context, satDB satellite.DB, satOverlay *overlay.Service, snServer *server.Server, snID *identity.FullIdentity) error {
	if err := satDB.PeerIdentities().Set(ctx, snID.ID, snID.PeerIdentity()); err != nil {
		return err
	}

	addr := snServer.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	_, err = satOverlay.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
		NodeID:     snID.ID,
		Address:    &pb.NodeAddress{Address: addr},
		LastIPPort: addr,
		LastNet:    net.JoinHostPort(host, port),
		IsUp:       true,
		IsTrusted:  true,
		Version:    &pb.NodeVersion{Version: "v0.0.0"},
		Capacity:   &pb.NodeCapacity{FreeDisk: (200 * memory.MB).Int64()},
	}, time.Now())
	return err
}

func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
