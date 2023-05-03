// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/testblobs"
)

// TestDownloadSharesHappyPath checks that the Share.Error field of all shares
// returned by the DownloadShares method contain no error if all shares were
// downloaded successfully.
func TestDownloadSharesHappyPath(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.NoError(t, share.Error)
		}
	})
}

// TestDownloadSharesOfflineNode checks that the Share.Error field of the
// shares returned by the DownloadShares method for offline nodes contain an
// error that:
//   - has the rpc.Error class
//   - is not a context.DeadlineExceeded error
//   - is not an RPC error
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesOfflineNode(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		// stop the first node in the segment
		stoppedNodeID := segment.Pieces[0].StorageNode
		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(stoppedNodeID))
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			if share.NodeID == stoppedNodeID {
				assert.True(t, rpc.Error.Has(share.Error), "unexpected error: %+v", share.Error)
				assert.False(t, errs.Is(share.Error, context.DeadlineExceeded), "unexpected error: %+v", share.Error)
				assert.True(t, errs2.IsRPC(share.Error, rpcstatus.Unknown), "unexpected error: %+v", share.Error)
			} else {
				assert.NoError(t, share.Error)
			}
		}
	})
}

// TestDownloadSharesMissingPiece checks that the Share.Error field of the
// shares returned by the DownloadShares method for nodes that don't have the
// audited piece contain an RPC error with code NotFound.
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesMissingPiece(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		// replace the piece id of the selected stripe with a new random one
		// to simulate missing piece on the storage nodes
		segment.RootPieceID = storj.NewPieceID()

		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, errs2.IsRPC(share.Error, rpcstatus.NotFound), "unexpected error: %+v", share.Error)
		}
	})
}

// TestDownloadSharesDialTimeout checks that the Share.Error field of the
// shares returned by the DownloadShares method for nodes that time out on
// dialing contain an error that:
//   - has the rpc.Error class
//   - is a context.DeadlineExceeded error
//   - is not an RPC error
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesDialTimeout(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := upl.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
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
		// data from storage nodes. This will cause context to cancel with timeout.
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

		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, rpc.Error.Has(share.Error), "unexpected error: %+v", share.Error)
			assert.True(t, errs.Is(share.Error, context.DeadlineExceeded), "unexpected error: %+v", share.Error)
		}
	})
}

// TestDownloadSharesDownloadTimeout checks that the Share.Error field of the
// shares returned by the DownloadShares method for nodes that are successfully
// dialed, but time out during the download of the share contain an error that:
//   - is an RPC error with code DeadlineExceeded
//   - does not have the rpc.Error class
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesDownloadTimeout(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		storageNodeDB := planet.StorageNodes[0].DB.(*testblobs.SlowDB)

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := upl.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			satellite.Log.Named("verifier"),
			satellite.Metabase.DB,
			satellite.Dialer,
			satellite.Overlay.Service,
			satellite.DB.Containment(),
			satellite.Orders.Service,
			satellite.Identity,
			minBytesPerSecond,
			150*time.Millisecond)

		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 200 * time.Millisecond
		storageNodeDB.SetLatency(delay)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		require.Len(t, shares, 1)
		share := shares[0]
		assert.True(t, errs2.IsRPC(share.Error, rpcstatus.DeadlineExceeded), "unexpected error: %+v", share.Error)
		assert.False(t, rpc.Error.Has(share.Error), "unexpected error: %+v", share.Error)
	})
}

