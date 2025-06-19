// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"syscall"
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
			assert.Equal(t, audit.NoFailure, share.FailurePhase)
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
				assert.Equal(t, audit.DialFailure, share.FailurePhase)
			} else {
				assert.NoError(t, share.Error)
				assert.Equal(t, audit.NoFailure, share.FailurePhase)
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
			assert.Equal(t, audit.RequestFailure, share.FailurePhase)
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
			assert.Equal(t, audit.DialFailure, share.FailurePhase)
		}
	})
}

// TestDownloadSharesDialIOTimeout checks that i/o timeout dial failures are
// handled appropriately.
//
// This test differs from TestDownloadSharesDialTimeout in that it causes the
// timeout error by replacing a storage node with a black hole TCP socket,
// causing the failure directly instead of faking it with dialer.DialLatency.
func TestDownloadSharesDialIOTimeout(t *testing.T) {
	var group errgroup.Group
	// we do this shutdown outside the testplanet scope, so that we can expect
	// that planet has been shut down before waiting for the black hole goroutines
	// to finish. (They won't finish until the remote end is closed, which happens
	// during planet shutdown.)
	defer func() { assert.NoError(t, group.Wait()) }()

	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// require all nodes for each operation
			Satellite: testplanet.ReconfigureRS(4, 4, 4, 4),
		},
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		blackHoleNode := planet.StorageNodes[testrand.Intn(len(planet.StorageNodes))]
		require.NoError(t, planet.StopPeer(blackHoleNode))

		// create a black hole in place of the storage node: a socket that only reads
		// bytes and never says anything back. A connection to here using a bare TCP Dial
		// would succeed, but a TLS Dial will not be able to handshake and will time out
		// or wait forever.
		listener, err := net.Listen("tcp", blackHoleNode.Addr())
		require.NoError(t, err)
		defer func() { assert.NoError(t, listener.Close()) }()
		t.Logf("black hole listening on %s", listener.Addr())

		group.Go(func() error {
			for {
				conn, err := listener.Accept()
				if err != nil {
					// this is terrible, but is apparently the standard and correct way to check
					// for this specific error. See parseCloseError() in net/error_test.go in the
					// Go stdlib.
					assert.ErrorContains(t, err, "use of closed network connection")
					return nil
				}
				t.Logf("connection made to black hole port %s", listener.Addr())
				group.Go(func() (err error) {
					defer func() { assert.NoError(t, conn.Close()) }()

					// black hole: just read until the socket is closed on the other end
					buf := make([]byte, 1024)
					for {
						_, err = conn.Read(buf)
						if err != nil {
							if !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, io.EOF) {
								t.Fatalf("expected econnreset or eof, got %q", err.Error())
							}
							return nil
						}
					}
				})
			}
		})

		randomIndex, err := audit.GetRandomStripe(ctx, segment)
		require.NoError(t, err)
		shareSize := segment.Redundancy.ShareSize

		limits, privateKey, cachedNodesInfo, err := satellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		verifier := satellite.Audit.Verifier
		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		observed := false
		for _, share := range shares {
			if share.NodeID.Compare(blackHoleNode.ID()) == 0 {
				assert.ErrorIs(t, share.Error, context.DeadlineExceeded)
				assert.Equal(t, audit.DialFailure, share.FailurePhase)
				observed = true
			} else {
				assert.NoError(t, share.Error)
			}
		}
		assert.Truef(t, observed, "No node in returned shares matched expected node ID")
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
		Reconfigure: testplanet.Reconfigure{},
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
		planet.StorageNodes[0].Storage2.PieceBackend.TestingSetLatency(200 * time.Millisecond)

		shares, err := verifier.DownloadShares(ctx, limits, privateKey, cachedNodesInfo, randomIndex, shareSize)
		require.NoError(t, err)

		require.Len(t, shares, 1)
		share := shares[0]
		assert.True(t, errs2.IsRPC(share.Error, rpcstatus.DeadlineExceeded), "unexpected error: %+v", share.Error)
		assert.Equal(t, audit.RequestFailure, share.FailurePhase)
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(segment.Pieces)
		piece := segment.Pieces[0]
		pieceID := segment.RootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		node.Storage2.PieceBackend.TestingDeletePiece(satellite.ID(), pieceID)

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

		// out of 4 nodes, leave one intact
		// make one to be offline.
		// make one to return `unknown error` when respond to `GET_AUDIT/GET` request.
		// delete the piece from one node which would cause audit failure
		unknownErrorNode := planet.FindNode(segment.Pieces[0].StorageNode)
		offlineNode := planet.FindNode(segment.Pieces[1].StorageNode)
		deletedPieceNode := planet.FindNode(segment.Pieces[2].StorageNode)
		deletedPieceNum := int32(segment.Pieces[2].Number)

		// return an error when the verifier attempts to download from this node
		unknownErrorNode.Storage2.PieceBackend.TestingSetError(errs.New("unknown error"))

		// stop the offline node
		err = planet.StopNodeAndUpdate(ctx, offlineNode)
		require.NoError(t, err)

		// delete piece from deletedPieceNode
		pieceID := segment.RootPieceID.Derive(deletedPieceNode.ID(), deletedPieceNum)
		deletedPieceNode.Storage2.PieceBackend.TestingDeletePiece(satellite.ID(), pieceID)

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

		var segment metabase.SegmentForAudit
		audits.Verifier.OnTestingCheckSegmentAlteredHook = func() {
			// remove one piece from the segment so that checkIfSegmentAltered fails
			segment, err = satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// delete the piece from the first node
		origNumPieces := len(segment.Pieces)
		piece := segment.Pieces[0]
		pieceID := segment.RootPieceID.Derive(piece.StorageNode, int32(piece.Number))
		node := planet.FindNode(piece.StorageNode)
		node.Storage2.PieceBackend.TestingDeletePiece(satellite.ID(), pieceID)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, origNumPieces-1)
		require.Len(t, report.Fails, 1)
		assert.Equal(t, metabase.Piece{
			StorageNode: piece.StorageNode,
			Number:      piece.Number,
		}, report.Fails[0])
		require.NotNil(t, report.Segment)
		assert.Equal(t, segment.StreamID, report.Segment.StreamID)
		assert.Equal(t, segment.Position, report.Segment.Position)
		assert.Equal(t, segment.Redundancy, report.Segment.Redundancy)
		assert.Equal(t, segment.Pieces, report.Segment.Pieces)
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		// make downloads on storage node slower than the timeout on the satellite for downloading shares
		slowNode := planet.FindNode(segment.Pieces[0].StorageNode)
		slowNode.Storage2.PieceBackend.TestingSetLatency(3 * time.Second)

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

		// return an error when the verifier attempts to download from this node
		badNode := planet.FindNode(segment.Pieces[0].StorageNode)
		badNode.Storage2.PieceBackend.TestingSetError(errs.New("unknown error"))

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
					config.Repairer.MaxExcessRateOptimalThreshold = 0.0
				},
				testplanet.ReconfigureRS(3, 5, 8, 10),
				testplanet.RepairExcludedCountryCodes([]string{"FR", "BE"}),
			),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		bucket := metabase.BucketName("testbucket")
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, bucket.String(), "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, bucket)

		remotePieces := segment.Pieces

		numExcluded := 5
		var nodesInExcluded storj.NodeIDList
		for i := 0; i < numExcluded; i++ {
			planet.FindNode(remotePieces[i].StorageNode).Contact.Chore.Pause(ctx)
			err = planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, remotePieces[i].StorageNode, "FR")
			require.NoError(t, err)
			nodesInExcluded = append(nodesInExcluded, remotePieces[i].StorageNode)
		}

		// make extra pieces after optimal bad, so we know there are exactly OptimalShares
		// retrievable shares. numExcluded of them are in an excluded country.
		for i := int(segment.Redundancy.OptimalShares); i < len(remotePieces); i++ {
			err = planet.StopNodeAndUpdate(ctx, planet.FindNode(remotePieces[i].StorageNode))
			require.NoError(t, err)
		}

		// trigger repair checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		count, err := satellite.Repair.Queue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		satellite.Repair.Repairer.Loop.TriggerWait()
		require.NoError(t, satellite.Repair.Repairer.WaitForPendingRepairs(ctx))

		// Verify that the segment was removed
		count, err = satellite.Repair.Queue.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)

		// Verify the segment has been repaired
		segmentAfterRepair, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, bucket)
		require.NotEqual(t, segment.Pieces, segmentAfterRepair.Pieces)

		// TODO: this check is currently disabled until a better option for the following problem is
		// found. first of all, the hashstore does not allow overwriting a piece even with the same
		// data. what can happpen is that the initial upload can upload a piece to a hashstore node
		// but not include it in the segment due to long tail cancellation. then, when we repair the
		// segment, the same node can be selected for the same piece since it's not part of the
		// segment. the upload fails and so we end up with 9 pieces after repair instead of 10. :(
		if false {
			require.Equal(t, 10, len(segmentAfterRepair.Pieces))
		}

		// the number of nodes that should still be online holding intact pieces, not in
		// excluded countries
		expectHealthyNodes := int(segment.Redundancy.OptimalShares) - numExcluded
		// repair should make this many new pieces to get the segment up to OptimalShares
		// shares, not counting excluded-country nodes
		expectNewPieces := int(segment.Redundancy.OptimalShares) - expectHealthyNodes
		// so there should be this many pieces after repair, not counting excluded-country
		// nodes
		expectPiecesAfterRepair := expectHealthyNodes + expectNewPieces
		// so there should be this many excluded-country pieces left in the segment (we
		// couldn't keep all of them, or we would have had more than TotalShares pieces).
		expectRemainingExcluded := int(segment.Redundancy.TotalShares) - expectPiecesAfterRepair

		found := 0
		for _, nodeID := range nodesInExcluded {
			for _, p := range segmentAfterRepair.Pieces {
				if p.StorageNode == nodeID {
					found++
					break
				}
			}
		}
		require.Equal(t, expectRemainingExcluded, found, "found wrong number of excluded-country pieces after repair")
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
				n.Storage2.PieceBackend.TestingCorruptPiece(satellite.ID(), pieceID)
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
		require.Equal(t, metabase.Piece{
			StorageNode: lastPiece.StorageNode,
			Number:      lastPiece.Number,
		}, report.Fails[0])
		require.NotNil(t, report.Segment)
		assert.Equal(t, segmentAfterRepair.StreamID, report.Segment.StreamID)
		assert.Equal(t, segmentAfterRepair.Position, report.Segment.Position)
		assert.Equal(t, segmentAfterRepair.Redundancy, report.Segment.Redundancy)
		assert.Equal(t, segmentAfterRepair.Pieces, report.Segment.Pieces)
	})
}

