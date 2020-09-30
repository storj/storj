// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
)

// TestDownloadSharesHappyPath checks that the Share.Error field of all shares
// returned by the DownloadShares method contain no error if all shares were
// downloaded successfully.
func TestDownloadSharesHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: uplink.Projects[0].ID, BucketName: "testbucket"}

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, bucket, pointer, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedIPsAndPorts, randomIndex, shareSize)
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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: uplink.Projects[0].ID, BucketName: "testbucket"}

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, bucket, pointer, nil)
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := pointer.GetRemote().GetRemotePieces()[0].NodeId
		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(stoppedNodeID))
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedIPsAndPorts, randomIndex, shareSize)
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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: uplink.Projects[0].ID, BucketName: "testbucket"}

		// replace the piece id of the selected stripe with a new random one
		// to simulate missing piece on the storage nodes
		pointer.GetRemote().RootPieceId = storj.NewPieceID()

		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, bucket, pointer, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, cachedIPsAndPorts, randomIndex, shareSize)
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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := upl.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: upl.Projects[0].ID, BucketName: "testbucket"}

		tlsOptions, err := tlsopts.NewOptions(satellite.Identity, tlsopts.Config{}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)
		dialer.DialTimeout = 20 * time.Millisecond
		dialer.DialLatency = 200 * time.Second

		connector := rpc.NewDefaultTCPConnector(nil)
		connector.TransferRate = 1 * memory.KB
		dialer.Connector = connector

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
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

		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, bucket, pointer, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedIPsAndPorts, randomIndex, shareSize)
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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storageNodeDB := planet.StorageNodes[0].DB.(*testblobs.SlowDB)

		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := upl.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: upl.Projects[0].ID, BucketName: "testbucket"}

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		randomIndex, err := audit.GetRandomStripe(ctx, pointer)
		require.NoError(t, err)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			satellite.Log.Named("verifier"),
			satellite.Metainfo.Service,
			satellite.Dialer,
			satellite.Overlay.Service,
			satellite.DB.Containment(),
			satellite.Orders.Service,
			satellite.Identity,
			minBytesPerSecond,
			150*time.Millisecond)

		shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, bucket, pointer, nil)
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 200 * time.Millisecond
		storageNodeDB.SetLatency(delay)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedIPsAndPorts, randomIndex, shareSize)
		require.NoError(t, err)

		require.Len(t, shares, 1)
		share := shares[0]
		assert.True(t, errs2.IsRPC(share.Error, rpcstatus.DeadlineExceeded), "unexpected error: %+v", share.Error)
		assert.False(t, rpc.Error.Has(share.Error), "unexpected error: %+v", share.Error)
	})
}

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(pointer.GetRemote().GetRemotePieces()))
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierExpired(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		// set pointer's expiration date to be already expired
		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)
		oldPointerBytes, err := pb.Marshal(pointer)
		require.NoError(t, err)
		newPointer := &pb.Pointer{}
		err = pb.Unmarshal(oldPointerBytes, newPointer)
		require.NoError(t, err)
		newPointer.ExpirationDate = time.Now().Add(-1 * time.Hour)
		newPointerBytes, err := pb.Marshal(newPointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, storage.Key(path), oldPointerBytes, newPointerBytes)
		require.NoError(t, err)

		// Verify should not return an error
		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		// Verify should delete the expired segment
		pointer, err = satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.Error(t, err)
		require.Nil(t, pointer)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierOfflineNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := pointer.GetRemote().GetRemotePieces()[0].NodeId
		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(stoppedNodeID))
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(pointer.GetRemote().GetRemotePieces())-1)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 1)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierMissingPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(pointer.GetRemote().GetRemotePieces())
		piece := pointer.GetRemote().GetRemotePieces()[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := planet.FindNode(piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 1)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

// TestVerifierMissingPieceHashesNotVerified tests that if piece hashes were not verified for a pointer,
// a node that fails an audit for that pointer does not get marked as failing an audit, but is removed from
// the pointer.
func TestVerifierMissingPieceHashesNotVerified(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		// update pointer to have PieceHashesVerified false
		err = satellite.Metainfo.Service.UnsynchronizedDelete(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)
		pointer.PieceHashesVerified = false
		err = satellite.Metainfo.Service.Put(ctx, metabase.SegmentKey(path), pointer)
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(pointer.GetRemote().GetRemotePieces())
		piece := pointer.GetRemote().GetRemotePieces()[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := planet.FindNode(piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		// expect no failed audit
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(satellite.Identity, tlsopts.Config{}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)
		dialer.DialTimeout = 20 * time.Millisecond
		dialer.DialLatency = 200 * time.Second

		connector := rpc.NewDefaultTCPConnector(nil)
		connector.TransferRate = 1 * memory.KB
		dialer.Connector = connector

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
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

		report, err := verifier.Verify(ctx, path, nil)
		require.True(t, audit.ErrNotEnoughShares.Has(err), "unexpected error: %+v", err)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, len(pointer.GetRemote().GetRemotePieces()))
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDeletedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		// delete the file
		err = ul.DeleteObject(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)
		assert.Empty(t, report)
	})
}

func TestVerifierModifiedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		metainfo := satellite.Metainfo.Service

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		path, err := queue.Next()
		require.NoError(t, err)

		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			// remove one piece from the segment so that checkIfSegmentAltered fails
			pointer, err := metainfo.Get(ctx, metabase.SegmentKey(path))
			require.NoError(t, err)
			pieceToRemove := pointer.Remote.RemotePieces[0]
			_, err = metainfo.UpdatePieces(ctx, metabase.SegmentKey(path), pointer, nil, []*pb.RemotePiece{pieceToRemove})
			require.NoError(t, err)
		}

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)
		assert.Empty(t, report)
	})
}

func TestVerifierReplacedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			// replace the file so that checkIfSegmentAltered fails
			err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
			require.NoError(t, err)
		}

		// Verify should not return an error, but report should be empty
		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)
		assert.Empty(t, report)
	})
}

func TestVerifierModifiedSegmentFailsOnce(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(pointer.GetRemote().GetRemotePieces())
		piece := pointer.GetRemote().GetRemotePieces()[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := planet.FindNode(piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 1)
		assert.Equal(t, report.Fails[0], piece.NodeId)
		assert.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
	})
}

// TestVerifierSlowDownload checks that a node that times out while sending data to the
// audit service gets put into containment mode.
func TestVerifierSlowDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// These config values are chosen to force the slow node to time out without timing out on the three normal nodes
				config.Audit.MinBytesPerSecond = 100 * memory.KiB
				config.Audit.MinDownloadTimeout = 950 * time.Millisecond

				config.Metainfo.RS.MinThreshold = 2
				config.Metainfo.RS.RepairThreshold = 2
				config.Metainfo.RS.SuccessThreshold = 4
				config.Metainfo.RS.TotalThreshold = 4
			},
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		slowNode := planet.FindNode(pointer.Remote.RemotePieces[0].NodeId)
		slowNodeDB := slowNode.DB.(*testblobs.SlowDB)
		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 1 * time.Second
		slowNodeDB.SetLatency(delay)

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.NotContains(t, report.Successes, slowNode.ID())
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.Unknown, 0)
		assert.Len(t, report.PendingAudits, 1)
		assert.Equal(t, report.PendingAudits[0].NodeID, slowNode.ID())
	})
}

// TestVerifierUnknownError checks that a node that returns an unknown error in response to an audit request
// does not get marked as successful, failed, or contained.
func TestVerifierUnknownError(t *testing.T) {
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		badNode := planet.FindNode(pointer.Remote.RemotePieces[0].NodeId)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		// return an error when the verifier attempts to download from this node
		badNodeDB.SetError(errs.New("unknown error"))

		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		require.Len(t, report.Successes, 3)
		require.Len(t, report.Fails, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Unknown, 1)
		require.Equal(t, report.Unknown[0], badNode.ID())
	})
}

