// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testblobs"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode"
)

// TestDownloadSharesHappyPath checks that the Share.Error field of all shares
// returned by the DownloadShares method contain no error if all shares were
// downloaded successfully.
func TestDownloadSharesHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.NoError(t, share.Error)
		}
	})
}

// TestDownloadSharesOfflineNode checks that the Share.Error field of the
// shares returned by the DownloadShares method for offline nodes contain an
// error that:
//   - has the transport.Error class
//   - is not a context.DeadlineExceeded error
//   - is not an RPC error
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesOfflineNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		err = stopStorageNode(ctx, planet, stoppedNodeID)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			if share.NodeID == stoppedNodeID {
				assert.True(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
				assert.False(t, errs.Is(share.Error, context.DeadlineExceeded), "unexpected error: %+v", share.Error)
				assert.True(t, errs2.IsRPC(share.Error, codes.Unknown), "unexpected error: %+v", share.Error)
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
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// replace the piece id of the selected stripe with a new random one
		// to simulate missing piece on the storage nodes
		stripe.Segment.GetRemote().RootPieceId = storj.NewPieceID()

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := audits.Verifier.DownloadShares(ctx, limits, privateKey, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, errs2.IsRPC(share.Error, codes.NotFound), "unexpected error: %+v", share.Error)
		}
	})
}

// TestDownloadSharesDialTimeout checks that the Share.Error field of the
// shares returned by the DownloadShares method for nodes that time out on
// dialing contain an error that:
//   - has the transport.Error class
//   - is a context.DeadlineExceeded error
//   - is not an RPC error
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = upl.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KiB,
		}

		tlsOpts, err := tlsopts.NewOptions(planet.Satellites[0].Identity, tlsopts.Config{})
		require.NoError(t, err)

		newTransport := transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
			Dial: 20 * time.Millisecond,
		})

		slowClient := network.NewClient(newTransport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			planet.Satellites[0].Log.Named("verifier"),
			planet.Satellites[0].Metainfo.Service,
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			5*time.Second)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
			assert.True(t, errs.Is(share.Error, context.DeadlineExceeded), "unexpected error: %+v", share.Error)
		}
	})
}

// TestDownloadSharesDownloadTimeout checks that the Share.Error field of the
// shares returned by the DownloadShares method for nodes that are successfully
// dialed, but time out during the download of the share contain an error that:
//   - is an RPC error with code DeadlineExceeded
//   - does not have the transport.Error class
//
// If this test fails, this most probably means we made a backward-incompatible
// change that affects the audit service.
func TestDownloadSharesDownloadTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storageNodeDB := planet.StorageNodes[0].DB.(*testblobs.SlowDB)
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		upl := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = upl.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			planet.Satellites[0].Log.Named("verifier"),
			planet.Satellites[0].Metainfo.Service,
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			150*time.Millisecond)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, privateKey, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		delay := 200 * time.Millisecond
		storageNodeDB.SetLatency(delay)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, stripe.Index, shareSize)
		require.NoError(t, err)

		require.Len(t, shares, 1)
		share := shares[0]
		assert.True(t, errs2.IsRPC(share.Error, codes.DeadlineExceeded), "unexpected error: %+v", share.Error)
		assert.False(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
	})
}

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(stripe.Segment.GetRemote().GetRemotePieces()))
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierOfflineNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Discovery.Service.Discovery.Pause()

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		err = stopStorageNode(ctx, planet, stoppedNodeID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(stripe.Segment.GetRemote().GetRemotePieces())-1)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 1)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierMissingPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(stripe.Segment.GetRemote().GetRemotePieces())
		piece := stripe.Segment.GetRemote().GetRemotePieces()[0]
		pieceID := stripe.Segment.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := getStorageNode(planet, piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, planet.Satellites[0].ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 1)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KiB,
		}

		tlsOpts, err := tlsopts.NewOptions(planet.Satellites[0].Identity, tlsopts.Config{})
		require.NoError(t, err)

		newTransport := transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
			Dial: 20 * time.Millisecond,
		})

		slowClient := network.NewClient(newTransport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel with timeout.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(
			planet.Satellites[0].Log.Named("verifier"),
			planet.Satellites[0].Metainfo.Service,
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			5*time.Second)

		report, err := verifier.Verify(ctx, stripe, nil)
		require.True(t, audit.ErrNotEnoughShares.Has(err), "unexpected error: %+v", err)

		assert.Len(t, report.Successes, 0)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, len(stripe.Segment.GetRemote().GetRemotePieces()))
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDeletedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// delete the file
		err = ul.Delete(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.True(t, audit.ErrSegmentDeleted.Has(err))
		assert.Empty(t, report)
	})
}

func TestVerifierModifiedSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		// replace the file
		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.True(t, audit.ErrSegmentDeleted.Has(err))
		assert.Empty(t, report)
	})
}

func TestVerifierModifiedSegmentFailsOnce(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		// delete the piece from the first node
		origNumPieces := len(stripe.Segment.GetRemote().GetRemotePieces())
		piece := stripe.Segment.GetRemote().GetRemotePieces()[0]
		pieceID := stripe.Segment.GetRemote().RootPieceId.Derive(piece.NodeId, piece.PieceNum)
		node := getStorageNode(planet, piece.NodeId)
		err = node.Storage2.Store.Delete(ctx, planet.Satellites[0].ID(), pieceID)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 1)
		assert.Equal(t, report.Fails[0], piece.NodeId)
		assert.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)

		//refetch the stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
		assert.NoError(t, err)
		require.NotNil(t, stripe)

		report, err = audits.Verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		//verify no failures because that segment is gone
		assert.Len(t, report.Successes, origNumPieces-1)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)

		for _, newPiece := range stripe.Segment.GetRemote().GetRemotePieces() {
			assert.NotEqual(t, newPiece.NodeId, piece.NodeId)
		}
	})
}