func TestVerifierHappyPath(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(segment.Pieces))
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierExpired(t *testing.T) {
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

		// move time into the future so the segment is expired
		audits.Verifier.SetNow(func() time.Time {
			return time.Now().Add(2 * time.Hour)
		})

		// Verify should not return an error
		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierOfflineNode(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// stop the first node in the segment
		stoppedNodeID := segment.Pieces[0].StorageNode
		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(stoppedNodeID))
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(segment.Pieces)-1)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 1)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierMissingPiece(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(segment.Pieces)
		piece := segment.Pieces[0]
		pieceID := segment.RootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 1)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierNotEnoughPieces(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// out of 4 nodes, leave one intact
		// make one to be offline.
		// make one to return `unknown error` when respond to `GET_AUDIT/GET` request.
		// delete the piece from one node which would cause audit failure
		unknownErrorNode := planet.FindNode(segment.Pieces[0].StorageNode)
		offlineNode := planet.FindNode(segment.Pieces[1].StorageNode)
		deletedPieceNode := planet.FindNode(segment.Pieces[2].StorageNode)
		deletedPieceNum := int32(segment.Pieces[2].Number)

		// return an error when the verifier attempts to download from this node
		unknownErrorDB := unknownErrorNode.DB.(*testblobs.BadDB)
		unknownErrorDB.SetError(errs.New("unknown error"))

		// stop the offline node
		err = planet.StopNodeAndUpdate(ctx, offlineNode)
		require.NoError(t, err)

		// delete piece from deletedPieceNode
		pieceID := segment.RootPieceID.Derive(deletedPieceNode.ID(), deletedPieceNum)
		err = deletedPieceNode.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.True(t, audit.ErrNotEnoughShares.Has(err))

		// without enough pieces to complete the audit,
		// offlines and unknowns should be marked, but
		// failures should not
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 1)
		assert.Len(t, report.Unknown, 1)
	})
}

func TestVerifierDialTimeout(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
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
		// data from storage nodes. This will cause context to cancel with timeout.
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

		report, err := verifier.Verify(ctx, queueSegment, nil)
		require.True(t, audit.ErrNotEnoughShares.Has(err), "unexpected error: %+v", err)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, len(segment.Pieces))
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDeletedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		segment, err := queue.Next(ctx)
		require.NoError(t, err)

		// delete the file
		err = ul.DeleteObject(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, segment, nil)
		require.NoError(t, err)
		assert.Zero(t, report.Successes)
		assert.Zero(t, report.Fails)
		assert.Zero(t, report.Offlines)
		assert.Zero(t, report.PendingAudits)
		assert.Zero(t, report.Unknown)
	})
}

func TestVerifierModifiedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		var segment metabase.Segment
		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			// remove one piece from the segment so that checkIfSegmentAltered fails
			segment, err = satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: queueSegment.StreamID,
				Position: queueSegment.Position,
			})
			require.NoError(t, err)

			err = satellite.Metabase.DB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID:      queueSegment.StreamID,
				Position:      queueSegment.Position,
				OldPieces:     segment.Pieces,
				NewPieces:     append([]metabase.Piece{segment.Pieces[0]}, segment.Pieces[2:]...),
				NewRedundancy: segment.Redundancy,
			})
			require.NoError(t, err)
		}

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)
		assert.Zero(t, report.Successes)
		assert.Zero(t, report.Fails)
		assert.Zero(t, report.Offlines)
		assert.Zero(t, report.PendingAudits)
		assert.Zero(t, report.Unknown)
	})
}

func TestVerifierReplacedSegment(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		segment, err := queue.Next(ctx)
		require.NoError(t, err)

		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			// replace the file so that checkIfSegmentAltered fails
			err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
			require.NoError(t, err)
		}

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, segment, nil)
		require.NoError(t, err)
		assert.Zero(t, report.Successes)
		assert.Zero(t, report.Fails)
		assert.Zero(t, report.Offlines)
		assert.Zero(t, report.PendingAudits)
		assert.Zero(t, report.Unknown)
	})
}

func TestVerifierModifiedSegmentFailsOnce(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(segment.Pieces)
		piece := segment.Pieces[0]
		pieceID := segment.RootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		require.Len(t, report.Fails, 1)
		assert.Equal(t, report.Fails[0], piece.StorageNode)
		assert.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
	})
}

// TestVerifierSlowDownload checks that a node that times out while sending data to the
// audit service gets put into containment mode.
func TestVerifierSlowDownload(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = 950 * time.Millisecond
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		slowNode := planet.FindNode(segment.Pieces[0].StorageNode)
		slowNodeDB := slowNode.DB.(*testblobs.SlowDB)
		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		slowNodeDB.SetLatency(3 * time.Second)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.NotContains(t, report.Successes, slowNode.ID())
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.Unknown, 0)
		require.Len(t, report.PendingAudits, 1)
		assert.Equal(t, report.PendingAudits[0].Locator.NodeID, slowNode.ID())
	})
}

