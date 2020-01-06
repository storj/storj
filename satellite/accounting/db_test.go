// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSaveBucketTallies(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		const days = 30

		now := time.Now().UTC()

		nodeID := testrand.NodeID()
		startDate := now.Add(time.Hour * 24 * -days)

		var nodes storj.NodeIDList
		nodes = append(nodes, nodeID, testrand.NodeID(), testrand.NodeID(), testrand.NodeID())

		rollups, tallies, lastDate := makeRollupsAndStorageNodeStorageTallies(nodes, startDate, days)

		lastRollup := rollups[lastDate]
		delete(rollups, lastDate)

		accountingDB := db.StoragenodeAccounting()

		// create last rollup timestamp
		_, err := accountingDB.LastTimestamp(ctx, accounting.LastRollup)
		require.NoError(t, err)

		// save tallies
		for latestTally, tallies := range tallies {
			err = accountingDB.SaveTallies(ctx, latestTally, tallies)
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

func TestProjectLimits(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		proj, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "test", OwnerID: testrand.UUID()})
		require.NoError(t, err)

		err = db.ProjectAccounting().UpdateProjectUsageLimit(ctx, proj.ID, 1)
		require.NoError(t, err)

		t.Run("get", func(t *testing.T) {
			storageLimit, err := db.ProjectAccounting().GetProjectStorageLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(1), storageLimit)

			bandwidthLimit, err := db.ProjectAccounting().GetProjectBandwidthLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(1), bandwidthLimit)
		})

		t.Run("update", func(t *testing.T) {
			err = db.ProjectAccounting().UpdateProjectUsageLimit(ctx, proj.ID, 4)
			require.NoError(t, err)

			storageLimit, err := db.ProjectAccounting().GetProjectStorageLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(4), storageLimit)

			bandwidthLimit, err := db.ProjectAccounting().GetProjectBandwidthLimit(ctx, proj.ID)
			assert.NoError(t, err)
			assert.Equal(t, memory.Size(4), bandwidthLimit)
		})
	})
}

func createBucketStorageTallies(projectID uuid.UUID) (map[string]*accounting.BucketTally, []accounting.BucketTally, error) {
	bucketTallies := make(map[string]*accounting.BucketTally)
	var expectedTallies []accounting.BucketTally

	for i := 0; i < 4; i++ {
		bucketName := fmt.Sprintf("%s%d", "testbucket", i)
		bucketID := storj.JoinPaths(projectID.String(), bucketName)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		tally := accounting.BucketTally{
			BucketName:     []byte(bucketName),
			ProjectID:      projectID,
			ObjectCount:    int64(1),
			InlineSegments: int64(1),
			RemoteSegments: int64(1),
			InlineBytes:    int64(1),
			RemoteBytes:    int64(1),
			MetadataSize:   int64(1),
		}
		bucketTallies[bucketID] = &tally
		expectedTallies = append(expectedTallies, tally)

	}
	return bucketTallies, expectedTallies, nil
}

// make rollups and tallies for specified nodes and date range.
func makeRollupsAndStorageNodeStorageTallies(nodes []storj.NodeID, start time.Time, days int) (accounting.RollupStats, map[time.Time]map[storj.NodeID]float64, time.Time) {
	rollups := make(accounting.RollupStats)
	tallies := make(map[time.Time]map[storj.NodeID]float64)

	const (
		hours = 12
	)

	for i := 0; i < days; i++ {
		startDay := time.Date(start.Year(), start.Month(), start.Day()+i, 0, 0, 0, 0, start.Location())
		if rollups[startDay] == nil {
			rollups[startDay] = make(map[storj.NodeID]*accounting.Rollup)
		}

		for _, node := range nodes {
			rollup := &accounting.Rollup{
				NodeID:    node,
				StartTime: startDay,
			}

			for h := 0; h < hours; h++ {
				startTime := startDay.Add(time.Hour * time.Duration(h))
				if tallies[startTime] == nil {
					tallies[startTime] = make(map[storj.NodeID]float64)
				}

				tallieAtRest := math.Round(testrand.Float64n(1000))
				tallies[startTime][node] = tallieAtRest
				rollup.AtRestTotal += tallieAtRest
			}

			rollups[startDay][node] = rollup
		}
	}

	return rollups, tallies, time.Date(start.Year(), start.Month(), start.Day()+days-1, 0, 0, 0, 0, start.Location())
}
