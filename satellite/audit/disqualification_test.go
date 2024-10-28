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
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// TestDisqualificationTooManyFailedAudits does the following:
//   - Create a failed audit report for a storagenode
//   - Record the audit report several times and check that the node isn't
//     disqualified until the audit reputation reaches the cut-off value.
func TestDisqualificationTooManyFailedAudits(t *testing.T) {
	var (
		auditDQCutOff = 0.96
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = auditDQCutOff
				// disable reputation write cache so changes are immediate
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			satellitePeer = planet.Satellites[0]
			nodeID        = planet.StorageNodes[0].ID()
			report        = audit.Report{
				Fails: metabase.Pieces{{StorageNode: nodeID}},
			}
		)
		satellitePeer.Audit.Worker.Loop.Pause()

		dossier, err := satellitePeer.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)

		require.Nil(t, dossier.Disqualified)

		satellitePeer.Audit.Reporter.RecordAudits(ctx, audit.Report{
			Successes: storj.NodeIDList{nodeID},
		})

		reputationInfo, err := satellitePeer.Reputation.Service.Get(ctx, nodeID)
		require.NoError(t, err)

		prevReputation := calcReputation(reputationInfo)

		// Report the audit failure until the node gets disqualified due to many
		// failed audits.
		iterations := 1
		for ; ; iterations++ {
			satellitePeer.Audit.Reporter.RecordAudits(ctx, report)

			reputationInfo, err := satellitePeer.Reputation.Service.Get(ctx, nodeID)
			require.NoError(t, err)

			reputation := calcReputation(reputationInfo)
			require.LessOrEqual(t, reputation, prevReputation,
				"(%d) expected reputation to remain or decrease (current <= previous)",
				iterations,
			)

			if reputation <= auditDQCutOff || reputation == prevReputation {
				require.NotNilf(t, reputationInfo.Disqualified,
					"Not disqualified, but should have been (iteration %d) - cut-off: %f, prev. reputation: %f, current reputation: %f",
					iterations, auditDQCutOff, prevReputation, reputation,
				)

				require.GreaterOrEqual(t, time.Since(*reputationInfo.Disqualified), time.Duration(0),
					"Disqualified should be in the past",
				)

				break
			}

			require.Nil(t, reputationInfo.Disqualified, "Disqualified")
			prevReputation = reputation
		}

		require.Greater(t, iterations, 1, "the number of iterations must be at least 2")
	})
}

func calcReputation(dossier *reputation.Info) float64 {
	var (
		alpha = dossier.AuditReputationAlpha
		beta  = dossier.AuditReputationBeta
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

		segments, err := satellitePeer.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, len(segments))

		segment := segments[0]
		disqualifiedNode := segment.Pieces[0].StorageNode

		err = satellitePeer.Reputation.Service.TestDisqualifyNode(ctx, disqualifiedNode, overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		limits, _, err := satellitePeer.Orders.Service.CreateGetOrderLimits(ctx, uplinkPeer.Identity.PeerIdentity(), bucket, segment, 0, 0)
		require.NoError(t, err)

		notNilLimits := []*pb.AddressedOrderLimit{}
		for _, orderLimit := range limits {
			if orderLimit.Limit != nil {
				notNilLimits = append(notNilLimits, orderLimit)
			}
		}
		assert.Len(t, notNilLimits, len(segment.Pieces)-1)

		for _, orderLimit := range notNilLimits {
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

		err := satellitePeer.Reputation.Service.TestDisqualifyNode(ctx, disqualifiedNode.ID(), overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		request := overlay.FindStorageNodesRequest{
			RequestedCount:  4,
			AlreadySelected: nil,
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.MinimumDiskSpace = 10 * memory.MB
				config.Reputation.AuditLambda = 0 // forget about history
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0 // make sure new reputation scores are larger than the DQ thresholds
				config.Reputation.SuspensionGracePeriod = time.Hour
				config.Reputation.SuspensionDQEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satellitePeer.Audit.Worker.Loop.Pause()

		disqualifiedNode := planet.StorageNodes[0]
		err := satellitePeer.Reputation.Service.TestDisqualifyNode(ctx, disqualifiedNode.ID(), overlay.DisqualificationReasonUnknown)
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
		node, err := satellitePeer.Overlay.Service.Get(ctx, disqualifiedNode.ID())
		require.NoError(t, err)
		err = satellitePeer.Reputation.Service.ApplyAudit(ctx, disqualifiedNode.ID(), overlay.ReputationStatus{Disqualified: node.Disqualified}, reputation.AuditSuccess)
		require.NoError(t, err)
		assert.True(t, isDisqualified(t, ctx, satellitePeer, disqualifiedNode.ID()))
	})
}

func isDisqualified(t *testing.T, ctx *testcontext.Context, satellite *testplanet.Satellite, nodeID storj.NodeID) bool {
	node, err := satellite.Overlay.Service.Get(ctx, nodeID)
	require.NoError(t, err)

	return node.Disqualified != nil
}
