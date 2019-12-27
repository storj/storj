// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
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
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.OverlayCache())
	})
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, overlay.NewCombinedCache(db.OverlayCache()))
	})
}

func testDatabase(ctx context.Context, t *testing.T, cache overlay.DB) {
	{ // TestKnownUnreliableOrOffline
		for _, tt := range []struct {
			nodeID      storj.NodeID
			auditAlpha  float64
			auditBeta   float64
			uptimeAlpha float64
			uptimeBeta  float64
		}{
			{storj.NodeID{1}, 20, 0, 20, 0}, // good reputations => good
			{storj.NodeID{2}, 0, 20, 20, 0}, // bad audit rep, good uptime rep => bad
			{storj.NodeID{3}, 20, 0, 0, 20}, // good audit rep, bad uptime rep => bad
		} {
			startingRep := overlay.NodeSelectionConfig{
				AuditReputationAlpha0:  tt.auditAlpha,
				AuditReputationBeta0:   tt.auditBeta,
				UptimeReputationAlpha0: tt.uptimeAlpha,
				UptimeReputationBeta0:  tt.uptimeBeta,
			}

			err := cache.UpdateAddress(ctx, &pb.Node{Id: tt.nodeID}, startingRep)
			require.NoError(t, err)

			// update stats so node disqualification is triggered
			_, err = cache.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       tt.nodeID,
				AuditSuccess: true,
				IsUp:         true,
				AuditLambda:  1, AuditWeight: 1,
				UptimeLambda: 1, UptimeWeight: 1,
				AuditDQ: 0.9, UptimeDQ: 0.9,
			})
			require.NoError(t, err)
		}

		nodeIds := storj.NodeIDList{
			storj.NodeID{1}, storj.NodeID{2},
			storj.NodeID{3}, storj.NodeID{4},
		}
		criteria := &overlay.NodeCriteria{OnlineWindow: time.Hour}

		invalid, err := cache.KnownUnreliableOrOffline(ctx, criteria, nodeIds)
		require.NoError(t, err)

		require.Contains(t, invalid, storj.NodeID{2}) // bad audit
		require.Contains(t, invalid, storj.NodeID{3}) // bad uptime
		require.Contains(t, invalid, storj.NodeID{4}) // not in db
		require.Len(t, invalid, 3)
	}

	{ // TestUpdateOperator
		nodeID := storj.NodeID{10}
		err := cache.UpdateAddress(ctx, &pb.Node{Id: nodeID}, overlay.NodeSelectionConfig{})
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
		uptimeAlpha := node.Reputation.UptimeReputationAlpha
		uptimeBeta := node.Reputation.UptimeReputationBeta

		updateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditSuccess: true,
			IsUp:         true,
			AuditLambda:  0.123, AuditWeight: 0.456,
			UptimeLambda: 0.789, UptimeWeight: 0.876,
			AuditDQ: 0, UptimeDQ: 0, // don't disqualify for any reason
		}
		stats, err := cache.UpdateStats(ctx, updateReq)
		require.NoError(t, err)

		expectedAuditAlpha := updateReq.AuditLambda*auditAlpha + updateReq.AuditWeight
		expectedAuditBeta := updateReq.AuditLambda * auditBeta
		expectedUptimeAlpha := updateReq.UptimeLambda*uptimeAlpha + updateReq.UptimeWeight
		expectedUptimeBeta := updateReq.UptimeLambda * uptimeBeta
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)
		require.EqualValues(t, stats.UptimeReputationAlpha, expectedUptimeAlpha)
		require.EqualValues(t, stats.UptimeReputationBeta, expectedUptimeBeta)

		auditAlpha = expectedAuditAlpha
		auditBeta = expectedAuditBeta
		uptimeAlpha = expectedUptimeAlpha
		uptimeBeta = expectedUptimeBeta

		updateReq.AuditSuccess = false
		updateReq.IsUp = false
		stats, err = cache.UpdateStats(ctx, updateReq)
		require.NoError(t, err)

		expectedAuditAlpha = updateReq.AuditLambda * auditAlpha
		expectedAuditBeta = updateReq.AuditLambda*auditBeta + updateReq.AuditWeight
		expectedUptimeAlpha = updateReq.UptimeLambda * uptimeAlpha
		expectedUptimeBeta = updateReq.UptimeLambda*uptimeBeta + updateReq.UptimeWeight
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)
		require.EqualValues(t, stats.UptimeReputationAlpha, expectedUptimeAlpha)
		require.EqualValues(t, stats.UptimeReputationBeta, expectedUptimeBeta)

	}

	{ // test UpdateCheckIn updates the reputation correctly when the node is offline/online
		nodeID := storj.NodeID{1}

		// get the existing node info that is stored in nodes table
		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		alpha := node.Reputation.UptimeReputationAlpha
		beta := node.Reputation.UptimeReputationBeta

		lambda := 0.789
		weight := 0.876
		dq := float64(0) // don't disqualify for any reason

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
		config := overlay.NodeSelectionConfig{
			UptimeReputationLambda: lambda,
			UptimeReputationWeight: weight,
			UptimeReputationDQ:     dq,
		}
		// update check-in when node is offline
		err = cache.UpdateCheckIn(ctx, info, time.Now().UTC(), config)
		require.NoError(t, err)
		node, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)

		expectedAlpha := lambda * alpha
		expectedBeta := lambda*beta + weight
		// confirm the reputation is updated correctly when node is offline
		require.EqualValues(t, node.Reputation.UptimeReputationAlpha, expectedAlpha)
		require.EqualValues(t, node.Reputation.UptimeReputationBeta, expectedBeta)

		alpha = expectedAlpha
		beta = expectedBeta

		info.IsUp = true
		// update check-in when node is online
		err = cache.UpdateCheckIn(ctx, info, time.Now().UTC(), config)
		require.NoError(t, err)
		node, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)

		expectedAlpha = lambda*alpha + weight
		expectedBeta = lambda * beta
		// confirm the reputation is updated correctly when node is online
		require.EqualValues(t, node.Reputation.UptimeReputationAlpha, expectedAlpha)
		require.EqualValues(t, node.Reputation.UptimeReputationBeta, expectedBeta)
	}
}
