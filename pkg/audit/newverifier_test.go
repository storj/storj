// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
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
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink"
)

func TestDownloadSharesHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
			audit.NewReporter(planet.Satellites[0].Overlay.Service, planet.Satellites[0].DB.Containment(), 1),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, _, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.NoError(t, share.Error)
		}
	})
}

func TestDownloadSharesOfflineNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
			audit.NewReporter(planet.Satellites[0].Overlay.Service, planet.Satellites[0].DB.Containment(), 1),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		// stop the first node in the pointer
		stoppedNodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		err = stopStorageNode(ctx, planet, stoppedNodeID)
		require.NoError(t, err)

		shares, nodes, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for i, share := range shares {
			if nodes[i] == stoppedNodeID {
				assert.True(t, transport.Error.Has(share.Error))
				assert.NotEqual(t, context.DeadlineExceeded, errs.Unwrap(share.Error))
				assert.Equal(t, codes.Unknown, status.Code(errs.Unwrap(share.Error)))
			} else {
				assert.NoError(t, share.Error)
			}
		}
	})
}

func TestDownloadSharesMissingPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
			audit.NewReporter(planet.Satellites[0].Overlay.Service, planet.Satellites[0].DB.Containment(), 1),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			128*memory.B)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, _, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.Error(t, share.Error)
			assert.False(t, transport.Error.Has(share.Error))
		}
	})
}

func TestDownloadSharesDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		slowClient := network.NewClient(planet.Satellites[0].Transport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 110 * memory.KB

		verifier := audit.NewVerifier(zap.L(),
			audit.NewReporter(planet.Satellites[0].Overlay.Service, planet.Satellites[0].DB.Containment(), 1),
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, _, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.True(t, transport.Error.Has(share.Error))
			assert.Equal(t, context.DeadlineExceeded, errs.Unwrap(share.Error))
			assert.Equal(t, codes.Unknown, status.Code(errs.Unwrap(share.Error)))
		}
	})
}

func TestDownloadSharesDownloadTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		upl := planet.Uplinks[0]
		testData := make([]byte, 8*memory.KiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		// Upload with larger erasure share size to simulate longer download over slow transport client
		err = upl.UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     4,
			ErasureShareSize: 8 * memory.KiB,
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
			BytesPerSecond: 1 * memory.KiB,
		}

		slowClient := network.NewClient(planet.Satellites[0].Transport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 110 * memory.KB

		verifier := audit.NewVerifier(zap.L(),
			audit.NewReporter(planet.Satellites[0].Overlay.Service, planet.Satellites[0].DB.Containment(), 1),
			slowClient,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond)

		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()
		limits, err := planet.Satellites[0].Orders.Service.CreateAuditOrderLimits(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, stripe.Segment, nil)
		require.NoError(t, err)

		shares, _, err := verifier.DownloadShares(ctx, limits, stripe.Index, shareSize)
		require.NoError(t, err)

		for _, share := range shares {
			assert.Equal(t, codes.DeadlineExceeded, status.Code(errs.Unwrap(share.Error)))
			assert.False(t, transport.Error.Has(share.Error))
			assert.NotEqual(t, context.DeadlineExceeded, errs.Unwrap(share.Error))
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
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  5,
			SuccessThreshold: 6,
			MaxThreshold:     6,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(metainfo)

		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		transport := planet.Satellites[0].Transport
		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()
		minBytesPerSecond := 128 * memory.B

		reporter := audit.NewReporter(overlay, containment, 1)
		verifier := audit.NewVerifier(zap.L(), reporter, transport, overlay, containment, orders, planet.Satellites[0].Identity, minBytesPerSecond)
		require.NotNil(t, verifier)

		// stop some storage nodes to ensure audit can deal with it
		err = planet.StopPeer(planet.StorageNodes[0])
		require.NoError(t, err)
		err = planet.StopPeer(planet.StorageNodes[1])
		require.NoError(t, err)

		// mark stopped nodes as offline in overlay cache
		_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, planet.StorageNodes[0].ID(), false)
		require.NoError(t, err)
		_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, planet.StorageNodes[1].ID(), false)
		require.NoError(t, err)

		verifiedNodes, err := verifier.Verify(ctx, stripe, nil)
		require.NoError(t, err)

		require.Len(t, verifiedNodes.Successes, 4)
		require.Len(t, verifiedNodes.Fails, 0)
	})
}
