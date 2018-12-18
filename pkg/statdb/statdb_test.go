// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func getRatio(s, t int64) (success, total int64, ratio float64) {
	ratio = float64(s) / float64(t)
	return s, t, ratio
}

func TestStatdb(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db *satellitedb.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.StatDB())
	})
}

func testDatabase(ctx context.Context, t *testing.T, sdb statdb.DB) {
	nodeID := storj.NodeID{1, 2, 3, 4, 5}
	currAuditSuccess := int64(4)
	currAuditCount := int64(10)
	currUptimeSuccess := int64(8)
	currUptimeCount := int64(25)

	{ // TestCreateNewAndWithStats
		auditSuccessCount, auditCount, auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeSuccessCount, uptimeCount, uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		nodeStats := &pb.NodeStats{
			AuditSuccessRatio:  auditSuccessRatio,
			UptimeRatio:        uptimeRatio,
			AuditCount:         auditCount,
			AuditSuccessCount:  auditSuccessCount,
			UptimeCount:        uptimeCount,
			UptimeSuccessCount: uptimeSuccessCount,
		}

		s, err := sdb.Create(ctx, nodeID, nodeStats)
		assert.NoError(t, err)
		assert.EqualValues(t, auditSuccessRatio, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio, s.UptimeRatio)

		s, err = sdb.Get(ctx, nodeID)
		assert.NoError(t, err)

		assert.EqualValues(t, nodeID, s.NodeId)
		assert.EqualValues(t, auditCount, s.AuditCount)
		assert.EqualValues(t, auditSuccessCount, s.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeCount, s.UptimeCount)
		assert.EqualValues(t, uptimeSuccessCount, s.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, s.UptimeRatio)
	}

	{ // TestCreateExists
		auditSuccessCount, auditCount, auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeSuccessCount, uptimeCount, uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		nodeStats := &pb.NodeStats{
			AuditSuccessRatio:  auditSuccessRatio,
			UptimeRatio:        uptimeRatio,
			AuditCount:         auditCount,
			AuditSuccessCount:  auditSuccessCount,
			UptimeCount:        uptimeCount,
			UptimeSuccessCount: uptimeSuccessCount,
		}
		_, err := sdb.Create(ctx, nodeID, nodeStats)
		assert.Error(t, err)
	}

	{ // TestGetDoesNotExist
		noNodeID := storj.NodeID{255, 255, 255, 255}

		_, err := sdb.Get(ctx, noNodeID)
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
			nodeStats := &pb.NodeStats{
				AuditSuccessRatio:  tt.auditSuccessRatio,
				UptimeRatio:        tt.uptimeRatio,
				AuditCount:         tt.auditCount,
				AuditSuccessCount:  tt.auditSuccessCount,
				UptimeCount:        tt.uptimeCount,
				UptimeSuccessCount: tt.uptimeSuccessCount,
			}

			_, err := sdb.Create(ctx, tt.nodeID, nodeStats)
			assert.NoError(t, err)
		}

		nodeIds := storj.NodeIDList{
			storj.NodeID{1}, storj.NodeID{2},
			storj.NodeID{3}, storj.NodeID{4},
			storj.NodeID{5}, storj.NodeID{6},
		}
		maxStats := &pb.NodeStats{
			AuditSuccessRatio: 0.5,
			UptimeRatio:       0.5,
		}

		invalid, err := sdb.FindInvalidNodes(ctx, nodeIds, maxStats)
		assert.NoError(t, err)

		assert.Contains(t, invalid, storj.NodeID{2})
		assert.Contains(t, invalid, storj.NodeID{3})
		assert.Contains(t, invalid, storj.NodeID{6})
		assert.Len(t, invalid, 3)
	}

	{ // TestUpdateExists
		auditSuccessCount, auditCount, auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeSuccessCount, uptimeCount, uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		s, err := sdb.Get(ctx, nodeID)
		assert.NoError(t, err)

		assert.EqualValues(t, nodeID, s.NodeId)
		assert.EqualValues(t, auditCount, s.AuditCount)
		assert.EqualValues(t, auditSuccessCount, s.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeCount, s.UptimeCount)
		assert.EqualValues(t, uptimeSuccessCount, s.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, s.UptimeRatio)

		updateReq := &statdb.UpdateRequest{
			NodeID:       nodeID,
			AuditSuccess: true,
			IsUp:         false,
		}
		stats, err := sdb.Update(ctx, updateReq)
		assert.NoError(t, err)

		currAuditSuccess = auditSuccessCount + 1
		currAuditCount = auditCount + 1
		currUptimeSuccess = uptimeSuccessCount
		currUptimeCount = uptimeCount + 1
		_, _, newAuditRatio := getRatio(currAuditSuccess, currAuditCount)
		_, _, newUptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		assert.EqualValues(t, newAuditRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio, stats.UptimeRatio)
	}

	{ // TestUpdateUptimeExists
		auditSuccessCount, auditCount, auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeSuccessCount, uptimeCount, uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		s, err := sdb.Get(ctx, nodeID)
		assert.NoError(t, err)

		assert.EqualValues(t, nodeID, s.NodeId)
		assert.EqualValues(t, auditCount, s.AuditCount)
		assert.EqualValues(t, auditSuccessCount, s.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeCount, s.UptimeCount)
		assert.EqualValues(t, uptimeSuccessCount, s.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, s.UptimeRatio)

		stats, err := sdb.UpdateUptime(ctx, nodeID, false)
		assert.NoError(t, err)

		currUptimeSuccess = uptimeSuccessCount
		currUptimeCount = uptimeCount + 1
		_, _, newUptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, auditCount, stats.AuditCount)
		assert.EqualValues(t, newUptimeRatio, stats.UptimeRatio)
	}

	{ // TestUpdateAuditSuccessExists
		auditSuccessCount, auditCount, auditSuccessRatio := getRatio(currAuditSuccess, currAuditCount)
		uptimeSuccessCount, uptimeCount, uptimeRatio := getRatio(currUptimeSuccess, currUptimeCount)

		stats, err := sdb.Get(ctx, nodeID)
		assert.NoError(t, err)

		assert.EqualValues(t, nodeID, stats.NodeId)
		assert.EqualValues(t, auditCount, stats.AuditCount)
		assert.EqualValues(t, auditSuccessCount, stats.AuditSuccessCount)
		assert.EqualValues(t, auditSuccessRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, uptimeCount, stats.UptimeCount)
		assert.EqualValues(t, uptimeSuccessCount, stats.UptimeSuccessCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)

		stats, err = sdb.UpdateAuditSuccess(ctx, nodeID, false)
		assert.NoError(t, err)

		currAuditSuccess = auditSuccessCount
		currAuditCount = auditCount + 1
		_, _, newAuditRatio := getRatio(currAuditSuccess, currAuditCount)
		assert.EqualValues(t, newAuditRatio, stats.AuditSuccessRatio)
		assert.EqualValues(t, auditCount+1, stats.AuditCount)
		assert.EqualValues(t, uptimeRatio, stats.UptimeRatio)
	}

	{ // TestUpdateBatchExists
		nodeID1 := storj.NodeID{255, 1}
		nodeID2 := storj.NodeID{255, 2}

		auditSuccessCount1, auditCount1, auditRatio1 := getRatio(4, 10)
		uptimeSuccessCount1, uptimeCount1, uptimeRatio1 := getRatio(8, 25)
		nodeStats := &pb.NodeStats{
			AuditSuccessCount:  auditSuccessCount1,
			AuditCount:         auditCount1,
			AuditSuccessRatio:  auditRatio1,
			UptimeSuccessCount: uptimeSuccessCount1,
			UptimeCount:        uptimeCount1,
			UptimeRatio:        uptimeRatio1,
		}

		s, err := sdb.Create(ctx, nodeID1, nodeStats)
		assert.NoError(t, err)
		assert.EqualValues(t, auditRatio1, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio1, s.UptimeRatio)

		auditSuccessCount2, auditCount2, auditRatio2 := getRatio(7, 10)
		uptimeSuccessCount2, uptimeCount2, uptimeRatio2 := getRatio(8, 20)
		nodeStats = &pb.NodeStats{
			AuditSuccessCount:  auditSuccessCount2,
			AuditCount:         auditCount2,
			AuditSuccessRatio:  auditRatio2,
			UptimeSuccessCount: uptimeSuccessCount2,
			UptimeCount:        uptimeCount2,
			UptimeRatio:        uptimeRatio2,
		}

		s, err = sdb.Create(ctx, nodeID2, nodeStats)
		assert.NoError(t, err)
		assert.EqualValues(t, auditRatio2, s.AuditSuccessRatio)
		assert.EqualValues(t, uptimeRatio2, s.UptimeRatio)

		updateReqList := []*statdb.UpdateRequest{
			&statdb.UpdateRequest{
				NodeID:       nodeID1,
				AuditSuccess: true,
				IsUp:         false,
			},
			&statdb.UpdateRequest{
				NodeID:       nodeID2,
				AuditSuccess: true,
				IsUp:         true,
			},
		}
		statsList, _, err := sdb.UpdateBatch(ctx, updateReqList)
		assert.NoError(t, err)

		_, _, newAuditRatio1 := getRatio(auditSuccessCount1+1, auditCount1+1)
		_, _, newUptimeRatio1 := getRatio(uptimeSuccessCount1, uptimeCount1+1)
		_, _, newAuditRatio2 := getRatio(auditSuccessCount2+1, auditCount2+1)
		_, _, newUptimeRatio2 := getRatio(uptimeSuccessCount2+1, uptimeCount2+1)
		stats1 := statsList[0]
		stats2 := statsList[1]
		assert.EqualValues(t, newAuditRatio1, stats1.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio1, stats1.UptimeRatio)
		assert.EqualValues(t, newAuditRatio2, stats2.AuditSuccessRatio)
		assert.EqualValues(t, newUptimeRatio2, stats2.UptimeRatio)
	}
}
