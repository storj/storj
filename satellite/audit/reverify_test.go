// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
)

func TestReverifySuccess(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// This is a bulky test but all it's doing is:
		// - uploads random data
		// - uses the cursor to get a stripe
		// - calls reverify on that stripe
		// - expects one storage node to be marked as a success in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		containment := satellite.DB.Containment()

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeSuccess, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyFailMissingShare(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - delete piece from node
		// - calls reverify on that piece
		// - expects one storage node to be marked as a fail in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		reporter := audits.Reporter
		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		containment := satellite.DB.Containment()

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]
		rootPieceID := segment.RootPieceID

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		err = reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		// delete the piece from the first node
		pieceID := rootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		node.Storage2.PieceBackend.TestingDeletePiece(satellite.ID(), pieceID)

		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeFailure, outcome)

		err = reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyOffline(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audits for one node holding a piece for that stripe
		// - stop the node that has the pending audit
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as offline in the audit report

		satellite := planet.Satellites[0]
		audits := satellite.Audit
		reporter := audits.Reporter

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		err = reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
		require.NoError(t, err)

		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeNodeOffline, outcome)

		err = reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// make sure that pending audit is not removed
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, pending.NodeID)
		require.NoError(t, err)
	})
}

func TestReverifyOfflineDialTimeout(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audit for one node holding a piece for that stripe
		// - uses a slow transport client so that dial timeout will happen (an offline case)
		// - calls reverify on that same stripe
		// - expects the reverification to be alive still

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
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
		reverifier := audit.NewReverifier(
			satellite.Log.Named("reverifier"),
			verifier,
			satellite.DB.ReverifyQueue(),
			audit.Config{})

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		outcome, reputation := reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeTimedOut, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// make sure that pending audit is not removed
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, pending.NodeID)
		require.NoError(t, err)
	})
}

func TestReverifyDeletedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data to all nodes
		// - gets a segment from the audit queue
		// - creates one pending audit for a node holding a piece for that segment
		// - deletes the file
		// - calls reverify on the deleted file
		// - expects reverification to pass successfully and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		// delete the file
		err = ul.DeleteObject(ctx, satellite, "testbucket", "test/path1")
		require.NoError(t, err)

		// call reverify on the deleted file and expect OutcomeNotNecessary
		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeNotNecessary, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// expect that the node was removed from containment since the segment it was contained for has been deleted
		containment := satellite.DB.Containment()
		_, err = containment.Get(ctx, piece.StorageNode)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func cloneAndDropPiece(
	ctx context.Context, metabaseDB *metabase.DB, segment *metabase.SegmentForAudit, pieceNum int,
) error {
	newPieces := make([]metabase.Piece, len(segment.Pieces))
	copy(newPieces, segment.Pieces)
	return metabaseDB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID:      segment.StreamID,
		Position:      segment.Position,
		OldPieces:     segment.Pieces,
		NewPieces:     append(newPieces[:pieceNum], newPieces[pieceNum+1:]...),
		NewRedundancy: segment.Redundancy,
	})
}

func TestReverifyModifiedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data to an object on all nodes
		// - creates a pending audit for a particular piece of that object
		// - removes a (different) piece from the object so that the segment is modified
		// - expects reverification to pass with OutcomeNotNecessary and the storage node to be not in containment mode

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		containment := satellite.DB.Containment()

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		otherPiece := (pieceIndex + 1) % len(segment.Pieces)

		// remove a piece from the file (a piece that the contained node isn't holding)
		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			err := cloneAndDropPiece(ctx, satellite.Metabase.DB, &segment, otherPiece)
			require.NoError(t, err)
		}

		// try reverifying the piece we just removed
		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), &audit.PieceLocator{
			StreamID: segment.StreamID,
			Position: segment.Position,
			NodeID:   segment.Pieces[otherPiece].StorageNode,
			PieceNum: int(segment.Pieces[otherPiece].Number),
		})
		require.Equal(t, audit.OutcomeNotNecessary, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)
		require.Equal(t, audit.OutcomeNotNecessary, outcome)

		// expect that the node was removed from containment since the piece it was contained for is no longer part of the segment
		_, err = containment.Get(ctx, segment.Pieces[otherPiece].StorageNode)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestReverifyReplacedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		// - uploads random data to an object on all nodes
		// - creates a pending audit for a particular piece of that object
		// - re-uploads the object (with the same contents) so that the segment is modified
		// - expects reverification to pass with OutcomeNotNecessary and the storage node to be not in containment

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData1 := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			NodeID:   piece.StorageNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(piece.Number),
		}

		containment := satellite.DB.Containment()

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		// replace the file
		err = ul.Upload(ctx, satellite, "testbucket", "test/path1", testData1)
		require.NoError(t, err)

		// reverify the segment that was not modified
		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeNotNecessary, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// expect that the node was removed from containment since the segment it was contained for has been changed
		_, err = containment.Get(ctx, piece.StorageNode)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestReverifyExpired tests the case where the segment passed into Reverify is expired.