func TestVerifyPieceHashes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		nodes := storj.NodeIDList{
			planet.StorageNodes[0].ID(),
			planet.StorageNodes[1].ID(),
			planet.StorageNodes[2].ID(),
			planet.StorageNodes[3].ID(),
			planet.StorageNodes[4].ID(),
			planet.StorageNodes[5].ID(),
		}

		// happy path test cases
		for i, tt := range []struct {
			report  audit.Report
			err     error
			changed bool
		}{
			{ // empty report is sometimes returned if the segment was expired or deleted.
				report:  audit.Report{},
				changed: false,
			},
			{ // all nodes from the pointer responded successfully to the audit
				report:  audit.Report{Successes: nodes},
				changed: true,
			},
			{ // one node failed the audit
				report:  audit.Report{Successes: nodes[1:], Fails: nodes[:1]},
				changed: true,
			},
			{ // 4 nodes failed the audit
				report:  audit.Report{Successes: nodes[4:], Fails: nodes[:4]},
				changed: true,
			},
			{ // one node was offline
				report:  audit.Report{Successes: nodes[1:], Offlines: nodes[:1]},
				changed: true,
			},
			{ // 4 nodes were offline
				report:  audit.Report{Successes: nodes[4:], Offlines: nodes[:4]},
				changed: true,
			},
			{ // one node was contained and scheduled for reverification
				report:  audit.Report{Successes: nodes[1:], PendingAudits: []*audit.PendingAudit{{NodeID: nodes[0]}}},
				changed: true,
			},
			{ // 4 nodes were contained and scheduled for reverification
				report:  audit.Report{Successes: nodes[4:], PendingAudits: []*audit.PendingAudit{{NodeID: nodes[0]}, {NodeID: nodes[1]}, {NodeID: nodes[2]}, {NodeID: nodes[3]}}},
				changed: true,
			},
			{ // one node returned unknown error
				report:  audit.Report{Successes: nodes[1:], Unknown: nodes[:1]},
				changed: true,
			},
			{ // 4 nodes returned unknown error
				report:  audit.Report{Successes: nodes[4:], Unknown: nodes[:4]},
				changed: true,
			},
			{ // one node failed the audit and 2 nodes were offline
				report:  audit.Report{Successes: nodes[3:], Fails: nodes[:1], Offlines: nodes[1:3]},
				changed: true,
			},
			{ // one node failed the audit, one was offline, one was contained, and one returned unknown error
				report:  audit.Report{Successes: nodes[4:], Fails: nodes[:1], Offlines: nodes[1:2], PendingAudits: []*audit.PendingAudit{{NodeID: nodes[2]}}, Unknown: nodes[3:4]},
				changed: true,
			},
			{ // remaining nodes are below repair threshold
				report:  audit.Report{Successes: nodes[5:], Offlines: nodes[:5]},
				changed: false,
			},
			{ // Verify returns an error
				report:  audit.Report{},
				err:     errors.New("test error"),
				changed: false,
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			testReport := tt.report
			testErr := tt.err

			audits.Verifier.OnTestingVerifyMockFunc = func() (audit.Report, error) {
				return testReport, testErr
			}

			ul := planet.Uplinks[0]
			testData := testrand.Bytes(8 * memory.KiB)

			err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
			require.NoError(t, err)

			keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
			require.NoError(t, err, errTag)
			require.Equal(t, 1, len(keys))

			key := metabase.SegmentKey(keys[0])

			// verifying segments with piece_hashes_verified = true should return no error and changed = false
			changed, err := audits.Verifier.VerifyPieceHashes(ctx, string(key), false)
			require.NoError(t, err, errTag)
			assert.False(t, changed, errTag)

			// assert that piece_hashes_verified = true before setting it to false
			pointer, err := satellite.Metainfo.Service.Get(ctx, key)
			require.NoError(t, err, errTag)
			require.True(t, pointer.PieceHashesVerified, errTag)

			// set the piece_hashes_verified to false and store it in the pointer
			pointer.PieceHashesVerified = false
			err = satellite.Metainfo.Service.UnsynchronizedPut(ctx, key, pointer)
			require.NoError(t, err, errTag)

			// verifying (dry run) segments with piece_hashes_verified = false should return no error and changed = true
			changed, err = audits.Verifier.VerifyPieceHashes(ctx, string(key), true)
			assert.Equal(t, tt.err, err, errTag)
			assert.Equal(t, tt.changed, changed, errTag)

			// assert that piece_hashes_verified is still false after the dry run
			dryRunPointer, err := satellite.Metainfo.Service.Get(ctx, key)
			require.NoError(t, err, errTag)
			assert.False(t, dryRunPointer.PieceHashesVerified, errTag)

			// assert the no piece was removed from the pointer by the dry run
			for i, piece := range dryRunPointer.Remote.RemotePieces {
				require.GreaterOrEqual(t, len(pointer.Remote.RemotePieces), i, errTag)
				assert.Equal(t, pointer.Remote.RemotePieces[i].NodeId, piece.NodeId, errTag)
			}

			// verifying (no dry run) segments with piece_hashes_verified = false should return no error and changed = true
			changed, err = audits.Verifier.VerifyPieceHashes(ctx, string(key), false)
			assert.Equal(t, tt.err, err, errTag)
			assert.Equal(t, tt.changed, changed, errTag)

			// assert that piece_hashes_verified = true if the segment was verified
			verifiedPointer, err := satellite.Metainfo.Service.Get(ctx, key)
			require.NoError(t, err, errTag)
			assert.Equal(t, tt.changed, verifiedPointer.PieceHashesVerified, errTag)

			if changed {
				// assert the remaining pieces in the pointer are the expected ones
				for _, piece := range verifiedPointer.Remote.RemotePieces {
					assert.Contains(t, tt.report.Successes, piece.NodeId, errTag)
					assert.NotContains(t, tt.report.Fails, piece.NodeId, errTag)
					assert.NotContains(t, tt.report.Offlines, piece.NodeId, errTag)
					assert.NotContains(t, tt.report.Unknown, piece.NodeId, errTag)
					for _, pending := range tt.report.PendingAudits {
						assert.NotEqual(t, pending.NodeID, piece.NodeId, errTag)
					}
				}
			} else {
				// assert the no piece was removed from the pointer if it wasn't verified
				for i, piece := range verifiedPointer.Remote.RemotePieces {
					require.GreaterOrEqual(t, len(pointer.Remote.RemotePieces), i, errTag)
					assert.Equal(t, pointer.Remote.RemotePieces[i].NodeId, piece.NodeId, errTag)
				}
			}

			// fixing non-existing object should return no error and changed = false
			err = satellite.Metainfo.Service.UnsynchronizedDelete(ctx, key)
			require.NoError(t, err, errTag)

			changed, err = audits.Verifier.VerifyPieceHashes(ctx, string(key), false)
			require.NoError(t, err, errTag)
			assert.False(t, changed, errTag)
		}
	})
}

func TestVerifierMissingPieceHashesNotVerified_UsedToVerifyPieceHashes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
		path, err := queue.Next()
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)

		// update pointer to have PieceHashesVerified false
		err = satellite.Metainfo.Service.UnsynchronizedDelete(ctx, metabase.SegmentKey(path))
		require.NoError(t, err)
		pointer.PieceHashesVerified = false
		err = satellite.Metainfo.Service.Put(ctx, metabase.SegmentKey(path), pointer)
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(pointer.GetRemote().GetRemotePieces())
		piece := pointer.GetRemote().GetRemotePieces()[0]
		pieceID := pointer.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := planet.FindNode(piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		audits.Verifier.UsedToVerifyPieceHashes = true
		report, err := audits.Verifier.Verify(ctx, path, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		// expect a failed audit
		assert.Len(t, report.Fails, 1)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}
