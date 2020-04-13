// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRefresh(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var nodeCfg = overlay.NodeSelectionConfig{
			AuditCount:       1,
			UptimeCount:      1,
			NewNodeFraction:  0.2,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 100 * memory.MiB,
		}
		cache := overlay.NewSelectedNodesCache(ctx, zap.NewNop(),
			db.OverlayCache(), time.Hour,
			nodeCfg,
		)
		// the cache should have no nodes to start
		err := cache.Init(ctx)
		require.NoError(t, err)
		reputable, new := cache.Size(ctx)
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		addNodesToNodesTable(ctx, t, db.OverlayCache())
		// set the last refresh as 2 hrs ago and check that refresh hits when we call GetNodes
		cache.SetLastRefresh(ctx, time.Now().UTC().Add(2*-time.Hour))
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
		require.NoError(t, err)
		require.Equal(t, 2, len(nodes))
	})
}

func addNodesToNodesTable(ctx context.Context, t *testing.T, db overlay.DB) {
	const reputableNodeCount = 5
	for i := 0; i <= reputableNodeCount; i++ {
		subnet := strconv.Itoa(i) + ".1.2"
		addr := subnet + ".3:8080"
		n := overlay.NodeCheckInInfo{
			NodeID: storj.NodeID{byte(i)},
			Address: &pb.NodeAddress{
				Address:   addr,
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
			},
			LastNet:    subnet,
			LastIPPort: addr,
			IsUp:       true,
			Capacity: &pb.NodeCapacity{
				FreeDisk:      200 * memory.MiB.Int64(),
				FreeBandwidth: 1 * memory.TB.Int64(),
			},
			Version: &pb.NodeVersion{
				Version:    "v1.1.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    true,
			},
		}
		err := db.UpdateCheckIn(ctx, n, time.Now().UTC(), nodeCfg)
		require.NoError(t, err)
	}
}