func TestReverifyExpired(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.UploadWithExpiration(ctx, satellite, "testbucket", "test/path", testData, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// move time into the future so the segment is expired
		audits.Reverifier.SetNow(func() time.Time {
			return time.Now().Add(2 * time.Hour)
		})

		pieceIndex := testrand.Intn(len(segment.Pieces))
		piece := segment.Pieces[pieceIndex]

		pending := &audit.PieceLocator{
			StreamID: segment.StreamID,
			Position: segment.Position,
			NodeID:   piece.StorageNode,
			PieceNum: int(piece.Number),
		}

		// Reverify should not return an error
		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeNotNecessary, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// expect that the node was removed from containment since the segment it was
		// contained for has expired
		_, err = satellite.DB.Containment().Get(ctx, piece.StorageNode)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

// TestReverifySlowDownload checks that a node that times out while sending data to the
// audit service gets put into containment mode.
func TestReverifySlowDownload(t *testing.T) {
	const auditTimeout = time.Second
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = auditTimeout
				},
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowPiece := segment.Pieces[0]
		slowNode := slowPiece.StorageNode
		containment := satellite.DB.Containment()

		pending := &audit.PieceLocator{
			NodeID:   slowNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(slowPiece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		node := planet.FindNode(slowNode)
		node.Storage2.PieceBackend.TestingSetLatency(10 * auditTimeout)

		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeTimedOut, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)
		require.Equal(t, audit.OutcomeTimedOut, outcome)

		// expect that the node is still in containment
		_, err = containment.Get(ctx, slowNode)
		require.NoError(t, err)
	})
}

// TestReverifyUnknownError checks that a node that returns an unknown error during an audit does not get marked as successful, failed, or contained.
func TestReverifyUnknownError(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		badPiece := segment.Pieces[0]
		badNode := badPiece.StorageNode
		containment := satellite.DB.Containment()

		pending := &audit.PieceLocator{
			NodeID:   badNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(badPiece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		node := planet.FindNode(badNode)
		node.Storage2.PieceBackend.TestingSetError(errs.New("unknown error"))

		outcome, reputation := audits.Reverifier.ReverifyPiece(ctx, zaptest.NewLogger(t), pending)
		require.Equal(t, audit.OutcomeUnknownError, outcome)

		err = audits.Reporter.RecordReverificationResult(ctx, &audit.ReverificationJob{Locator: *pending}, outcome, reputation)
		require.NoError(t, err)

		// make sure that pending audit is removed
		_, err = containment.Get(ctx, pending.NodeID)
		require.Truef(t, audit.ErrContainedNotFound.Has(err), "expected ErrContainedNotFound but got error %+v", err)
	})
}

func TestMaxReverifyCount(t *testing.T) {
	const auditTimeout = time.Second
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = auditTimeout
					// disable reputation write cache so changes are immediate
					config.Reputation.FlushInterval = 0
				},
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.ReverifyWorker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)
		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowPiece := segment.Pieces[0]
		slowNode := slowPiece.StorageNode
		containment := satellite.DB.Containment()

		pending := &audit.PieceLocator{
			NodeID:   slowNode,
			StreamID: segment.StreamID,
			Position: segment.Position,
			PieceNum: int(slowPiece.Number),
		}

		err = audits.Reporter.ReportReverificationNeeded(ctx, pending)
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		node := planet.FindNode(slowNode)
		node.Storage2.PieceBackend.TestingSetLatency(10 * auditTimeout)

		oldRep, err := satellite.Reputation.Service.Get(ctx, slowNode)
		require.NoError(t, err)

		rq := audits.ReverifyQueue.(interface {
			TestingFudgeUpdateTime(ctx context.Context, pendingAudit *audit.PieceLocator, updateTime time.Time) error
		})

		// give node enough timeouts to reach max
		for i := 0; i < satellite.Config.Audit.MaxReverifyCount; i++ {
			// run the reverify worker; each loop should complete once there are
			// no more reverifications to do in the queue
			audits.ReverifyWorker.Loop.TriggerWait()

			// make sure the node is still contained
			job, err := containment.Get(ctx, slowNode)
			require.NoError(t, err)

			// Fudge the update time of the reverification audit so that the reverification happens again
			updateTime := job.InsertedAt
			if job.LastAttempt != nil {
				updateTime = *job.LastAttempt
			}
			err = rq.TestingFudgeUpdateTime(ctx, pending, updateTime.Add(-satellite.Config.Audit.ReverificationRetryInterval-time.Microsecond))
			require.NoError(t, err)
		}

		// final timeout should trigger failure and removal from containment
		audits.ReverifyWorker.Loop.TriggerWait()

		_, err = containment.Get(ctx, slowNode)
		require.Truef(t, audit.ErrContainedNotFound.Has(err), "expected ErrContainedNotFound but got error %+v", err)

		newRep, err := satellite.Reputation.Service.Get(ctx, slowNode)
		require.NoError(t, err)
		require.Less(t, oldRep.AuditReputationBeta, newRep.AuditReputationBeta)
	})
}

