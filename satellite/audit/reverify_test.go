// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
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
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/storagenode"
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

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment.Redundancy.ShareSize
		pieces := segment.Pieces
		rootPieceID := segment.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, pieces[0].StorageNode, pieces[0].Number, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(pieces[0].Number))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].StorageNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Successes, 1)
		require.Equal(t, report.Successes[0], pieces[0].StorageNode)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
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
		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment.Redundancy.ShareSize
		pieces := segment.Pieces
		rootPieceID := segment.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, pieces[0].StorageNode, pieces[0].Number, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(pieces[0].Number))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].StorageNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the piece from the first node
		piece := pieces[0]
		pieceID := rootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], pieces[0].StorageNode)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
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

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		pieces := segment.Pieces
		rootPieceID := segment.RootPieceID
		redundancy := segment.Redundancy

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].StorageNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		nodeID := pieces[0].StorageNode
		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], nodeID)

		// make sure that pending audit is removed
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
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

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		pieces := segment.Pieces
		rootPieceID := segment.RootPieceID
		redundancy := segment.Redundancy

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].StorageNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(pieces[0].StorageNode))
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Offlines, 1)
		require.Equal(t, report.Offlines[0], pieces[0].StorageNode)

		// make sure that pending audit is not removed
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, pending.NodeID)
		require.NoError(t, err)
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

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(satellite.Identity, tlsopts.Config{}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)
		dialer.DialTimeout = 20 * time.Millisecond
		dialer.DialLatency = 200 * time.Second

		connector := rpc.NewHybridConnector()
		connector.SetTransferRate(1 * memory.KB)
		dialer.Connector = connector

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			satellite.Log.Named("verifier"),
			satellite.Metabase.DB,
			dialer,
			satellite.Overlay.Service,
			satellite.DB.Containment(),
			satellite.Orders.Service,
			satellite.Identity,
			minBytesPerSecond,
			5*time.Second)

		pieces := segment.Pieces
		rootPieceID := segment.RootPieceID
		redundancy := segment.Redundancy

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].StorageNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = satellite.DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Offlines, 1)
		require.Equal(t, report.Offlines[0], pending.NodeID)

		// make sure that pending audit is not removed
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, pending.NodeID)
		require.NoError(t, err)
	})
}

func TestReverifyDeletedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to all nodes
		// - gets a segment from the audit queue
		// - creates one pending audit for a node holding a piece for that segment
		// - deletes the file
		// - calls reverify on the deleted file
		// - expects reverification to return a segment deleted error, and expects the storage node to still be in containment
		// - uploads a new file and calls reverify on it
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		nodeID := segment.Pieces[0].StorageNode
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           segment.RootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         segment.Redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		containment := satellite.DB.Containment()
		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the file
		err = ul.DeleteObject(ctx, satellite, "testbucket", "test/path1")
		require.NoError(t, err)

		// call reverify on the deleted file and expect no error
		// but expect that the node is still in containment
		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)
		assert.Empty(t, report)

		_, err = containment.Get(ctx, nodeID)
		require.NoError(t, err)

		// upload a new file to call reverify on
		testData2 := testrand.Bytes(8 * memory.KiB)
		err = ul.Upload(ctx, satellite, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue = audits.Queues.Fetch()
		queueSegment, err = queue.Next()
		require.NoError(t, err)

		// reverify the new segment
		report, err = audits.Verifier.Reverify(ctx, queueSegment)
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to a file on all nodes
		// - creates a pending audit for a particular node in that file
		// - removes a piece from the file so that the segment is modified
		// - uploads a new file to all nodes and calls reverify on it
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		nodeID := segment.Pieces[0].StorageNode
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           segment.RootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         segment.Redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		containment := satellite.DB.Containment()

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// remove a piece from the file (a piece that the contained node isn't holding)
		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			err = satellite.Metabase.DB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID:      queueSegment.StreamID,
				Position:      queueSegment.Position,
				OldPieces:     segment.Pieces,
				NewPieces:     append([]metabase.Piece{segment.Pieces[0]}, segment.Pieces[2:]...),
				NewRedundancy: segment.Redundancy,
			})
			require.NoError(t, err)
		}

		// upload another file to call reverify on
		testData2 := testrand.Bytes(8 * memory.KiB)
		err = ul.Upload(ctx, satellite, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		// select the segment that was not used for the pending audit
		audits.Chore.Loop.TriggerWait()
		queue = audits.Queues.Fetch()
		queueSegment1, err := queue.Next()
		require.NoError(t, err)
		queueSegment2, err := queue.Next()
		require.NoError(t, err)
		reverifySegment := queueSegment1
		if queueSegment1 == queueSegment {
			reverifySegment = queueSegment2
		}

		// reverify the segment that was not modified
		report, err := audits.Verifier.Reverify(ctx, reverifySegment)
		require.NoError(t, err)
		assert.Empty(t, report.Fails)
		assert.Empty(t, report.Successes)
		assert.Empty(t, report.PendingAudits)

		// expect that the node was removed from containment since the segment it was contained for has been changed
		_, err = containment.Get(ctx, nodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyReplacedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// - uploads random data to a file on all nodes
		// - creates a pending audit for a particular node in that file
		// - re-uploads the file so that the segment is modified
		// - uploads a new file to all nodes and calls reverify on it
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		nodeID := segment.Pieces[0].StorageNode
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           segment.RootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         segment.Redundancy.ShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		containment := satellite.DB.Containment()

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// replace the file
		err = ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		// upload another file to call reverify on
		testData2 := testrand.Bytes(8 * memory.KiB)
		err = ul.Upload(ctx, satellite, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		// select the segment that was not used for the pending audit
		audits.Chore.Loop.TriggerWait()
		queue = audits.Queues.Fetch()
		queueSegment1, err := queue.Next()
		require.NoError(t, err)
		queueSegment2, err := queue.Next()
		require.NoError(t, err)
		reverifySegment := queueSegment1
		if queueSegment1 == queueSegment {
			reverifySegment = queueSegment2
		}

		// reverify the segment that was not modified
		report, err := audits.Verifier.Reverify(ctx, reverifySegment)
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
		Reconfigure: testplanet.Reconfigure{
			// upload to three nodes so there is definitely at least one node overlap between the two files
			Satellite: testplanet.ReconfigureRS(1, 2, 3, 3),
		},
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

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = ul.Upload(ctx, satellite, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment1, err := queue.Next()
		require.NoError(t, err)
		queueSegment2, err := queue.Next()
		require.NoError(t, err)
		require.NotEqual(t, queueSegment1, queueSegment2)

		segment1, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment1.StreamID,
			Position: queueSegment1.Position,
		})
		require.NoError(t, err)

		segment2, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment2.StreamID,
			Position: queueSegment2.Position,
		})
		require.NoError(t, err)

		// find a node that contains a piece for both files
		// save that node ID and the piece number associated with it for segment1
		var selectedNode storj.NodeID
		var selectedPieceNum uint16
		p1Nodes := make(map[storj.NodeID]uint16)
		for _, piece := range segment1.Pieces {
			p1Nodes[piece.StorageNode] = piece.Number
		}
		for _, piece := range segment2.Pieces {
			pieceNum, ok := p1Nodes[piece.StorageNode]
			if ok {
				selectedNode = piece.StorageNode
				selectedPieceNum = pieceNum
				break
			}
		}
		require.NotEqual(t, selectedNode, storj.NodeID{})

		randomIndex, err := audit.GetRandomStripe(ctx, segment1)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment1.Redundancy.ShareSize
		rootPieceID := segment1.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, selectedNode, selectedPieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(selectedPieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            selectedNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment1.StreamID,
			Position:          queueSegment1.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the piece for segment1 from the selected node
		pieceID := segment1.RootPieceID.Derive(selectedNode, int32(selectedPieceNum))
		node := planet.FindNode(selectedNode)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		// reverify with segment2. Since the selected node was put in containment for segment1,
		// it should be audited for segment1 and fail
		report, err := audits.Verifier.Reverify(ctx, queueSegment2)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], selectedNode)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestReverifyExpired1 tests the case where the segment passed into Reverify is expired.
func TestReverifyExpired1(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.UploadWithExpiration(ctx, satellite, "testbucket", "test/path", testData, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		// move time into the future so the segment is expired
		audits.Verifier.SetNow(func() time.Time {
			return time.Now().Add(2 * time.Hour)
		})

		// Reverify should not return an error
		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

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
		Reconfigure: testplanet.Reconfigure{
			// upload to three nodes so there is definitely at least one node overlap between the two files
			Satellite: testplanet.ReconfigureRS(1, 2, 3, 3),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)

		err := ul.UploadWithExpiration(ctx, satellite, "testbucket", "test/path1", testData1, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		err = ul.Upload(ctx, satellite, "testbucket", "test/path2", testData2)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment1, err := queue.Next()
		require.NoError(t, err)
		queueSegment2, err := queue.Next()
		require.NoError(t, err)
		require.NotEqual(t, queueSegment1, queueSegment2)

		// make sure queueSegment1 is the one with the expiration date
		if queueSegment1.ExpiresAt == nil {
			queueSegment1, queueSegment2 = queueSegment2, queueSegment1
		}

		segment1, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment1.StreamID,
			Position: queueSegment1.Position,
		})
		require.NoError(t, err)

		segment2, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment2.StreamID,
			Position: queueSegment2.Position,
		})
		require.NoError(t, err)

		// find a node that contains a piece for both files
		// save that node ID and the piece number associated with it for segment1
		var selectedNode storj.NodeID
		var selectedPieceNum uint16
		p1Nodes := make(map[storj.NodeID]uint16)
		for _, piece := range segment1.Pieces {
			p1Nodes[piece.StorageNode] = piece.Number
		}
		for _, piece := range segment2.Pieces {
			pieceNum, ok := p1Nodes[piece.StorageNode]
			if ok {
				selectedNode = piece.StorageNode
				selectedPieceNum = pieceNum
				break
			}
		}
		require.NotEqual(t, selectedNode, storj.NodeID{})

		randomIndex, err := audit.GetRandomStripe(ctx, segment1)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment1.Redundancy.ShareSize
		rootPieceID := segment1.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, selectedNode, selectedPieceNum, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(selectedPieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            selectedNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment1.StreamID,
			Position:          queueSegment1.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// move time into the future so segment1 is expired
		audits.Verifier.SetNow(func() time.Time {
			return time.Now().Add(2 * time.Hour)
		})

		// reverify with segment2. Since the selected node was put in containment for segment1,
		// it should be audited for segment1
		// since segment1 has expired, we expect no failure and we expect that the segment has been deleted
		// and that the selected node has been removed from containment mode
		report, err := audits.Verifier.Reverify(ctx, queueSegment2)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 0)

		// Reverify should remove the node from containment mode
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestReverifySlowDownload checks that a node that times out while sending data to the
// audit service gets put into containment mode.
func TestReverifySlowDownload(t *testing.T) {
	const auditTimeout = time.Second
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = auditTimeout
				},
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowPiece := segment.Pieces[0]
		slowNode := slowPiece.StorageNode

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment.Redundancy.ShareSize
		rootPieceID := segment.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, slowNode, slowPiece.Number, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(slowPiece.Number))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            slowNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		node := planet.FindNode(slowNode)
		slowNodeDB := node.DB.(*testblobs.SlowDB)
		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 10 * auditTimeout
		slowNodeDB.SetLatency(delay)

		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 1)
		assert.Len(t, report.Unknown, 0)
		assert.Equal(t, report.PendingAudits[0].NodeID, slowNode)

		_, err = audits.Reporter.RecordAudits(ctx, report)
		assert.NoError(t, err)

		_, err = containment.Get(ctx, slowNode)
		assert.NoError(t, err)
	})
}