// TestVerifierUnknownError checks that a node that returns an unknown error in response to an audit request
// does not get marked as successful, failed, or contained.
func TestVerifierUnknownError(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		badNode := planet.FindNode(segment.Pieces[0].StorageNode)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		// return an error when the verifier attempts to download from this node
		badNodeDB.SetError(errs.New("unknown error"))

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, 3)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Unknown, 1)
		assert.Equal(t, report.Unknown[0], badNode.ID())
	})
}

func TestAuditRepairedSegmentInExcludedCountries(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 20,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, 5, 8, 10),
				testplanet.RepairExcludedCountryCodes([]string{"FR", "BE"}),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		bucket := "testbucket"
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, bucket, "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, bucket)

		remotePieces := segment.Pieces

		numExcluded := 5
		var nodesInExcluded storj.NodeIDList
		for i := 0; i < numExcluded; i++ {
			err = planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, remotePieces[i].StorageNode, "FR")
			require.NoError(t, err)
			nodesInExcluded = append(nodesInExcluded, remotePieces[i].StorageNode)
		}

		// make extra pieces after optimal bad
		for i := int(segment.Redundancy.OptimalShares); i < len(remotePieces); i++ {
			err = planet.StopNodeAndUpdate(ctx, planet.FindNode(remotePieces[i].StorageNode))
			require.NoError(t, err)
		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)

		// Verify the segment has been repaired
		segmentAfterRepair, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, bucket)
		require.NotEqual(t, segment.Pieces, segmentAfterRepair.Pieces)
		require.Equal(t, 10, len(segmentAfterRepair.Pieces))

		// check excluded area nodes still exist
		for i, n := range nodesInExcluded {
			var found bool
			for _, p := range segmentAfterRepair.Pieces {
				if p.StorageNode == n {
					found = true
					break
				}
			}
			require.True(t, found, fmt.Sprintf("node %s not in segment, but should be\n", segmentAfterRepair.Pieces[i].StorageNode.String()))
		}
		nodesInPointer := make(map[storj.NodeID]bool)
		for _, n := range segmentAfterRepair.Pieces {
			// check for duplicates
			_, ok := nodesInPointer[n.StorageNode]
			require.False(t, ok)
			nodesInPointer[n.StorageNode] = true
		}

		lastPieceIndex := segmentAfterRepair.Pieces.Len() - 1
		lastPiece := segmentAfterRepair.Pieces[lastPieceIndex]
		for _, n := range planet.StorageNodes {
			if n.ID() == lastPiece.StorageNode {
				pieceID := segmentAfterRepair.RootPieceID.Derive(n.ID(), int32(lastPiece.Number))
				corruptPieceData(ctx, t, planet, n, pieceID)
			}
		}

		// now audit
		report, err := satellite.Audit.Verifier.Verify(ctx, audit.Segment{
			StreamID:      segmentAfterRepair.StreamID,
			Position:      segmentAfterRepair.Position,
			ExpiresAt:     segmentAfterRepair.ExpiresAt,
			EncryptedSize: segmentAfterRepair.EncryptedSize,
		}, nil)
		require.NoError(t, err)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], lastPiece.StorageNode)
	})
}

// getRemoteSegment returns a remote pointer its path from satellite.
//
//nolint:golint
func getRemoteSegment(
	ctx context.Context, t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName string,
) (_ metabase.Segment, key metabase.SegmentKey) {
	t.Helper()

	objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	require.False(t, segments[0].Inline())

	return segments[0], metabase.SegmentLocation{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  objects[0].ObjectKey,
		Position:   segments[0].Position,
	}.Encode()
}

