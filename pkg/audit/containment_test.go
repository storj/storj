// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

func TestReverifySuccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// This is a bulky test but all it's doing is:
		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - the actual share is downloaded to make sure ExpectedShareHash is correct
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a success in the audit report

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			containment,
			orders,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)
		require.NotNil(t, verifier)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		limit, err := orders.CreateAuditOrderLimit(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, pieces[0].NodeId, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := verifier.GetShare(ctx, limit, stripe.Index, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, report.Successes, 1)
		require.Equal(t, report.Successes[0], pieces[0].NodeId)
	})
}

func TestReverifyFailMissingShare(t *testing.T) {
	t.Skip("todo: find out why a node with deleted piece yields Unknown error instead of NotFound (like it does for verify)")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - the actual share is downloaded to make sure ExpectedShareHash is correct
		// - delete piece from node
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a fail in the audit report

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			containment,
			orders,
			planet.Satellites[0].Identity,
			128*memory.B,
			5*time.Second)
		require.NotNil(t, verifier)

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		limit, err := orders.CreateAuditOrderLimit(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, pieces[0].NodeId, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := verifier.GetShare(ctx, limit, stripe.Index, shareSize, int(pieces[0].PieceNum))
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         shareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(share.Data),
			ReverifyCount:     0,
		}

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the piece from the first node
		nodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		pieceID := stripe.Segment.GetRemote().RootPieceId.Derive(nodeID)
		node := getStorageNode(planet, nodeID)
		err = node.Storage2.Store.Delete(ctx, planet.Satellites[0].ID(), pieceID)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates a pending audit for a node holding a piece for that stripe
		// - makes ExpectedShareHash have random data
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as a fail in the audit report

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		minBytesPerSecond := 128 * memory.B

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			5*time.Second)
		require.NotNil(t, verifier)

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		randBytes := make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(randBytes),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], pieces[0].NodeId)
	})
}

func TestReverifyOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audits for one node holding a piece for that stripe
		// - stop the node that has the pending audit
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as offline in the audit report

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		cursor := audit.NewCursor(planet.Satellites[0].Metainfo.Service)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		minBytesPerSecond := 128 * memory.B

		verifier := audit.NewVerifier(zap.L(),
			planet.Satellites[0].Transport,
			planet.Satellites[0].Overlay.Service,
			planet.Satellites[0].DB.Containment(),
			planet.Satellites[0].Orders.Service,
			planet.Satellites[0].Identity,
			minBytesPerSecond,
			5*time.Second)
		require.NotNil(t, verifier)

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		randBytes := make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(randBytes),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		err = stopStorageNode(ctx, planet, pieces[0].NodeId)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, report.Offlines, 1)
		require.Equal(t, report.Offlines[0], pieces[0].NodeId)
	})
}

func TestReverifyContainedDialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// - uploads random data
		// - uses the cursor to get a stripe
		// - creates pending audit for one node holding a piece for that stripe
		// - uses a slow transport client so that dial timeout will happen (a contained case)
		// - calls reverify on that same stripe
		// - expects one storage node to be marked as contained in the audit report

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

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

		pieces := stripe.Segment.GetRemote().GetRemotePieces()

		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		randBytes := make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(randBytes),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, report.PendingAudits, 1)
		require.Equal(t, report.PendingAudits[0], pending)
	})
}

func TestContainIncrementAndGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		someHash := pkcrypto.SHA256Hash(randBytes)

		input := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: someHash,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, input)
		require.NoError(t, err)

		output, err := planet.Satellites[0].DB.Containment().Get(ctx, input.NodeID)
		require.NoError(t, err)

		require.Equal(t, input, output)

		// check contained flag set to true
		node, err := planet.Satellites[0].DB.OverlayCache().Get(ctx, input.NodeID)
		require.NoError(t, err)
		require.True(t, node.Contained)

		nodeID1 := planet.StorageNodes[1].ID()
		_, err = planet.Satellites[0].DB.Containment().Get(ctx, nodeID1)
		require.Error(t, err, audit.ErrContainedNotFound.New(nodeID1.String()))
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestContainIncrementPendingEntryExists(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		hash1 := pkcrypto.SHA256Hash(randBytes)

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: hash1,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)

		randBytes = make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)
		hash2 := pkcrypto.SHA256Hash(randBytes)

		info2 := &audit.PendingAudit{
			NodeID:            info1.NodeID,
			PieceID:           storj.PieceID{},
			StripeIndex:       1,
			ShareSize:         1,
			ExpectedShareHash: hash2,
			ReverifyCount:     0,
		}

		// expect failure when an entry with the same nodeID but different expected share data already exists
		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info2)
		require.Error(t, err)
		require.True(t, audit.ErrAlreadyExists.Has(err))

		// expect reverify count for an entry to be 0 after first IncrementPending call
		pending, err := planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.EqualValues(t, 0, pending.ReverifyCount)

		// expect reverify count to be 1 after second IncrementPending call
		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)
		pending, err = planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.EqualValues(t, 1, pending.ReverifyCount)
	})
}

func TestContainDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		hash1 := pkcrypto.SHA256Hash(randBytes)

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: hash1,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)

		// check contained flag set to true
		node, err := planet.Satellites[0].DB.OverlayCache().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.True(t, node.Contained)

		isDeleted, err := planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.True(t, isDeleted)

		// check contained flag set to false
		node, err = planet.Satellites[0].DB.OverlayCache().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.False(t, node.Contained)

		// get pending audit that doesn't exist
		_, err = planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.Error(t, err, audit.ErrContainedNotFound.New(info1.NodeID.String()))
		require.True(t, audit.ErrContainedNotFound.Has(err))

		// delete pending audit that doesn't exist
		isDeleted, err = planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.False(t, isDeleted)
	})
}