// TestReverifyUnknownError checks that a node that returns an unknown error during an audit does not get marked as successful, failed, or contained.
func TestReverifyUnknownError(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		badPiece := segment.Pieces[0]
		badNode := badPiece.StorageNode

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment.Redundancy.ShareSize
		rootPieceID := segment.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, badNode, badPiece.Number, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(badPiece.Number))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            badNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		node := planet.FindNode(badNode)
		badNodeDB := node.DB.(*testblobs.BadDB)
		// return an error when the satellite requests a share
		badNodeDB.SetError(errs.New("unknown error"))

		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Unknown, 1)
		require.Equal(t, report.Unknown[0], badNode)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestMaxReverifyCount(t *testing.T) {
	const auditTimeout = time.Second
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = auditTimeout
				},
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowPiece := segment.Pieces[0]
		slowNode := slowPiece.StorageNode

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		orders := satellite.Orders.Service
		containment := satellite.DB.Containment()

		shareSize := segment.Redundancy.ShareSize
		rootPieceID := segment.RootPieceID

		limit, privateKey, cachedNodeInfo, err := orders.CreateAuditOrderLimit(ctx, slowNode, slowPiece.Number, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, privateKey, cachedNodeInfo.LastIPPort, randomIndex, shareSize, int(slowPiece.Number))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            slowNode,
			PieceID:           rootPieceID,
			StripeIndex:       randomIndex,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
			StreamID:          queueSegment.StreamID,
			Position:          queueSegment.Position,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		node := planet.FindNode(slowNode)
		slowNodeDB := node.DB.(*testblobs.SlowDB)
		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 10 * auditTimeout
		slowNodeDB.SetLatency(delay)

		oldRep, err := satellite.Reputation.Service.Get(ctx, slowNode)
		require.NoError(t, err)

		// give node enough timeouts to reach max
		for i := 0; i < planet.Satellites[0].Config.Audit.MaxReverifyCount; i++ {
			report, err := audits.Verifier.Reverify(ctx, queueSegment)
			require.NoError(t, err)
			assert.Len(t, report.Successes, 0)
			assert.Len(t, report.Fails, 0)
			assert.Len(t, report.Offlines, 0)
			assert.Len(t, report.PendingAudits, 1)
			assert.Len(t, report.Unknown, 0)
			assert.Equal(t, report.PendingAudits[0].NodeID, slowNode)

			_, err = audits.Reporter.RecordAudits(ctx, report)
			assert.NoError(t, err)

			_, err = containment.Get(ctx, slowNode)
			assert.NoError(t, err)
		}

		// final timeout should trigger failure and removal from containment
		report, err := audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)
		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 1)
		assert.Len(t, report.Unknown, 0)
		assert.Equal(t, report.PendingAudits[0].NodeID, slowNode)

		_, err = audits.Reporter.RecordAudits(ctx, report)
		assert.NoError(t, err)

		_, err = containment.Get(ctx, slowNode)
		assert.True(t, audit.ErrContainedNotFound.Has(err))

		newRep, err := satellite.Reputation.Service.Get(ctx, slowNode)
		require.NoError(t, err)
		assert.Less(t, oldRep.AuditReputationBeta, newRep.AuditReputationBeta)
	})
}
