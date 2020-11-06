// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink/private/storage/meta"
)

// TestDisqualificationTooManyFailedAudits does the following:
// * Create a failed audit report for a storagenode
// * Record the audit report several times and check that the node isn't
//	 disqualified until the audit reputation reaches the cut-off value.
func TestDisqualificationTooManyFailedAudits(t *testing.T) {
	var (
		auditDQCutOff = 0.4
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AuditReputationLambda = 1
				config.Overlay.Node.AuditReputationWeight = 1
				config.Overlay.Node.AuditReputationDQ = auditDQCutOff
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			satellitePeer = planet.Satellites[0]
			nodeID        = planet.StorageNodes[0].ID()
			report        = audit.Report{
				Fails: storj.NodeIDList{nodeID},
			}
		)
		satellitePeer.Audit.Worker.Loop.Pause()

		dossier, err := satellitePeer.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)

		require.Equal(t, float64(1), dossier.Reputation.AuditReputationAlpha)
		require.Equal(t, float64(0), dossier.Reputation.AuditReputationBeta)

		prevReputation := calcReputation(dossier)

		// Report the audit failure until the node gets disqualified due to many
		// failed audits.
		iterations := 1
		for ; ; iterations++ {
			_, err := satellitePeer.Audit.Reporter.RecordAudits(ctx, report, "")
			require.NoError(t, err)

			dossier, err := satellitePeer.Overlay.Service.Get(ctx, nodeID)
			require.NoError(t, err)

			reputation := calcReputation(dossier)
			require.Truef(t, prevReputation >= reputation,
				"(%d) expected reputation to remain or decrease (previous >= current): %f >= %f",
				iterations, prevReputation, reputation,
			)

			if reputation <= auditDQCutOff || reputation == prevReputation {
				require.NotNilf(t, dossier.Disqualified,
					"Disqualified (%d) - cut-off: %f, prev. reputation: %f, current reputation: %f",
					iterations, auditDQCutOff, prevReputation, reputation,
				)

				require.True(t, time.Since(*dossier.Disqualified) >= 0,
					"Disqualified should be in the past",
				)

				break
			}

			require.Nil(t, dossier.Disqualified, "Disqualified")
			prevReputation = reputation
		}

		require.True(t, iterations > 1, "the number of iterations must be at least 2")
	})
}

func calcReputation(dossier *overlay.NodeDossier) float64 {
	var (
		alpha = dossier.Reputation.AuditReputationAlpha
		beta  = dossier.Reputation.AuditReputationBeta
	)

	return alpha / (alpha + beta)
}

func TestDisqualifiedNodesGetNoDownload(t *testing.T) {
	// Uploads random data.
	// Mark a node as disqualified.
	// Check we don't get it when we require order limit.

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		satellitePeer.Audit.Worker.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellitePeer, "testbucket", "test/path", testData)
		require.NoError(t, err)

		bucket := metabase.BucketLocation{ProjectID: uplinkPeer.Projects[0].ID, BucketName: "testbucket"}

		items, _, err := satellitePeer.Metainfo.Service.List(ctx, metabase.SegmentKey{}, "", true, 10, meta.All)
		require.NoError(t, err)
		require.Equal(t, 1, len(items))

		pointer, err := satellitePeer.Metainfo.Service.Get(ctx, metabase.SegmentKey(items[0].Path))
		require.NoError(t, err)

		disqualifiedNode := pointer.GetRemote().GetRemotePieces()[0].NodeId
		err = satellitePeer.DB.OverlayCache().DisqualifyNode(ctx, disqualifiedNode)
		require.NoError(t, err)

		limits, _, err := satellitePeer.Orders.Service.CreateGetOrderLimits(ctx, bucket, pointer)
		require.NoError(t, err)
		assert.Len(t, limits, len(pointer.GetRemote().GetRemotePieces())-1)

		for _, orderLimit := range limits {
			assert.False(t, isDisqualified(t, ctx, satellitePeer, orderLimit.Limit.StorageNodeId))
			assert.NotEqual(t, orderLimit.Limit.StorageNodeId, disqualifiedNode)
		}
	})
}

func TestDisqualifiedNodesGetNoUpload(t *testing.T) {

	// - mark a node as disqualified
	// - check that we have an error if we try to create a segment using all storage nodes

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		disqualifiedNode := planet.StorageNodes[0]
		satellitePeer.Audit.Worker.Loop.Pause()

		err := satellitePeer.DB.OverlayCache().DisqualifyNode(ctx, disqualifiedNode.ID())
		require.NoError(t, err)

		request := overlay.FindStorageNodesRequest{
			RequestedCount: 4,
			ExcludedIDs:    nil,
			MinimumVersion: "", // semver or empty
		}
		nodes, err := satellitePeer.Overlay.Service.FindStorageNodesForUpload(ctx, request)
		assert.True(t, overlay.ErrNotEnoughNodes.Has(err))

		assert.Len(t, nodes, 3)
		for _, node := range nodes {
			assert.False(t, isDisqualified(t, ctx, satellitePeer, node.ID))
			assert.NotEqual(t, node.ID, disqualifiedNode)
		}

	})
}

func TestDisqualifiedNodeRemainsDisqualified(t *testing.T) {

	// - mark a node as disqualified
	// - give it high audit rate
	// - check that the node remains disqualified

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satellitePeer.Audit.Worker.Loop.Pause()

		disqualifiedNode := planet.StorageNodes[0]
		err := satellitePeer.DB.OverlayCache().DisqualifyNode(ctx, disqualifiedNode.ID())
		require.NoError(t, err)

		info := overlay.NodeCheckInInfo{
			NodeID: disqualifiedNode.ID(),
			IsUp:   true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		err = satellitePeer.DB.OverlayCache().UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)

		assert.True(t, isDisqualified(t, ctx, satellitePeer, disqualifiedNode.ID()))

		_, err = satellitePeer.Overlay.Service.BatchUpdateStats(ctx, []*overlay.UpdateRequest{{
			NodeID:                disqualifiedNode.ID(),
			AuditOutcome:          overlay.AuditSuccess,
			AuditLambda:           0, // forget about history
			AuditWeight:           1,
			AuditDQ:               0, // make sure new reputation scores are larger than the DQ thresholds
			SuspensionGracePeriod: time.Hour,
			SuspensionDQEnabled:   true,
		}})
		require.NoError(t, err)
		assert.True(t, isDisqualified(t, ctx, satellitePeer, disqualifiedNode.ID()))
	})
}

func isDisqualified(t *testing.T, ctx *testcontext.Context, satellite *testplanet.Satellite, nodeID storj.NodeID) bool {
	node, err := satellite.Overlay.Service.Get(ctx, nodeID)
	require.NoError(t, err)

	return node.Disqualified != nil
}