// corruptPieceData manipulates piece data on a storage node.
func corruptPieceData(ctx context.Context, t *testing.T, planet *testplanet.Planet, corruptedNode *testplanet.StorageNode, corruptedPieceID storj.PieceID) {
	t.Helper()

	blobRef := blobstore.BlobRef{
		Namespace: planet.Satellites[0].ID().Bytes(),
		Key:       corruptedPieceID.Bytes(),
	}

	// get currently stored piece data from storagenode
	reader, err := corruptedNode.Storage2.BlobsCache.Open(ctx, blobRef)
	require.NoError(t, err)
	pieceSize, err := reader.Size()
	require.NoError(t, err)
	require.True(t, pieceSize > 0)
	pieceData := make([]byte, pieceSize)

	// delete piece data
	err = corruptedNode.Storage2.BlobsCache.Delete(ctx, blobRef)
	require.NoError(t, err)

	// create new random data
	_, err = rand.Read(pieceData)
	require.NoError(t, err)

	// corrupt piece data (not PieceHeader) and write back to storagenode
	// this means repair downloading should fail during piece hash verification
	pieceData[pieceSize-1]++ // if we don't do this, this test should fail
	writer, err := corruptedNode.Storage2.BlobsCache.Create(ctx, blobRef, pieceSize)
	require.NoError(t, err)

	n, err := writer.Write(pieceData)
	require.NoError(t, err)
	require.EqualValues(t, n, pieceSize)

	err = writer.Commit(ctx)
	require.NoError(t, err)
}

func TestIdentifyContainedNodes(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// mark a node as contained
		containedNode := segment.Pieces[0].StorageNode
		containment := satellite.DB.Containment()
		err = containment.Insert(ctx, &audit.PieceLocator{
			StreamID: testrand.UUID(),
			NodeID:   containedNode,
		})
		require.NoError(t, err)

		gotContainedNodes, err := audits.Verifier.IdentifyContainedNodes(ctx, audit.Segment{
			StreamID: segment.StreamID,
			Position: segment.Position,
		})
		require.NoError(t, err)
		require.Len(t, gotContainedNodes, 1)
		_, ok := gotContainedNodes[containedNode]
		require.True(t, ok, "expected node to be indicated as contained, but it was not")
	})
}

func TestConcurrentAuditsSuccess(t *testing.T) {
	const (
		numConcurrentAudits = 10
		minPieces           = 5
	)

	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: minPieces, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// every segment gets a piece on every node, so that every segment audit
			// hits the same set of nodes, and every node is touched by every audit
			Satellite: testplanet.ReconfigureRS(minPieces, minPieces, minPieces, minPieces),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.ReverifyWorker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]

		for n := 0; n < numConcurrentAudits; n++ {
			testData := testrand.Bytes(8 * memory.KiB)
			err := ul.Upload(ctx, satellite, "testbucket", fmt.Sprintf("test/path/%d", n), testData)
			require.NoError(t, err)
		}

		listResult, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{Limit: numConcurrentAudits})
		require.NoError(t, err)
		require.Len(t, listResult.Segments, numConcurrentAudits)

		// do all the audits at the same time; at least some nodes will get more than one at the same time
		group, auditCtx := errgroup.WithContext(ctx)
		reports := make([]audit.Report, numConcurrentAudits)

		for n, seg := range listResult.Segments {
			n := n
			seg := seg
			group.Go(func() error {
				report, err := audits.Verifier.Verify(auditCtx, audit.Segment{
					StreamID: seg.StreamID,
					Position: seg.Position,
				}, nil)
				if err != nil {
					return err
				}
				reports[n] = report
				return nil
			})
		}
		err = group.Wait()
		require.NoError(t, err)

		for _, report := range reports {
			require.Len(t, report.Fails, 0)
			require.Len(t, report.Unknown, 0)
			require.Len(t, report.PendingAudits, 0)
			require.Len(t, report.Offlines, 0)
			require.Equal(t, len(report.Successes), minPieces)

			// apply the audit results, as the audit worker would have done
			audits.Reporter.RecordAudits(ctx, report)
		}

		// nothing should be in the reverify queue
		_, err = audits.ReverifyQueue.GetNextJob(ctx, time.Minute)
		require.Error(t, err)
		require.True(t, audit.ErrEmptyQueue.Has(err), err)
	})
}

