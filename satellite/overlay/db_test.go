// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storagenode"
)

func TestDQNodesLastSeenBefore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Overlay.DQStrayNodes.Loop.Pause()

		node1 := planet.StorageNodes[0]
		node2 := planet.StorageNodes[1]
		node3 := planet.StorageNodes[2]
		node1.Contact.Chore.Pause(ctx)
		node2.Contact.Chore.Pause(ctx)
		node3.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].Overlay.DB

		// initial check that node1 is not disqualified
		n1Info, err := cache.Get(ctx, node1.ID())
		require.NoError(t, err)
		require.Nil(t, n1Info.Disqualified)

		// initial check that node2 is not disqualified
		n2Info, err := cache.Get(ctx, node2.ID())
		require.NoError(t, err)
		require.Nil(t, n2Info.Disqualified)

		// gracefully exit node3
		req := &overlay.ExitStatusRequest{
			NodeID:              node3.ID(),
			ExitInitiatedAt:     time.Now(),
			ExitLoopCompletedAt: time.Now(),
			ExitFinishedAt:      time.Now(),
		}

		dossier, err := cache.UpdateExitStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, dossier.ExitStatus.ExitFinishedAt)

		// check that limit works, set limit = 1
		// run twice to DQ nodes 1 and 2
		for i := 0; i < 2; i++ {
			_, n, err := cache.DQNodesLastSeenBefore(ctx, time.Now(), 1)
			require.NoError(t, err)
			require.Equal(t, n, 1)
		}

		n1Info, err = cache.Get(ctx, node1.ID())
		require.NoError(t, err)
		require.NotNil(t, n1Info.Disqualified)
		n1DQTime := n1Info.Disqualified

		n2Info, err = cache.Get(ctx, node2.ID())
		require.NoError(t, err)
		require.NotNil(t, n2Info.Disqualified)
		n2DQTime := n2Info.Disqualified

		// there should be no more nodes for DQ
		// use higher limit to double check that DQ times
		// for nodes 1 and 2 have not changed
		_, n, err := cache.DQNodesLastSeenBefore(ctx, time.Now(), 100)
		require.NoError(t, err)
		require.Equal(t, n, 0)

		// double check that node 3 is not DQ
		n3Info, err := cache.Get(ctx, node3.ID())
		require.NoError(t, err)
		require.Nil(t, n3Info.Disqualified)

		// double check dq times not updated
		n1Info, err = cache.Get(ctx, node1.ID())
		require.NoError(t, err)
		require.NotNil(t, n1Info.Disqualified)
		require.Equal(t, n1DQTime, n1Info.Reputation.Status.Disqualified)

		n2Info, err = cache.Get(ctx, node2.ID())
		require.NoError(t, err)
		require.NotNil(t, n2Info.Disqualified)
		require.Equal(t, n2DQTime, n2Info.Reputation.Status.Disqualified)
	})
}

// In the event of a new node failing the satellite's initial pingback, its last_contact_success
// is set to '0001-01-01 00:00:00+00'. The DQ stray nodes chore DQs nodes where last_contact_success < now() - 30d.
// Make sure these nodes are not DQd with stray nodes.
func TestDQStrayNodesFailedPingback(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		sat.Overlay.DQStrayNodes.Loop.Pause()
		oc := sat.DB.OverlayCache()

		testID := testrand.NodeID()
		checkIn := overlay.NodeCheckInInfo{
			NodeID: testID,
			IsUp:   false,
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
		require.NoError(t, oc.UpdateCheckIn(ctx, checkIn, time.Now(), sat.Config.Overlay.Node))

		d, err := oc.Get(ctx, testID)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, d.Reputation.LastContactSuccess.UTC())
		require.Nil(t, d.Reputation.Status.Disqualified)

		sat.Overlay.DQStrayNodes.Loop.TriggerWait()

		d, err = oc.Get(ctx, testID)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, d.Reputation.LastContactSuccess.UTC())
		require.Nil(t, d.Reputation.Status.Disqualified)
	})
}

func TestOperatorConfig(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Operator.Wallet = fmt.Sprintf("0x%d123456789012345678901234567890123456789", index)
				config.Operator.WalletFeatures = []string{fmt.Sprintf("test_%d", index)}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeA := planet.StorageNodes[0]
		nodeB := planet.StorageNodes[1]
		nodeA.Contact.Chore.Pause(ctx)
		nodeB.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].DB.OverlayCache()

		for _, node := range []*testplanet.StorageNode{nodeA, nodeB} {
			nodeInfo, err := cache.Get(ctx, node.ID())
			require.NoError(t, err)
			require.Equal(t, node.Config.Operator.Email, nodeInfo.Operator.Email)
			require.Equal(t, node.Config.Operator.Wallet, nodeInfo.Operator.Wallet)
			require.Equal(t, []string(node.Config.Operator.WalletFeatures), nodeInfo.Operator.WalletFeatures)
		}
	})
}

func TestDBDisqualifyNode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		overlayDB := db.OverlayCache()
		now := time.Now().Truncate(time.Second).UTC()

		cases := []struct {
			Name           string
			NodeID         storj.NodeID
			DisqualifiedAt time.Time
			Reason         overlay.DisqualificationReason
		}{
			{
				Name:           "Unknown",
				NodeID:         testrand.NodeID(),
				DisqualifiedAt: now,
				Reason:         overlay.DisqualificationReasonUnknown,
			},
			{
				Name:           "Audit Failure",
				NodeID:         testrand.NodeID(),
				DisqualifiedAt: now,
				Reason:         overlay.DisqualificationReasonAuditFailure,
			},
			{
				Name:           "Suspension",
				NodeID:         testrand.NodeID(),
				DisqualifiedAt: now,
				Reason:         overlay.DisqualificationReasonSuspension,
			},
			{
				Name:           "Node Offline",
				NodeID:         testrand.NodeID(),
				DisqualifiedAt: now,
				Reason:         overlay.DisqualificationReasonNodeOffline,
			},
		}

		for _, testcase := range cases {
			t.Run(testcase.Name, func(t *testing.T) {
				checkIn := overlay.NodeCheckInInfo{
					NodeID: testcase.NodeID,
					Address: &pb.NodeAddress{
						Address: "127.0.0.1:0",
					},
					IsUp: true,
					Version: &pb.NodeVersion{
						Version:    "v0.0.0",
						CommitHash: "",
						Timestamp:  now,
					},
				}

				err := overlayDB.UpdateCheckIn(ctx, checkIn, now, overlay.NodeSelectionConfig{})
				require.NoError(t, err)

				_, err = overlayDB.DisqualifyNode(ctx, testcase.NodeID, testcase.DisqualifiedAt, testcase.Reason)
				require.NoError(t, err)

				info, err := overlayDB.Get(ctx, testcase.NodeID)
				require.NoError(t, err)
				require.NotNil(t, info.Disqualified)
				assert.Equal(t, testcase.DisqualifiedAt, info.Disqualified.UTC())
				assert.Equal(t, testcase.Reason, *info.DisqualificationReason)
			})
		}
	})
}
