// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestStatDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testDatabase(ctx, t, db.OverlayCache())
	})
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testDatabase(ctx, t, db.OverlayCache())
	})
}

func testDatabase(ctx context.Context, t *testing.T, cache overlay.DB) {
	{ // TestKnownUnreliableOrOffline
		for i, tt := range []struct {
			nodeID           storj.NodeID
			suspended        bool
			disqualified     bool
			offline          bool
			gracefullyexited bool
		}{
			{storj.NodeID{1}, false, false, false, false}, // good
			{storj.NodeID{2}, false, true, false, false},  // disqualified
			{storj.NodeID{3}, true, false, false, false},  // suspended
			{storj.NodeID{4}, false, false, true, false},  // offline
			{storj.NodeID{5}, false, false, false, true},  // gracefully exited
		} {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			d := overlay.NodeCheckInInfo{
				NodeID:     tt.nodeID,
				Address:    &pb.NodeAddress{Address: addr, Transport: pb.NodeTransport_TCP_TLS_GRPC},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				Capacity:   &pb.NodeCapacity{},
				IsUp:       true,
			}
			err := cache.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			if tt.suspended {
				err = cache.SuspendNode(ctx, tt.nodeID, time.Now())
				require.NoError(t, err)
			}
			if tt.disqualified {
				err = cache.DisqualifyNode(ctx, tt.nodeID)
				require.NoError(t, err)
			}
			if tt.offline {
				checkInInfo := getNodeInfo(tt.nodeID)
				err = cache.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-2*time.Hour), overlay.NodeSelectionConfig{})
				require.NoError(t, err)
			}
			if tt.gracefullyexited {
				req := &overlay.ExitStatusRequest{
					NodeID:              tt.nodeID,
					ExitInitiatedAt:     time.Now(),
					ExitLoopCompletedAt: time.Now(),
					ExitFinishedAt:      time.Now(),
				}
				_, err := cache.UpdateExitStatus(ctx, req)
				require.NoError(t, err)
			}
		}

		nodeIds := storj.NodeIDList{
			storj.NodeID{1}, storj.NodeID{2},
			storj.NodeID{3}, storj.NodeID{4},
			storj.NodeID{5}, storj.NodeID{6},
		}
		criteria := &overlay.NodeCriteria{OnlineWindow: time.Hour}

		invalid, err := cache.KnownUnreliableOrOffline(ctx, criteria, nodeIds)
		require.NoError(t, err)

		require.Contains(t, invalid, storj.NodeID{2}) // disqualified
		require.Contains(t, invalid, storj.NodeID{3}) // suspended
		require.Contains(t, invalid, storj.NodeID{4}) // offline
		require.Contains(t, invalid, storj.NodeID{5}) // gracefully exited
		require.Contains(t, invalid, storj.NodeID{6}) // not in db
		require.Len(t, invalid, 5)
	}

	{ // TestUpdateOperator
		nodeID := storj.NodeID{10}
		addr := "127.0.1.0:8080"
		lastNet := "127.0.1"
		d := overlay.NodeCheckInInfo{
			NodeID:     nodeID,
			Address:    &pb.NodeAddress{Address: addr, Transport: pb.NodeTransport_TCP_TLS_GRPC},
			LastIPPort: addr,
			LastNet:    lastNet,
			Version:    &pb.NodeVersion{Version: "v1.0.0"},
			Capacity:   &pb.NodeCapacity{},
		}
		err := cache.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)

		update, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: "0x1111111111111111111111111111111111111111",
				Email:  "abc123@mail.test",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, update)
		require.Equal(t, "0x1111111111111111111111111111111111111111", update.Operator.Wallet)
		require.Equal(t, "abc123@mail.test", update.Operator.Email)

		found, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, "0x1111111111111111111111111111111111111111", found.Operator.Wallet)
		require.Equal(t, "abc123@mail.test", found.Operator.Email)

		updateEmail, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: update.Operator.Wallet,
				Email:  "def456@mail.test",
			},
		})

		require.NoError(t, err)
		assert.NotNil(t, updateEmail)
		assert.Equal(t, "0x1111111111111111111111111111111111111111", updateEmail.Operator.Wallet)
		assert.Equal(t, "def456@mail.test", updateEmail.Operator.Email)

		updateWallet, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: "0x2222222222222222222222222222222222222222",
				Email:  updateEmail.Operator.Email,
			},
		})

		require.NoError(t, err)
		assert.NotNil(t, updateWallet)
		assert.Equal(t, "0x2222222222222222222222222222222222222222", updateWallet.Operator.Wallet)
		assert.Equal(t, "def456@mail.test", updateWallet.Operator.Email)
	}

	{ // TestUpdateExists
		nodeID := storj.NodeID{1}
		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		auditAlpha := node.Reputation.AuditReputationAlpha
		auditBeta := node.Reputation.AuditReputationBeta

		updateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditSuccess,
			IsUp:         true,
			AuditLambda:  0.123, AuditWeight: 0.456,
			AuditDQ: 0, // don't disqualify for any reason
		}
		stats, err := cache.UpdateStats(ctx, updateReq)
		require.NoError(t, err)

		expectedAuditAlpha := updateReq.AuditLambda*auditAlpha + updateReq.AuditWeight
		expectedAuditBeta := updateReq.AuditLambda * auditBeta
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)

		auditAlpha = expectedAuditAlpha
		auditBeta = expectedAuditBeta

		updateReq.AuditOutcome = overlay.AuditFailure
		updateReq.IsUp = false
		stats, err = cache.UpdateStats(ctx, updateReq)
		require.NoError(t, err)

		expectedAuditAlpha = updateReq.AuditLambda * auditAlpha
		expectedAuditBeta = updateReq.AuditLambda*auditBeta + updateReq.AuditWeight
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)

	}

	{ // test UpdateCheckIn updates the reputation correctly when the node is offline/online
		nodeID := storj.NodeID{1}

		// get the existing node info that is stored in nodes table
		_, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		info := overlay.NodeCheckInInfo{
			NodeID: nodeID,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			IsUp: false,
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}
		// update check-in when node is offline
		err = cache.UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)
		_, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)

		info.IsUp = true
		// update check-in when node is online
		err = cache.UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)
		_, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)

	}
}