func TestConcurrentAuditsUnknownError(t *testing.T) {
	const (
		numConcurrentAudits = 10
		minPieces           = 5
		badNodes            = minPieces / 2
	)

	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: minPieces, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// every segment gets a piece on every node, so that every segment audit
			// hits the same set of nodes, and every node is touched by every audit
			Satellite: testplanet.ReconfigureRS(minPieces-badNodes, minPieces, minPieces, minPieces),
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.ReverifyWorker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]

		for n := 0; n < numConcurrentAudits; n++ {
			testData := testrand.Bytes(8 * memory.KiB)
			err := ul.Upload(ctx, satellite, "testbucket", fmt.Sprintf("test/path/%d", n), testData)
			require.NoError(t, err)
		}

		listResult, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{Limit: numConcurrentAudits})
		require.NoError(t, err)
		require.Len(t, listResult.Segments, numConcurrentAudits)

		// make ~half of the nodes time out on all responses
		for n := 0; n < badNodes; n++ {
			planet.StorageNodes[n].DB.(*testblobs.BadDB).SetError(fmt.Errorf("an unrecognized error"))
		}

		// do all the audits at the same time; at least some nodes will get more than one at the same time
		group, auditCtx := errgroup.WithContext(ctx)
		reports := make([]audit.Report, numConcurrentAudits)

		for n, seg := range listResult.Segments {
			n := n
			seg := seg
			group.Go(func() error {
				report, err := audits.Verifier.Verify(auditCtx, audit.Segment{
					StreamID: seg.StreamID,
					Position: seg.Position,
				}, nil)
				if err != nil {
					return err
				}
				reports[n] = report
				return nil
			})
		}
		err = group.Wait()
		require.NoError(t, err)

		for _, report := range reports {
			require.Len(t, report.Fails, 0)
			require.Len(t, report.Unknown, badNodes)
			require.Len(t, report.PendingAudits, 0)
			require.Len(t, report.Offlines, 0)
			require.Equal(t, len(report.Successes), minPieces-badNodes)

			// apply the audit results, as the audit worker would have done
			audits.Reporter.RecordAudits(ctx, report)
		}

		// nothing should be in the reverify queue
		_, err = audits.ReverifyQueue.GetNextJob(ctx, time.Minute)
		require.Error(t, err)
		require.True(t, audit.ErrEmptyQueue.Has(err), err)
	})
}

func TestConcurrentAuditsFailure(t *testing.T) {
	const (
		numConcurrentAudits = 10
		minPieces           = 5
		badNodes            = minPieces / 2
	)

	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: minPieces, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// every segment gets a piece on every node, so that every segment audit
			// hits the same set of nodes, and every node is touched by every audit
			Satellite: testplanet.ReconfigureRS(minPieces-badNodes, minPieces, minPieces, minPieces),
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.ReverifyWorker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]

		for n := 0; n < numConcurrentAudits; n++ {
			testData := testrand.Bytes(8 * memory.KiB)
			err := ul.Upload(ctx, satellite, "testbucket", fmt.Sprintf("test/path/%d", n), testData)
			require.NoError(t, err)
		}

		listResult, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{Limit: numConcurrentAudits})
		require.NoError(t, err)
		require.Len(t, listResult.Segments, numConcurrentAudits)

		// make ~half of the nodes return a Not Found error on all responses
		for n := 0; n < badNodes; n++ {
			// Can't make _all_ calls return errors, or the directory read verification will fail
			// (as it is triggered explicitly when ErrNotExist is returned from Open) and cause the
			// node to panic before the test is done.
			planet.StorageNodes[n].DB.(*testblobs.BadDB).SetError(os.ErrNotExist)
		}

		// do all the audits at the same time; at least some nodes will get more than one at the same time
		group, auditCtx := errgroup.WithContext(ctx)
		reports := make([]audit.Report, numConcurrentAudits)

		for n, seg := range listResult.Segments {
			n := n
			seg := seg
			group.Go(func() error {
				report, err := audits.Verifier.Verify(auditCtx, audit.Segment{
					StreamID: seg.StreamID,
					Position: seg.Position,
				}, nil)
				if err != nil {
					return err
				}
				reports[n] = report
				return nil
			})
		}
		err = group.Wait()
		require.NoError(t, err)

		for n, report := range reports {
			require.Len(t, report.Unknown, 0, n)
			require.Len(t, report.PendingAudits, 0, n)
			require.Len(t, report.Offlines, 0, n)
			require.Len(t, report.Fails, badNodes, n)
			require.Equal(t, len(report.Successes), minPieces-badNodes, n)

			// apply the audit results, as the audit worker would have done
			audits.Reporter.RecordAudits(ctx, report)
		}

		// nothing should be in the reverify queue
		_, err = audits.ReverifyQueue.GetNextJob(ctx, time.Minute)
		require.Error(t, err)
		require.True(t, audit.ErrEmptyQueue.Has(err), err)
	})
}

