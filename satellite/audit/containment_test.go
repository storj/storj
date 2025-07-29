// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/storagenode"
)

func TestContainInsertAndGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()

		input := &audit.PieceLocator{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPositionFromEncoded(uint64(rand.Int63())),
			NodeID:   planet.StorageNodes[0].ID(),
			PieceNum: 0,
		}

		err := containment.Insert(ctx, input)
		require.NoError(t, err)

		output, err := containment.Get(ctx, input.NodeID)
		require.NoError(t, err)

		assert.Equal(t, *input, output.Locator)
		assert.EqualValues(t, 0, output.ReverifyCount)

		nodeID1 := planet.StorageNodes[1].ID()
		_, err = containment.Get(ctx, nodeID1)
		require.Error(t, err, audit.ErrContainedNotFound.New("%v", nodeID1))
		assert.Truef(t, audit.ErrContainedNotFound.Has(err), "expected ErrContainedNotFound but got %+v", err)
	})
}

func TestContainIncrementPendingEntryExists(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()

		info1 := &audit.PieceLocator{
			NodeID: planet.StorageNodes[0].ID(),
		}

		err := containment.Insert(ctx, info1)
		require.NoError(t, err)

		// expect reverify count for an entry to be 0 after first IncrementPending call
		pending, err := containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, 0, pending.ReverifyCount)

		// expect reverify count to be 0 still after second IncrementPending call
		err = containment.Insert(ctx, info1)
		require.NoError(t, err)
		pending, err = containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, 0, pending.ReverifyCount)

		// after the job is selected for work, its ReverifyCount should be increased to 1
		job, err := planet.Satellites[0].DB.ReverifyQueue().GetNextJob(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, pending.Locator, job.Locator)
		assert.EqualValues(t, 1, job.ReverifyCount)

		pending, err = containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, pending.ReverifyCount)
	})
}

func TestContainDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()

		// add two reverification jobs for the same node
		info1 := &audit.PieceLocator{
			NodeID:   planet.StorageNodes[0].ID(),
			StreamID: testrand.UUID(),
		}
		info2 := &audit.PieceLocator{
			NodeID:   planet.StorageNodes[0].ID(),
			StreamID: testrand.UUID(),
		}

		err := containment.Insert(ctx, info1)
		require.NoError(t, err)
		err = containment.Insert(ctx, info2)
		require.NoError(t, err)

		// 'get' will choose one of them (we don't really care which)
		got, err := containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		if got.Locator != *info1 {
			require.Equal(t, *info2, got.Locator)
		}
		require.EqualValues(t, 0, got.ReverifyCount)

		// delete one of the pending reverifications
		wasDeleted, stillInContainment, err := containment.Delete(ctx, info2)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		require.True(t, stillInContainment)

		// 'get' now is sure to select info1
		got, err = containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.Equal(t, *info1, got.Locator)
		require.EqualValues(t, 0, got.ReverifyCount)

		// delete the other pending reverification
		wasDeleted, stillInContainment, err = containment.Delete(ctx, info1)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		require.False(t, stillInContainment)

		// try to get a pending reverification that isn't in the queue
		_, err = containment.Get(ctx, info1.NodeID)
		require.Error(t, err, audit.ErrContainedNotFound.New("%v", info1.NodeID))
		require.True(t, audit.ErrContainedNotFound.Has(err))

		// and try to delete that pending reverification that isn't in the queue
		wasDeleted, _, err = containment.Delete(ctx, info1)
		require.NoError(t, err)
		assert.False(t, wasDeleted)
	})
}

// UpdateStats used to remove nodes from containment. It doesn't anymore.
// This is a sanity check.
func TestContainUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Audit.ContainmentSyncChoreInterval = -1 // disable containment sync chore
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Contact.Interval = -1 // disable contact chore
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()
		cache := planet.Satellites[0].DB.OverlayCache()

		info1 := &audit.PieceLocator{
			NodeID: planet.StorageNodes[0].ID(),
		}

		err := containment.Insert(ctx, info1)
		require.NoError(t, err)

		// update node stats
		err = planet.Satellites[0].Reputation.Service.ApplyAudit(ctx, info1.NodeID, overlay.ReputationStatus{}, reputation.AuditSuccess)
		require.NoError(t, err)

		// check contained flag set to false
		node, err := cache.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.False(t, node.Contained)

		// get pending audit
		_, err = containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
	})
}