// getRemoteSegment returns a remote pointer its path from satellite.
//
//nolint:golint
func getRemoteSegment(
	ctx context.Context, t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName metabase.BucketName,
) (_ metabase.SegmentForAudit, key metabase.SegmentKey) {
	t.Helper()

	objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	require.False(t, segments[0].Inline())

	return metabase.SegmentForAudit{
			StreamID:      segments[0].StreamID,
			Position:      segments[0].Position,
			CreatedAt:     segments[0].CreatedAt,
			RepairedAt:    segments[0].RepairedAt,
			ExpiresAt:     segments[0].ExpiresAt,
			RootPieceID:   segments[0].RootPieceID,
			EncryptedSize: segments[0].EncryptedSize,
			Redundancy:    segments[0].Redundancy,
			Pieces:        segments[0].Pieces,
			Placement:     segments[0].Placement,
		}, metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
			ObjectKey:  objects[0].ObjectKey,
			Position:   segments[0].Position,
		}.Encode()
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

		segment, err := satellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
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
			planet.StorageNodes[n].Storage2.PieceBackend.TestingSetError(errors.New("an unrecognized error"))
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
			planet.StorageNodes[n].Storage2.PieceBackend.TestingSetError(fs.ErrNotExist)
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
			planet.StorageNodes[n].Storage2.PieceBackend.TestingSetLatency(time.Hour)
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
