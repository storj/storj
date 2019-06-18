// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

// TestDownloadSharesHappyPath checks that the Share.Error field of all shares
// returned by the DownloadShares method contain no error if all shares were
// downloaded successfully.
func TestDownloadSharesHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		err = stopStorageNode(ctx, planet, stoppedNodeID)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			if share.NodeID == stoppedNodeID {
				assert.True(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
				assert.False(t, errs.IsFunc(share.Error, func(err error) bool {
					return err == context.DeadlineExceeded
				}), "unexpected error: %+v", share.Error)
				assert.True(t, errs.IsFunc(share.Error, func(err error) bool {
					return status.Code(err) == codes.Unknown
				}), "unexpected error: %+v", share.Error)
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		// replace the piece id of the selected stripe with a new random one
		// to simulate missing piece on the storage nodes
		stripe.Segment.GetRemote().RootPieceId = storj.NewPieceID()

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, errs.IsFunc(share.Error, func(err error) bool {
				return status.Code(err) == codes.NotFound
			}), "unexpected error: %+v", share.Error)
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		upl := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = upl.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
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
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(zap.L(),
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			5*time.Second)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
			assert.True(t, errs.IsFunc(share.Error, func(err error) bool {
				return err == context.DeadlineExceeded
			}), "unexpected error: %+v", share.Error)
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		upl := planet.Uplinks[0]
		testData := make([]byte, 32*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		// Upload with larger erasure share size to simulate longer download over slow transport client
		err = upl.UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     4,
			ErasureShareSize: 32 * memory.KiB,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		// set stripe index to 0 to ensure we are auditing large enough stripe
		// instead of the last stripe, which could be smaller
		stripe.Index = 0

		network := &transport.SimulatedNetwork{
			BytesPerSecond: 128 * memory.KiB,
		}

		slowClient := network.NewClient(planet.Satellites[0].Transport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 1 * memory.MiB

		verifier := audit.NewVerifier(zap.L(),
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			100*time.Millisecond)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, errs.IsFunc(share.Error, func(err error) bool {
				return status.Code(err) == codes.DeadlineExceeded
			}), "unexpected error: %+v", share.Error)
			assert.False(t, transport.Error.Has(share.Error), "unexpected error: %+v", share.Error)
		}
	})
}

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		report, err := verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(stripe.Segment.GetRemote().GetRemotePieces()))
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierOfflineNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Discovery.Service.Discovery.Pause()
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		// stop the first node in the pointer
		stoppedNodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		err = stopStorageNode(ctx, planet, stoppedNodeID)
		require.NoError(t, err)

		report, err := verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(stripe.Segment.GetRemote().GetRemotePieces())-1)
		assert.Len(t, report.Fails, 0)
		assert.Len(t, report.Offlines, 1)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierMissingPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)

		// delete the piece from the first node
		nodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		pieceID := stripe.Segment.GetRemote().RootPieceId.Derive(nodeID)
		node := getStorageNode(planet, nodeID)
		err = node.Storage2.Store.Delete(ctx, planet.Satellites[0].ID(), pieceID)
		require.NoError(t, err)

		report, err := verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		assert.Len(t, report.Successes, len(stripe.Segment.GetRemote().GetRemotePieces())-1)
		assert.Len(t, report.Fails, 1)
		assert.Len(t, report.Offlines, 0)
		assert.Len(t, report.PendingAudits, 0)
	})
}

func TestVerifierDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)
		stripe, _, err := cursor.NextStripe(ctx)
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
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 100 * memory.KiB

		verifier := audit.NewVerifier(zap.L(),
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

func getStorageNode(planet *testplanet.Planet, nodeID storj.NodeID) *storagenode.Peer {
	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {
			return node
		}
	}
	return nil
}

func stopStorageNode(ctx context.Context, planet *testplanet.Planet, nodeID storj.NodeID) error {
	node := getStorageNode(planet, nodeID)
	if node == nil {
		return fmt.Errorf("no such node: %s", nodeID.String())
	}

	err := planet.StopPeer(node)
	if err != nil {
		return err
	}

	// mark stopped node as offline in overlay cache
	_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, nodeID, false)
	return err
}
