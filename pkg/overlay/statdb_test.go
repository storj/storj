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

		err := cache.Update(ctx, &pb.Node{Id: nodeID})
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

	{ // TestFindInvalidNodes
		for _, tt := range []struct {
			nodeID             storj.NodeID
			auditSuccessCount  int64
			auditCount         int64
			auditSuccessRatio  float64
			uptimeSuccessCount int64
			uptimeCount        int64
			uptimeRatio        float64
		}{
			{storj.NodeID{1}, 20, 20, 1, 20, 20, 1},   // good audit success
			{storj.NodeID{2}, 5, 20, 0.25, 20, 20, 1}, // bad audit success, good uptime
			{storj.NodeID{3}, 20, 20, 1, 5, 20, 0.25}, // good audit success, bad uptime
			{storj.NodeID{4}, 0, 0, 0, 20, 20, 1},     // "bad" audit success, no audits
			{storj.NodeID{5}, 20, 20, 1, 0, 0, 0.25},  // "bad" uptime success, no checks
			{storj.NodeID{6}, 0, 1, 0, 5, 5, 1},       // bad audit success exactly one audit
			{storj.NodeID{7}, 0, 20, 0, 20, 20, 1},    // bad ratios, excluded from query
		} {
			nodeStats := &overlay.NodeStats{
				AuditSuccessRatio:  tt.auditSuccessRatio,
				UptimeRatio:        tt.uptimeRatio,
				AuditCount:         tt.auditCount,
				AuditSuccessCount:  tt.auditSuccessCount,
				UptimeCount:        tt.uptimeCount,
				UptimeSuccessCount: tt.uptimeSuccessCount,
			}

			err := cache.Update(ctx, &pb.Node{Id: tt.nodeID})
			require.NoError(t, err)

			_, err = cache.CreateStats(ctx, tt.nodeID, nodeStats)
			require.NoError(t, err)
		}

		nodeIds := storj.NodeIDList{
			storj.NodeID{1}, storj.NodeID{2},
			storj.NodeID{3}, storj.NodeID{4},
			storj.NodeID{5}, storj.NodeID{6},
		}
		maxStats := &overlay.NodeStats{
			AuditSuccessRatio: 0.5,
			UptimeRatio:       0.5,
		}

		invalid, err := cache.FindInvalidNodes(ctx, nodeIds, maxStats)
		require.NoError(t, err)

		assert.Contains(t, invalid, storj.NodeID{2})
		assert.Contains(t, invalid, storj.NodeID{3})
		assert.Contains(t, invalid, storj.NodeID{6})
		assert.Len(t, invalid, 3)
	}

	{ // TestUpdateOperator
		nodeID := storj.NodeID{10}
		err := cache.Update(ctx, &pb.Node{Id: nodeID})
		require.NoError(t, err)

		update, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Wallet: "0x1111111111111111111111111111111111111111",
			Email:  "abc123@gmail.com",
		})

		require.NoError(t, err)
		assert.NotNil(t, update)

		found, err := cache.Get(ctx, nodeID)
		assert.NotNil(t, found)
		require.NoError(t, err)

		assert.Equal(t, "0x1111111111111111111111111111111111111111", update.Operator.Wallet)
		assert.Equal(t, "abc123@gmail.com", update.Operator.Email)

		updateEmail, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Wallet: update.Operator.Wallet,
			Email:  "def456@gmail.com",
		})

		require.NoError(t, err)
		assert.NotNil(t, updateEmail)
		assert.Equal(t, "0x1111111111111111111111111111111111111111", updateEmail.Operator.Wallet)
		assert.Equal(t, "def456@gmail.com", updateEmail.Operator.Email)

		updateWallet, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Wallet: "0x2222222222222222222222222222222222222222",
			Email:  updateEmail.Operator.Email,
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