func TestConcurrentAuditsTimeout(t *testing.T) {
	const (
		numConcurrentAudits = 10
		minPieces           = 5
		slowNodes           = minPieces / 2
		retryInterval       = 5 * time.Minute
	)

	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: minPieces, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// every segment should get a piece on every node, so that every segment audit
			// hits the same set of nodes, and every node is touched by every audit
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// These config values are chosen to cause relatively quick timeouts
					// while allowing the non-slow nodes to complete operations
					config.Audit.MinBytesPerSecond = 100 * memory.KiB
					config.Audit.MinDownloadTimeout = time.Second
				},
				testplanet.ReconfigureRS(minPieces-slowNodes, minPieces, minPieces, minPieces),
			),
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.ReverifyWorker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]

		for n := 0; n < numConcurrentAudits; n++ {
			testData := testrand.Bytes(8 * memory.KiB)
			err := ul.Upload(ctx, satellite, "testbucket", fmt.Sprintf("test/path/%d", n), testData)
			require.NoError(t, err)
		}

		listResult, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{Limit: numConcurrentAudits})
		require.NoError(t, err)
		require.Len(t, listResult.Segments, numConcurrentAudits)

		// make ~half of the nodes time out on all responses
		for n := 0; n < slowNodes; n++ {
			planet.StorageNodes[n].DB.(*testblobs.SlowDB).SetLatency(time.Hour)
		}

		// do all the audits at the same time; at least some nodes will get more than one at the same time
		group, auditCtx := errgroup.WithContext(ctx)
		reports := make([]audit.Report, numConcurrentAudits)

		for n, seg := range listResult.Segments {
			n := n
			seg := seg
			group.Go(func() error {
				report, err := audits.Verifier.Verify(auditCtx, audit.Segment{
					StreamID: seg.StreamID,
					Position: seg.Position,
				}, nil)
				if err != nil {
					return err
				}
				reports[n] = report
				return nil
			})
		}
		err = group.Wait()
		require.NoError(t, err)

		rq := audits.ReverifyQueue.(interface {
			audit.ReverifyQueue
			TestingFudgeUpdateTime(ctx context.Context, pendingAudit *audit.PieceLocator, updateTime time.Time) error
		})

		for _, report := range reports {
			require.Len(t, report.Fails, 0)
			require.Len(t, report.Unknown, 0)
			require.Len(t, report.PendingAudits, slowNodes)
			require.Len(t, report.Offlines, 0)
			require.Equal(t, len(report.Successes), minPieces-slowNodes)

			// apply the audit results, as the audit worker would have done
			audits.Reporter.RecordAudits(ctx, report)

			// fudge the insert time backward by retryInterval so the jobs will be available to GetNextJob
			for _, pending := range report.PendingAudits {
				err := rq.TestingFudgeUpdateTime(ctx, &pending.Locator, time.Now().Add(-retryInterval))
				require.NoError(t, err)
			}
		}

		// the slow nodes should have been added to the reverify queue multiple times;
		// once for each timed-out piece fetch
		queuedReverifies := make([]*audit.ReverificationJob, 0, numConcurrentAudits*slowNodes)
		for {
			job, err := audits.ReverifyQueue.GetNextJob(ctx, retryInterval)
			if err != nil {
				if audit.ErrEmptyQueue.Has(err) {
					break
				}
				require.NoError(t, err)
			}
			queuedReverifies = append(queuedReverifies, job)
		}
		require.Len(t, queuedReverifies, numConcurrentAudits*slowNodes)

		appearancesPerNode := make(map[storj.NodeID]int)
		for _, job := range queuedReverifies {
			appearancesPerNode[job.Locator.NodeID]++
		}

		require.Len(t, appearancesPerNode, slowNodes)
		for n := 0; n < slowNodes; n++ {
			require.EqualValues(t, appearancesPerNode[planet.StorageNodes[n].ID()], numConcurrentAudits)
		}
	})
}
