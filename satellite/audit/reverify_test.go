// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

func TestReverifySuccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// This is a bulky test but all it's doing is:
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - the actual share is downloaded to make sure ExpectedShareHash is correct
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a success in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, pieces[0].NodeId, pieces[0].PieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Successes, 1)
		require.Equal(t, report.Successes[0], pieces[0].NodeId)
	})
}

func TestReverifyFailMissingShare(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - the actual share is downloaded to make sure ExpectedShareHash is correct
		// - delete piece from node
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a fail in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, pieces[0].NodeId, pieces[0].PieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the piece from the first node
		piece := pointer.GetRemote().GetRemotePieces()[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := getStorageNode(planet, piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], pieces[0].NodeId)
	})
}

// TestReverifyFailMissingShareHashesNotVerified tests that if piece hashes were not verified for a pointer,
// a node that fails an audit for that pointer does not get marked as failing an audit, but is removed from
// the pointer.
func TestReverifyFailMissingShareNotVerified(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, pieces[0].NodeId, pieces[0].PieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// update pointer to have PieceHashesVerified false
		err = satellite.Metainfo.Service.UnsynchronizedDelete(ctx, path)
		require.NoError(t, err)
		pointer.PieceHashesVerified = false
		err = satellite.Metainfo.Service.Put(ctx, path, pointer)
		require.NoError(t, err)

		// delete the piece from the first node
		piece := pieces[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := getStorageNode(planet, piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		// expect no failed audit
		require.Len(t, report.Fails, 0)
	})
}

func TestReverifyFailBadData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates a pending audit for a node holding a piece for that stripe
		// - makes ExpectedShareHash have random data
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a fail in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		redundancy := pointer.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			Path:              path,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		nodeID := pieces[0].NodeId
		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], nodeID)
	})
}

func TestReverifyOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audits for one node holding a piece for that stripe
		// - stop the node that has the pending audit
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as offline in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		redundancy := pointer.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
			Path:              path,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		err = stopStorageNode(ctx, planet, pieces[0].NodeId)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Offlines, 1)
		require.Equal(t, report.Offlines[0], pieces[0].NodeId)
	})
}

func TestReverifyOfflineDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audit for one node holding a piece for that stripe
		// - uses a slow transport client so that dial timeout will happen (an offline case)
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as offline in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(satellite.Identity, tlsopts.Config{}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)
		dialer.DialTimeout = 20 * time.Millisecond
		dialer.DialLatency = 200 * time.Second
		dialer.TransferRate = 1 * memory.KB

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			satellite.Log.Named("verifier"),
			satellite.Metainfo.Service,
			dialer,
			satellite.Overlay.Service,
			satellite.DB.Containment(),
			satellite.Orders.Service,
			satellite.Identity,
			minBytesPerSecond,
			5*time.Second)

		pieces := pointer.GetRemote().GetRemotePieces()

		rootPieceID := pointer.GetRemote().RootPieceId
		redundancy := pointer.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			Path:              path,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Offlines, 1)
		require.Equal(t, report.Offlines[0], pending.NodeID)
	})
}

func TestReverifyDeletedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to all nodes
		// - gets a path from the audit queue
		// - creates one pending audit for a node holding a piece for that segment
		// - deletes the file
		// - calls reverify on the deleted file
		// - expects reverification to return a segment deleted error, and expects the storage node to still be in containment
		// - uploads a new file and calls reverify on it
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}

		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		nodeID := pointer.GetRemote().GetRemotePieces()[0].NodeId
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           pointer.GetRemote().RootPieceId,
			StripeIndex:       randomIndex,
			ShareSize:         pointer.GetRemote().GetRedundancy().GetErasureShareSize(),
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			Path:              path,
		}

		containment := satellite.DB.Containment()
		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the file
		err = ul.Delete(ctx, satellite, "testbucket", "test/path1")
		require.NoError(t, err)

		// call reverify on the deleted file and expect a segment deleted error
		// but expect that the node is still in containment
		report, err := audits.Verifier.Reverify(ctx, path)
		require.True(t, audit.ErrSegmentDeleted.Has(err))
		assert.Empty(t, report)

		_, err = containment.Get(ctx, nodeID)
		require.NoError(t, err)

		// upload a new file to call reverify on
		testData2 := testrand.Bytes(8 * memory.KiB)
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err = queue.Next()
		require.NoError(t, err)

		// reverify the new path
		report, err = audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)
		assert.Empty(t, report.Fails)
		assert.Empty(t, report.Successes)
		assert.Empty(t, report.PendingAudits)

		// expect that the node was removed from containment since the segment it was contained for has been deleted
		_, err = containment.Get(ctx, nodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyModifiedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to a file on all nodes
		// - creates a pending audit for a particular node in that file
		// - re-uploads the file so that the segment is modified
		// - uploads a new file to all nodes and calls reverify on it
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}
		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		pendingPath, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, pendingPath)
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		nodeID := pointer.GetRemote().GetRemotePieces()[0].NodeId
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           pointer.GetRemote().RootPieceId,
			StripeIndex:       randomIndex,
			ShareSize:         pointer.GetRemote().GetRedundancy().GetErasureShareSize(),
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			Path:              pendingPath,
		}

		containment := satellite.DB.Containment()

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// replace the file
		err = ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		// upload another file to call reverify on
		testData2 := testrand.Bytes(8 * memory.KiB)
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		// select the encrypted path that was not used for the pending audit
		audits.Chore.Loop.TriggerWait()
		path1, err := queue.Next()
		require.NoError(t, err)
		path2, err := queue.Next()
		require.NoError(t, err)
		reverifyPath := path1
		if path1 == pendingPath {
			reverifyPath = path2
		}

		// reverify the path that was not modified
		report, err := audits.Verifier.Reverify(ctx, reverifyPath)
		require.NoError(t, err)
		assert.Empty(t, report.Fails)
		assert.Empty(t, report.Successes)
		assert.Empty(t, report.PendingAudits)

		// expect that the node was removed from containment since the segment it was contained for has been changed
		_, err = containment.Get(ctx, nodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyDifferentShare(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to two files
		// - get a random stripe to audit from file 1
		// - creates one pending audit for a node holding a piece for that stripe
		// - the actual share is downloaded to make sure ExpectedShareHash is correct
		// - delete piece for file 1 from the selected node
		// - calls reverify on some stripe from file 2
		// - expects one storage node to be marked as a fail in the audit report
		// - (if file 2 is used during reverify, the node will pass the audit and the test should fail)

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)
		// upload to three nodes so there is definitely at least one node overlap between the two files
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     3,
		}
		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path1, err := queue.Next()
		require.NoError(t, err)
		path2, err := queue.Next()
		require.NoError(t, err)
		require.NotEqual(t, path1, path2)

		pointer1, err := satellite.Metainfo.Service.Get(ctx, path1)
		require.NoError(t, err)
		pointer2, err := satellite.Metainfo.Service.Get(ctx, path2)
		require.NoError(t, err)

		// find a node that contains a piece for both files
		// save that node ID and the piece number associated with it for pointer1
		var selectedNode storj.NodeID
		var selectedPieceNum int32
		p1Nodes := make(map[storj.NodeID]int32)
		for _, piece := range pointer1.GetRemote().GetRemotePieces() {
			p1Nodes[piece.NodeId] = piece.PieceNum
		}
		for _, piece := range pointer2.GetRemote().GetRemotePieces() {
			pieceNum, ok := p1Nodes[piece.NodeId]
			if ok {
				selectedNode = piece.NodeId
				selectedPieceNum = pieceNum
				break
			}
		}
		require.NotEqual(t, selectedNode, storj.NodeID{})

		randomIndex, err := audit.GetRandomStripe(ctx, pointer1)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer1.GetRemote().GetRedundancy().GetErasureShareSize()

		rootPieceID := pointer1.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, selectedNode, selectedPieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(selectedPieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            selectedNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path1,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the piece for pointer1 from the selected node
		pieceID := pointer1.GetRemote().RootPieceId.Derive(selectedNode, selectedPieceNum)
		node := getStorageNode(planet, selectedNode)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		// reverify with path 2. Since the selected node was put in containment for path1,
		// it should be audited for path1 and fail
		report, err := audits.Verifier.Reverify(ctx, path2)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], selectedNode)
	})
}

// TestReverifyExpired1 tests the case where the segment passed into Reverify is expired
func TestReverifyExpired1(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		// set pointer's expiration date to be already expired
		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)
		oldPointerBytes, err := proto.Marshal(pointer)
		require.NoError(t, err)
		newPointer := &pb.Pointer{}
		err = proto.Unmarshal(oldPointerBytes, newPointer)
		require.NoError(t, err)
		newPointer.ExpirationDate = time.Now().UTC().Add(-1 * time.Hour)
		newPointerBytes, err := proto.Marshal(newPointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, storage.Key(path), oldPointerBytes, newPointerBytes)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.Error(t, err)
		require.True(t, audit.ErrSegmentExpired.Has(err))

		// Reverify should delete the expired segment
		pointer, err = satellite.Metainfo.Service.Get(ctx, path)
		require.Error(t, err)
		require.Nil(t, pointer)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

