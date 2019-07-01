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
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		var stripe *audit.Stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		limit, err := orders.CreateAuditOrderLimit(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, pieces[0].NodeId, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, stripe.Index, shareSize, int(pieces[0].PieceNum))
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

		report, err := audits.Verifier.Reverify(ctx, stripe)
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		var stripe *audit.Stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		shareSize := stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize()

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		limit, err := orders.CreateAuditOrderLimit(ctx, planet.Satellites[0].Identity.PeerIdentity(), bucketID, pieces[0].NodeId, rootPieceID, shareSize)
		require.NoError(t, err)

		share, err := audits.Verifier.GetShare(ctx, limit, stripe.Index, shareSize, int(pieces[0].PieceNum))
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

		report, err := audits.Verifier.Reverify(ctx, stripe)
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		var stripe *audit.Stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, report.Successes, 0)
		require.Len(t, report.Offlines, 0)
		require.Len(t, report.PendingAudits, 0)
		require.Len(t, report.Fails, 1)
		require.Equal(t, report.Fails[0], pieces[0].NodeId)
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		var stripe *audit.Stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		err = stopStorageNode(ctx, planet, pieces[0].NodeId)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, stripe)
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		var stripe *audit.Stripe
		stripe, _, err = audits.Cursor.NextStripe(ctx)
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

		pieces := stripe.Segment.GetRemote().GetRemotePieces()

		rootPieceID := stripe.Segment.GetRemote().RootPieceId
		redundancy := stripe.Segment.GetRemote().GetRedundancy()

		pending := &audit.PendingAudit{
			NodeID:            pieces[0].NodeId,
			PieceID:           rootPieceID,
			StripeIndex:       stripe.Index,
			ShareSize:         redundancy.ErasureShareSize,
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
		require.NoError(t, err)

		report, err := verifier.Reverify(ctx, stripe)
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
		// - uses the cursor to get a stripe
		// - creates one pending audit for a node holding a piece for that stripe
		// - deletes the file
		// - calls reverify on that same stripe
		// - expects reverification to pass successufully and the storage node to be not in containment mode

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		nodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           stripe.Segment.GetRemote().RootPieceId,
			StripeIndex:       stripe.Index,
			ShareSize:         stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize(),
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
		}

		containment := planet.Satellites[0].DB.Containment()

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// delete the file
		err = ul.Delete(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, stripe)
		require.NoError(t, err)
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

		audits := planet.Satellites[0].Audit.Service
		err := audits.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		stripe, _, err := audits.Cursor.NextStripe(ctx)
		require.NoError(t, err)

		nodeID := stripe.Segment.GetRemote().GetRemotePieces()[0].NodeId
		pending := &audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           stripe.Segment.GetRemote().RootPieceId,
			StripeIndex:       stripe.Index,
			ShareSize:         stripe.Segment.GetRemote().GetRedundancy().GetErasureShareSize(),
			ExpectedShareHash: pkcrypto.SHA256Hash(nil),
			ReverifyCount:     0,
		}

		containment := planet.Satellites[0].DB.Containment()

		err = containment.IncrementPending(ctx, pending)
		require.NoError(t, err)

		// replace the file
		err = ul.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		report, err := audits.Verifier.Reverify(ctx, stripe)
		require.NoError(t, err)
		assert.Empty(t, report)

		_, err = containment.Get(ctx, nodeID)
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}
