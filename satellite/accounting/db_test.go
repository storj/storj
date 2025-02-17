// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSaveBucketTallies(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		// Setup: create bucket storage tallies
		projectID := testrand.UUID()

		bucketTallies, expectedTallies, err := createBucketStorageTallies(projectID)
		require.NoError(t, err)

		// Execute test:  retrieve the save tallies and confirm they contains the expected data
		intervalStart := time.Now()
		pdb := db.ProjectAccounting()

		err = pdb.SaveTallies(ctx, intervalStart, bucketTallies)
		require.NoError(t, err)

		tallies, err := pdb.GetTallies(ctx)
		require.NoError(t, err)
		for _, tally := range tallies {
			require.Contains(t, expectedTallies, tally)
		}
	})
}

func TestStorageNodeUsage(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		const days = 30

		now := time.Now().UTC()

		nodeID := testrand.NodeID()
		startDate := now.Add(time.Hour * 24 * -days)

		var nodes storj.NodeIDList
		nodes = append(nodes, nodeID, testrand.NodeID(), testrand.NodeID(), testrand.NodeID())

		rollups, tallies, lastDate := makeRollupsAndStorageNodeStorageTallies(nodes, startDate, days)

		lastRollup := rollups[lastDate]

		accountingDB := db.StoragenodeAccounting()

		// create last rollup timestamp
		_, err := accountingDB.LastTimestamp(ctx, accounting.LastRollup)
		require.NoError(t, err)

		// save tallies
		for latestTally, tallies := range tallies {
			err = accountingDB.SaveTallies(ctx, latestTally, tallies.nodeIDs, tallies.totals)
			require.NoError(t, err)
		}

		// save rollup
		err = accountingDB.SaveRollup(ctx, lastDate.Add(time.Hour*-24), rollups)
		require.NoError(t, err)

		t.Run("usage with pending tallies", func(t *testing.T) {
			nodeStorageUsages, err := accountingDB.QueryStorageNodeUsage(ctx, nodeID, time.Time{}, now)
			require.NoError(t, err)
			assert.NotNil(t, nodeStorageUsages)
			assert.Equal(t, days, len(nodeStorageUsages))

			// check usage from rollups
			for _, usage := range nodeStorageUsages[:len(nodeStorageUsages)-1] {
				assert.Equal(t, nodeID, usage.NodeID)
				dateRollup, ok := rollups[usage.Timestamp.UTC()]
				if assert.True(t, ok) {
					nodeRollup, ok := dateRollup[nodeID]
					if assert.True(t, ok) {
						assert.Equal(t, nodeRollup.AtRestTotal, usage.StorageUsed)
					}
				}
			}

			// check last usage that calculated from tallies
			lastUsage := nodeStorageUsages[len(nodeStorageUsages)-1]

			assert.Equal(t, nodeID, lastUsage.NodeID)
			assert.Equal(t, lastRollup[nodeID].StartTime, lastUsage.Timestamp.UTC())
			assert.Equal(t, lastRollup[nodeID].AtRestTotal, lastUsage.StorageUsed)
		})

		t.Run("usage entirely from rollups", func(t *testing.T) {
			const (
				start = 10
				// should be greater than 2
				// not to include tallies into result
				end = 2
			)

			startDate, endDate := now.Add(time.Hour*24*-start), now.Add(time.Hour*24*-end)

			nodeStorageUsages, err := accountingDB.QueryStorageNodeUsage(ctx, nodeID, startDate, endDate)
			require.NoError(t, err)
			assert.NotNil(t, nodeStorageUsages)
			assert.Equal(t, start-end, len(nodeStorageUsages))

			for _, usage := range nodeStorageUsages {
				assert.Equal(t, nodeID, usage.NodeID)
				dateRollup, ok := rollups[usage.Timestamp.UTC()]
				if assert.True(t, ok) {
					nodeRollup, ok := dateRollup[nodeID]
					if assert.True(t, ok) {
						assert.Equal(t, nodeRollup.AtRestTotal, usage.StorageUsed)
					}
				}
			}
		})
	})
}