func TestTimeDelayBeforeReverifies(t *testing.T) {
	const (
		auditTimeout     = time.Second
		reverifyInterval = time.Second / 4
	)
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = auditTimeout
					// disable reputation write cache so changes are immediate
					config.Reputation.FlushInterval = 0
				},
				testplanet.ReconfigureRS(2, 2, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowPiece := segment.Pieces[0]
		slowNode := planet.FindNode(slowPiece.StorageNode)
		slowNode.Storage2.PieceBackend.TestingSetLatency(10 * auditTimeout)

		report, err := audits.Verifier.Verify(ctx, audit.Segment{
			StreamID: segment.StreamID,
			Position: segment.Position,
		}, nil)
		require.NoError(t, err)

		approximateQueueTime := time.Now()
		audits.Reporter.RecordAudits(ctx, report)
		node, err := satellite.Overlay.DB.Get(ctx, slowNode.ID())
		require.NoError(t, err)
		require.True(t, node.Contained)
		pendingJob, err := satellite.DB.Containment().Get(ctx, slowNode.ID())
		require.NoError(t, err)
		require.NotNil(t, pendingJob)
		dbQueueTime := pendingJob.InsertedAt // note this is not necessarily comparable with times from time.Now()

		reverifyQueue := satellite.Audit.ReverifyQueue

		// To demonstrate that a Reverify won't happen until reverifyInterval has elapsed, we will
		// call reverifyQueue.GetNextJob up to 10 times, evenly spaced within reverifyInterval,
		// asserting that the reverification job is still there, unchanged, and that the node
		// is still contained, until after reverifyInterval.
		//
		// Yes, this is unfortunately dependent on the system clock and on sleep()s. But I've tried
		// to make it as independent of actual timing as I can.
		const (
			numCallsTarget = 10
			callInterval   = reverifyInterval / numCallsTarget
		)

		for {
			// reverify queue won't let us get the job yet
			nextJob, err := reverifyQueue.GetNextJob(ctx, reverifyInterval)
			if err == nil {
				// unless reverifyInterval has elapsed
				if time.Since(approximateQueueTime) >= reverifyInterval {
					// in which case, it's good to get this
					require.Equal(t, slowNode.ID(), nextJob.Locator.NodeID)
					require.True(t, dbQueueTime.Equal(nextJob.InsertedAt), nextJob)
					break
				}
				require.Failf(t, "Got no error", "only %s has elapsed. nextJob=%+v", time.Since(approximateQueueTime), nextJob)
			}
			require.Error(t, err)
			require.True(t, audit.ErrEmptyQueue.Has(err), err)
			require.Nil(t, nextJob)

			// reverification job is still in the queue, though
			pendingJob, err := reverifyQueue.GetByNodeID(ctx, slowNode.ID())
			require.NoError(t, err)
			require.Equal(t, slowNode.ID(), pendingJob.Locator.NodeID)
			require.True(t, dbQueueTime.Equal(pendingJob.InsertedAt), pendingJob)

			// and the node is still contained
			node, err := satellite.Overlay.DB.Get(ctx, slowNode.ID())
			require.NoError(t, err)
			require.True(t, node.Contained)

			// wait a bit
			sync2.Sleep(ctx, callInterval)
			require.NoError(t, ctx.Err())
		}

		// Now we need to demonstrate that a second Reverify won't happen until reverifyInterval
		// has elapsed again. This code will be largely the same as the first time around.

		for {
			// reverify queue won't let us get the job yet
			nextJob, err := reverifyQueue.GetNextJob(ctx, reverifyInterval)
			if err == nil {
				// unless 2*reverifyInterval has elapsed
				if time.Since(approximateQueueTime) >= 2*reverifyInterval {
					// in which case, it's good to get this
					require.Equal(t, slowNode.ID(), nextJob.Locator.NodeID)
					require.True(t, dbQueueTime.Equal(nextJob.InsertedAt), nextJob)
					break
				}
			}
			require.Error(t, err)
			require.True(t, audit.ErrEmptyQueue.Has(err), err)
			require.Nil(t, nextJob)

			// reverification job is still in the queue, though
			pendingJob, err := reverifyQueue.GetByNodeID(ctx, slowNode.ID())
			require.NoError(t, err)
			require.Equal(t, slowNode.ID(), pendingJob.Locator.NodeID)
			require.True(t, dbQueueTime.Equal(pendingJob.InsertedAt), pendingJob)
			require.True(t, pendingJob.LastAttempt.After(dbQueueTime), pendingJob)

			// and the node is still contained
			node, err := satellite.Overlay.DB.Get(ctx, slowNode.ID())
			require.NoError(t, err)
			require.True(t, node.Contained)

			// wait a bit
			sync2.Sleep(ctx, callInterval)
			require.NoError(t, ctx.Err())
		}
	})
}
