// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/audit"
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
		dialer.RequestTimeout = 0
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
		// - uploads random data
		// - gets a path from the audit queue
		// - creates one pending audit for a node holding a piece for that segment
		// - deletes the file
		// - calls reverify on that same stripe
		// - expects reverification to pass successufully and the storage node to be not in containment mode

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
		err = ul.Delete(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.True(t, audit.ErrSegmentDeleted.Has(err))
		assert.Empty(t, report)

		_, err = containment.Get(ctx, nodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyModifiedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - re-uploads the file
		// - calls reverify on that same stripe
		// - expects reverification to pass successufully and the storage node to be not in containment mode

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

		// replace the file
		err = ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, path)
		require.NoError(t, err)
		assert.Empty(t, report)

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