// There can be more than one rollup in a day. Test that the sums are grouped by day.
func TestStorageNodeUsage_TwoRollupsInADay(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now().UTC()

		nodeID := testrand.NodeID()

		accountingDB := db.StoragenodeAccounting()

		// create last rollup timestamp
		_, err := accountingDB.LastTimestamp(ctx, accounting.LastRollup)
		require.NoError(t, err)

		t1 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		t2 := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
		rollups := make(accounting.RollupStats)

		rollups[t1] = make(map[storj.NodeID]*accounting.Rollup)
		rollups[t2] = make(map[storj.NodeID]*accounting.Rollup)

		rollups[t1][nodeID] = &accounting.Rollup{
			NodeID:          nodeID,
			AtRestTotal:     1000,
			StartTime:       t1,
			IntervalEndTime: t1,
		}
		rollups[t2][nodeID] = &accounting.Rollup{
			NodeID:          nodeID,
			AtRestTotal:     500,
			StartTime:       t2,
			IntervalEndTime: t2,
		}
		// save rollup
		err = accountingDB.SaveRollup(ctx, now.Add(time.Hour*-24), rollups)
		require.NoError(t, err)

		nodeStorageUsages, err := accountingDB.QueryStorageNodeUsage(ctx, nodeID, t1.Add(-24*time.Hour), t2.Add(24*time.Hour))
		require.NoError(t, err)
		require.NotNil(t, nodeStorageUsages)
		require.Equal(t, 1, len(nodeStorageUsages))

		require.Equal(t, nodeID, nodeStorageUsages[0].NodeID)
		require.EqualValues(t, 1500, nodeStorageUsages[0].StorageUsed)
	})
}

func TestProjectLimits(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "test", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		err = db.ProjectAccounting().UpdateProjectUsageLimit(ctx, proj.ID, 1)
		require.NoError(t, err)
		err = db.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, proj.ID, 2)

		t.Run("get", func(t *testing.T) {
			storageLimit, err := db.ProjectAccounting().GetProjectStorageLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(1).Int64(), *storageLimit)

			bandwidthLimit, err := db.ProjectAccounting().GetProjectBandwidthLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(2).Int64(), *bandwidthLimit)
		})

		t.Run("update", func(t *testing.T) {
			err = db.ProjectAccounting().UpdateProjectUsageLimit(ctx, proj.ID, 4)
			require.NoError(t, err)
			err = db.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, proj.ID, 3)

			storageLimit, err := db.ProjectAccounting().GetProjectStorageLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(4).Int64(), *storageLimit)

			bandwidthLimit, err := db.ProjectAccounting().GetProjectBandwidthLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(3).Int64(), *bandwidthLimit)
		})
	})
}

func createBucketStorageTallies(projectID uuid.UUID) (map[metabase.BucketLocation]*accounting.BucketTally, []accounting.BucketTally, error) {
	bucketTallies := make(map[metabase.BucketLocation]*accounting.BucketTally)
	var expectedTallies []accounting.BucketTally

	for i := 0; i < 4; i++ {
		bucketName := metabase.BucketName(fmt.Sprintf("%s%d", "testbucket", i))
		bucketLocation := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
		}

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		tally := accounting.BucketTally{
			BucketLocation: metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: bucketName,
			},
			ObjectCount:   int64(1),
			TotalSegments: int64(2),
			TotalBytes:    int64(2),
			MetadataSize:  int64(1),
		}
		bucketTallies[bucketLocation] = &tally
		expectedTallies = append(expectedTallies, tally)

	}
	return bucketTallies, expectedTallies, nil
}

type nodesAndTallies struct {
	nodeIDs []storj.NodeID
	totals  []float64
}

// make rollups and tallies for specified nodes and date range.
func makeRollupsAndStorageNodeStorageTallies(nodes []storj.NodeID, start time.Time, days int) (accounting.RollupStats, map[time.Time]nodesAndTallies, time.Time) {
	rollups := make(accounting.RollupStats)
	tallies := make(map[time.Time]nodesAndTallies)

	const (
		hours = 12
	)

	for i := 0; i < days; i++ {
		startDay := time.Date(start.Year(), start.Month(), start.Day()+i, 0, 0, 0, 0, start.Location())
		if rollups[startDay] == nil {
			rollups[startDay] = make(map[storj.NodeID]*accounting.Rollup)
		}

		for h := 0; h < hours; h++ {
			intervalEndTime := startDay.Add(time.Hour * time.Duration(h))
			tallies[intervalEndTime] = nodesAndTallies{
				nodeIDs: make([]storj.NodeID, len(nodes)),
				totals:  make([]float64, len(nodes)),
			}
		}

		for nodeIndex, node := range nodes {
			rollup := &accounting.Rollup{
				NodeID:    node,
				StartTime: startDay,
			}

			for h := 0; h < hours; h++ {
				intervalEndTime := startDay.Add(time.Hour * time.Duration(h))

				tallieAtRest := math.Round(testrand.Float64n(1000))
				tallies[intervalEndTime].nodeIDs[nodeIndex] = node
				tallies[intervalEndTime].totals[nodeIndex] = tallieAtRest
				rollup.AtRestTotal += tallieAtRest
				rollup.IntervalEndTime = intervalEndTime
			}

			rollups[startDay][node] = rollup
		}
	}

	return rollups, tallies, time.Date(start.Year(), start.Month(), start.Day()+days-1, 0, 0, 0, 0, start.Location())
}