// TestReverifyExpired2 tests the case where the segment passed into Reverify is not expired,
// but the segment a node is contained for has expired.
func TestReverifyExpired2(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)
		// upload to three nodes so there is definitely at least one node overlap between the two files
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     3,
		}
		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path1, err := queue.Next()
		require.NoError(t, err)
		path2, err := queue.Next()
		require.NoError(t, err)
		require.NotEqual(t, path1, path2)

		pointer1, err := satellite.Metainfo.Service.Get(ctx, path1)
		require.NoError(t, err)
		pointer2, err := satellite.Metainfo.Service.Get(ctx, path2)
		require.NoError(t, err)

		// find a node that contains a piece for both files
		// save that node ID and the piece number associated with it for pointer1
		var selectedNode storj.NodeID
		var selectedPieceNum int32
		p1Nodes := make(map[storj.NodeID]int32)
		for _, piece := range pointer1.GetRemote().GetRemotePieces() {
			p1Nodes[piece.NodeId] = piece.PieceNum
		}
		for _, piece := range pointer2.GetRemote().GetRemotePieces() {
			pieceNum, ok := p1Nodes[piece.NodeId]
			if ok {
				selectedNode = piece.NodeId
				selectedPieceNum = pieceNum
				break
			}
		}
		require.NotEqual(t, selectedNode, storj.NodeID{})

		randomIndex, err := audit.GetRandomStripe(ctx, pointer1)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer1.GetRemote().GetRedundancy().GetErasureShareSize()

		rootPieceID := pointer1.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, selectedNode, selectedPieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(selectedPieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            selectedNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path1,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// update pointer1 to be expired
		oldPointerBytes, err := proto.Marshal(pointer1)
		require.NoError(t, err)
		newPointer := &pb.Pointer{}
		err = proto.Unmarshal(oldPointerBytes, newPointer)
		require.NoError(t, err)
		newPointer.ExpirationDate = time.Now().UTC().Add(-1 * time.Hour)
		newPointerBytes, err := proto.Marshal(newPointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, storage.Key(path1), oldPointerBytes, newPointerBytes)
		require.NoError(t, err)

		// reverify with path 2. Since the selected node was put in containment for path1,
		// it should be audited for path1
		// since path1 has expired, we expect no failure and we expect that the pointer has been deleted
		// and that the selected node has been removed from containment mode
		report, err := audits.Verifier.Reverify(ctx, path2)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 0)

		// Reverify should delete the expired segment
		pointer, err := satellite.Metainfo.Service.Get(ctx, path1)
		require.Error(t, err)
		require.Nil(t, pointer)

		// Reverify should remove the node from containment mode
		_, err = containment.Get(ctx, pending.NodeID)
		require.Error(t, err)
	})
}

// TestReverifySlowDownload checks that a node that times out while sending data to the
// audit service gets put into containment mode
func TestReverifySlowDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
				config.Audit.MinBytesPerSecond = 100 * memory.KiB
				config.Audit.MinDownloadTimeout = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  2,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		slowPiece := pointer.Remote.RemotePieces[0]
		slowNode := slowPiece.NodeId

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, slowNode, slowPiece.PieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            slowNode,
			PieceID:           pointer.Remote.RootPieceId,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			if node.ID() == slowNode {
				slowNodeDB := node.DB.(*testblobs.SlowDB)
				// make downloads on storage node slower than the timeout on the satellite for downloading shares
				delay := 1 * time.Second
				slowNodeDB.SetLatency(delay)
				break
			}
		}

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 1)
		require.Len(t, report.Unknown, 0)
		require.Equal(t, report.PendingAudits[0].NodeID, slowNode)
	})
}

// TestReverifyUnknownError checks that a node that returns an unknown error during an audit does not get marked as successful, failed, or contained.
func TestReverifyUnknownError(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		queue := audits.Queue

		audits.Worker.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  2,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		badPiece := pointer.Remote.RemotePieces[0]
		badNode := badPiece.NodeId

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := pointer.GetRemote().GetRemotePieces()
		rootPieceID := pointer.GetRemote().RootPieceId
		limit, privateKey, err := orders.CreateAuditOrderLimit(ctx, bucketID, badNode, badPiece.PieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, randomIndex, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            badNode,
			PieceID:           pointer.Remote.RootPieceId,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			Path:              path,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			if node.ID() == badNode {
				badNodeDB := node.DB.(*testblobs.BadDB)
				// return an error when the satellite requests a share
				badNodeDB.SetError(errs.New("unknown error"))
				break
			}
		}

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Unknown, 1)
		require.Equal(t, report.Unknown[0], badNode)
	})
}
