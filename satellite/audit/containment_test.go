// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink"
)

func TestContainIncrementAndGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()
		cache := planet.Satellites[0].DB.OverlayCache()

		input := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
		}

		err := containment.IncrementPending(ctx, input)
		require.NoError(t, err)

		output, err := containment.Get(ctx, input.NodeID)
		require.NoError(t, err)

		assert.Equal(t, input, output)

		// check contained flag set to true
		node, err := cache.Get(ctx, input.NodeID)
		require.NoError(t, err)
		assert.True(t, node.Contained)

		nodeID1 := planet.StorageNodes[1].ID()
		_, err = containment.Get(ctx, nodeID1)
		require.Error(t, err, audit.ErrContainedNotFound.New("%v", nodeID1))
		assert.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestContainIncrementPendingEntryExists(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
		}

		err := containment.IncrementPending(ctx, info1)
		require.NoError(t, err)

		info2 := &audit.PendingAudit{
			NodeID:            info1.NodeID,
			StripeIndex:       1,
			ShareSize:         1,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
		}

		// expect failure when an entry with the same nodeID but different expected share data already exists
		err = containment.IncrementPending(ctx, info2)
		assert.True(t, audit.ErrAlreadyExists.Has(err))

		// expect reverify count for an entry to be 0 after first IncrementPending call
		pending, err := containment.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.EqualValues(t, 0, pending.ReverifyCount)

		// expect reverify count to be 1 after second IncrementPending call
		err = containment.IncrementPending(ctx, info1)
		require.NoError(t, err)
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
		cache := planet.Satellites[0].DB.OverlayCache()

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
		}

		err := containment.IncrementPending(ctx, info1)
		require.NoError(t, err)

		// delete the node from containment db
		isDeleted, err := containment.Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.True(t, isDeleted)

		// check contained flag set to false
		node, err := cache.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.False(t, node.Contained)

		// get pending audit that doesn't exist
		_, err = containment.Get(ctx, info1.NodeID)
		assert.Error(t, err, audit.ErrContainedNotFound.New("%v", info1.NodeID))
		assert.True(t, audit.ErrContainedNotFound.Has(err))

		// delete pending audit that doesn't exist
		isDeleted, err = containment.Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.False(t, isDeleted)
	})
}

func TestContainUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		containment := planet.Satellites[0].DB.Containment()
		cache := planet.Satellites[0].DB.OverlayCache()

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
		}

		err := containment.IncrementPending(ctx, info1)
		require.NoError(t, err)

		// update node stats
		_, err = cache.BatchUpdateStats(ctx, []*overlay.UpdateRequest{{NodeID: info1.NodeID}}, 100)
		require.NoError(t, err)

		// check contained flag set to false
		node, err := cache.Get(ctx, info1.NodeID)
		require.NoError(t, err)
		assert.False(t, node.Contained)

		// get pending audit that doesn't exist
		_, err = containment.Get(ctx, info1.NodeID)
		assert.Error(t, err, audit.ErrContainedNotFound.New("%v", info1.NodeID))
		assert.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestContainGetDeleted ensures that we delete a pending audit for a node if the
// segment associated with the pending audit is deleted
func TestContainGetDeleted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		containment := satellite.DB.Containment()

		// stop audit worker
		satellite.Audit.Worker.Loop.Pause()

		// upload file
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  1,
			SuccessThreshold: 2,
			MaxThreshold:     2,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get path from audit chore
		satellite.Audit.Chore.Loop.TriggerWait()
		path, err := satellite.Audit.Queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		remote := pointer.GetRemote()
		pieces := remote.GetRemotePieces()

		piece := pieces[0]

		// create pending audit
		pending := &audit.PendingAudit{
			NodeID:            piece.NodeId,
			PieceID:           remote.RootPieceId,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			Path:              path,
		}
		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// expect that node is in containment mode
		_, err = containment.Get(ctx, piece.NodeId)
		require.NoError(t, err)

		// delete file
		err = ul.Delete(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		// get from containment and expect not found error
		_, err = containment.Get(ctx, piece.NodeId)
		require.Error(t, err)
		assert.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestContainGetNotDeleted ensures that a node in containment mode does not get deleted
// if a segment with that node is deleted that is not the segment the node was contained for
func TestContainGetNotDeleted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		containment := satellite.DB.Containment()

		// stop audit worker
		planet.Satellites[0].Audit.Worker.Loop.Pause()

		// upload one file
		testData := testrand.Bytes(8 * memory.KiB)
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  1,
			SuccessThreshold: 2,
			MaxThreshold:     2,
		}
		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testData)
		require.NoError(t, err)

		// get encrypted path from audit chore
		satellite.Audit.Chore.Loop.TriggerWait()
		path, err := satellite.Audit.Queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		remote := pointer.GetRemote()
		pieces := remote.GetRemotePieces()

		piece := pieces[0]

		// create pending audit
		pending := &audit.PendingAudit{
			NodeID:            piece.NodeId,
			PieceID:           remote.RootPieceId,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			Path:              path,
		}
		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// expect that node is in containment mode
		_, err = containment.Get(ctx, piece.NodeId)
		require.NoError(t, err)

		// upload and delete a different file
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testData)
		require.NoError(t, err)

		err = ul.Delete(ctx, satellite, "testbucket", "test/path2")
		require.NoError(t, err)

		// get from containment and expect that node is still there
		_, err = containment.Get(ctx, piece.NodeId)
		require.NoError(t, err)
	})
}

// TestContainIncrementDelted ensures that if we increment a pending audit for a node
// and that segment no longer exists, that the pending audit is deleted.
// TODO what if the segment the node is incremented for exists, but the original does not?
// TODO what if neither of the segments exists?
func TestContainIncrementDeleted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// stop audit
		planet.Satellites[0].Audit.Worker.Loop.Pause()

		// upload 2 files
		// add to containment with specific path and piece ID (file 1)
		// increment containment for node with file 2
		// get from containment mode
		// expect to see file 1
		// delete file 1
		// increment containment for node with file 2
		// get from containment mode
		// expect to see file 2, not file 1
	})
}
