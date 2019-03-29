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

		_, err := cache.CreateEntryIfNotExists(ctx, &pb.Node{Id: nodeID})
		require.NoError(t, err)

		stats, err := cache.CreateStats(ctx, nodeID, nodeStats)
		require.NoError(t, err)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		stats, err = cache.GetStats(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, nodeID, stats.NodeID)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		assert.EqualValues(t, currAuditSuccess, stats.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, stats.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, stats.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)
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

			_, err := cache.CreateEntryIfNotExists(ctx, &pb.Node{Id: tt.nodeID})
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
		stats, err := cache.CreateEntryIfNotExists(ctx, &pb.Node{Id: nodeID})

		require.NoError(t, err)

		assert.Equal(t, stats.Operator.Wallet, "")
		assert.Equal(t, stats.Operator.Email, "")

		update, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Wallet: "0x1111111111111111111111111111111111111111",
			Email:  "abc123@gmail.com",
		})

		require.NoError(t, err)
		assert.NotNil(t, update)

		found, err := cache.GetStats(ctx, nodeID)
		assert.NotNil(t, found)
		require.NoError(t, err)

		assert.Equal(t, "0x1111111111111111111111111111111111111111", found.Operator.Wallet)
		assert.Equal(t, "abc123@gmail.com", found.Operator.Email)

		updateEmail, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Email: "def456@gmail.com",
		})

		require.NoError(t, err)
		assert.NotNil(t, updateEmail)
		assert.Equal(t, updateEmail.Operator.Email, "def456@gmail.com")

		updateWallet, err := cache.UpdateOperator(ctx, nodeID, pb.NodeOperator{
			Wallet: "0x2222222222222222222222222222222222222222",
		})

		require.NoError(t, err)
		assert.NotNil(t, updateWallet)
		assert.Equal(t, updateWallet.Operator.Wallet, "0x2222222222222222222222222222222222222222")
	}

	{ // TestUpdateExists
		auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		stats, err := cache.GetStats(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, nodeID, stats.NodeID)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		assert.EqualValues(t, currAuditSuccess, stats.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, stats.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, stats.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		updateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditSuccess: true,
			IsUp:         false,
		}
		stats, err = cache.UpdateStats(ctx, updateReq)
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

		stats, err := cache.GetStats(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, nodeID, stats.NodeID)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		assert.EqualValues(t, currAuditSuccess, stats.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, stats.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, stats.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		stats, err = cache.UpdateUptime(ctx, nodeID, false)
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

		stats, err := cache.GetStats(ctx, nodeID)
		require.NoError(t, err)

		assert.EqualValues(t, nodeID, stats.NodeID)
		assert.EqualValues(t, currAuditCount, stats.AuditCount)
		assert.EqualValues(t, currAuditSuccess, stats.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, currUptimeCount, stats.UptimeCount)
		assert.EqualValues(t, currUptimeSuccess, stats.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		stats, err = cache.UpdateStats(ctx, &overlay.UpdateRequest{
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

	{ // TestUpdateBatchExists
		nodeID1 := storj.NodeID{255, 1}
		nodeID2 := storj.NodeID{255, 2}

		auditSuccessCount1 := int64(4)
		auditCount1 := int64(10)
		auditRatio1 := getRatio(auditSuccessCount1, auditCount1)

		uptimeSuccessCount1 := int64(8)
		uptimeCount1 := int64(25)
		uptimeRatio1 := getRatio(uptimeSuccessCount1, uptimeCount1)

		nodeStats := &overlay.NodeStats{
			AuditSuccessCount:  auditSuccessCount1,
			AuditCount:         auditCount1,
			AuditSuccessRatio:  auditRatio1,
			UptimeSuccessCount: uptimeSuccessCount1,
			UptimeCount:        uptimeCount1,
			UptimeRatio:        uptimeRatio1,
		}

		_, err := cache.CreateEntryIfNotExists(ctx, &pb.Node{Id: nodeID1})
		require.NoError(t, err)

		stats, err := cache.CreateStats(ctx, nodeID1, nodeStats)
		require.NoError(t, err)
		assert.EqualValues(t, auditRatio1, stats.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio1, stats.UptimeRatio)

		auditSuccessCount2 := int64(7)
		auditCount2 := int64(10)
		auditRatio2 := getRatio(auditSuccessCount2, auditCount2)

		uptimeSuccessCount2 := int64(8)
		uptimeCount2 := int64(20)
		uptimeRatio2 := getRatio(uptimeSuccessCount2, uptimeCount2)

		nodeStats = &overlay.NodeStats{
			AuditSuccessCount:  auditSuccessCount2,
			AuditCount:         auditCount2,
			AuditSuccessRatio:  auditRatio2,
			UptimeSuccessCount: uptimeSuccessCount2,
			UptimeCount:        uptimeCount2,
			UptimeRatio:        uptimeRatio2,
		}

		_, err = cache.CreateEntryIfNotExists(ctx, &pb.Node{Id: nodeID2})
		require.NoError(t, err)

		stats, err = cache.CreateStats(ctx, nodeID2, nodeStats)
		require.NoError(t, err)
		assert.EqualValues(t, auditRatio2, stats.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio2, stats.UptimeRatio)

		updateReqList := []*overlay.UpdateRequest{
			&overlay.UpdateRequest{
				NodeID:       nodeID1,
				AuditSuccess: true,
				IsUp:         false,
			},
			&overlay.UpdateRequest{
				NodeID:       nodeID2,
				AuditSuccess: true,
				IsUp:         true,
			},
		}
		statsList, _, err := cache.UpdateBatch(ctx, updateReqList)
		require.NoError(t, err)

		newAuditRatio1 := getRatio(auditSuccessCount1+1, auditCount1+1)
		newUptimeRatio1 := getRatio(uptimeSuccessCount1, uptimeCount1+1)
		newAuditRatio2 := getRatio(auditSuccessCount2+1, auditCount2+1)
		newUptimeRatio2 := getRatio(uptimeSuccessCount2+1, uptimeCount2+1)
		stats1 := statsList[0]
		stats2 := statsList[1]
		assert.EqualValues(t, newAuditRatio1, stats1.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio1, stats1.UptimeRatio)
		assert.EqualValues(t, newAuditRatio2, stats2.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio2, stats2.UptimeRatio)
	}
}
