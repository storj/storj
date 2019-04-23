// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func getRatio(success, total int64) (ratio float64) {
	ratio = float64(success) / float64(total)
	return ratio
}

func TestStatDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.OverlayCache())
	})
}

func testDatabase(ctx context.Context, t *testing.T, cache overlay.DB) {
	nodeID := storj.NodeID{1, 2, 3, 4, 5}
	currAuditSuccess := int64(4)
	currAuditCount := int64(10)
	currUptimeSuccess := int64(8)
	currUptimeCount := int64(25)

	{ // TestCreateNewAndWithStats
		auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		nodeStats := &overlay.NodeStats{
			AuditSuccessRatio:  auditSuccessRatio,
			UptimeRatio:        uptimeRatio,
			AuditCount:         currAuditCount,
			AuditSuccessCount:  currAuditSuccess,
			UptimeCount:        currUptimeCount,
			UptimeSuccessCount: currUptimeSuccess,
		}

		err := cache.UpdateAddress(ctx, &pb.Node{Id: nodeID})
		require.NoError(t, err)

		stats, err := cache.CreateStats(ctx, nodeID, nodeStats)
		require.NoError(t, err)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, currAuditCount, node.Reputation.AuditCount)
		assert.EqualValues(t, currAuditSuccess, node.Reputation.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, node.Reputation.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, node.Reputation.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, node.Reputation.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, node.Reputation.UptimeRatio)
	}

	{ // TestGetDoesNotExist
		noNodeID := storj.NodeID{255, 255, 255, 255}

		_, err := cache.Get(ctx, noNodeID)
		assert.Error(t, err)
	}

	{ // TestUpdateOperator
		nodeID := storj.NodeID{10}
		err := cache.UpdateAddress(ctx, &pb.Node{Id: nodeID})
		require.NoError(t, err)

		update, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: "0x1111111111111111111111111111111111111111",
				Email:  "abc123@gmail.com",
			},
		})

		require.NoError(t, err)
		assert.NotNil(t, update)

		found, err := cache.Get(ctx, nodeID)
		assert.NotNil(t, found)
		require.NoError(t, err)

		assert.Equal(t, "0x1111111111111111111111111111111111111111", update.Operator.Wallet)
		assert.Equal(t, "abc123@gmail.com", update.Operator.Email)

		updateEmail, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: update.Operator.Wallet,
				Email:  "def456@gmail.com",
			},
		})

		require.NoError(t, err)
		assert.NotNil(t, updateEmail)
		assert.Equal(t, "0x1111111111111111111111111111111111111111", updateEmail.Operator.Wallet)
		assert.Equal(t, "def456@gmail.com", updateEmail.Operator.Email)

		updateWallet, err := cache.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
			Operator: &pb.NodeOperator{
				Wallet: "0x2222222222222222222222222222222222222222",
				Email:  updateEmail.Operator.Email,
			},
		})

		require.NoError(t, err)
		assert.NotNil(t, updateWallet)
		assert.Equal(t, "0x2222222222222222222222222222222222222222", updateWallet.Operator.Wallet)
		assert.Equal(t, "def456@gmail.com", updateWallet.Operator.Email)
	}

	{ // TestUpdateExists
		auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, currAuditCount, node.Reputation.AuditCount)
		assert.EqualValues(t, currAuditSuccess, node.Reputation.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, node.Reputation.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, node.Reputation.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, node.Reputation.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, node.Reputation.UptimeRatio)

		updateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditSuccess: true,
			IsUp:         false,
		}
		stats, err := cache.UpdateStats(ctx, updateReq)
		require.NoError(t, err)

		currAuditSuccess++
		currAuditCount++
		currUptimeCount++
		newAuditRatio := getRatio(currAuditSuccess, currAuditCount)
		newUptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		assert.EqualValues(t, newAuditRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio, stats.UptimeRatio)
	}

	{ // TestUpdateUptimeExists
		auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, currAuditCount, node.Reputation.AuditCount)
		assert.EqualValues(t, currAuditSuccess, node.Reputation.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, node.Reputation.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, node.Reputation.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, node.Reputation.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, node.Reputation.UptimeRatio)

		stats, err := cache.UpdateUptime(ctx, nodeID, false)
		require.NoError(t, err)

		currUptimeCount++
		newUptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		assert.EqualValues(t, newUptimeRatio, stats.UptimeRatio)
	}

	{ // TestUpdateStatsExists
		auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		node, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, currAuditCount, node.Reputation.AuditCount)
		assert.EqualValues(t, currAuditSuccess, node.Reputation.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, node.Reputation.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, node.Reputation.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, node.Reputation.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, node.Reputation.UptimeRatio)

		stats, err := cache.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		})
		require.NoError(t, err)

		currAuditCount++
		newAuditRatio := getRatio(stats.AuditSuccessCount, stats.AuditCount)
		assert.EqualValues(t, newAuditRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		newUptimeRatio := getRatio(stats.UptimeSuccessCount, stats.UptimeCount)
		assert.EqualValues(t, newUptimeRatio, stats.UptimeRatio)
	}
}
