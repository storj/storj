// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	rollupsCount = 25
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
		actualTallies, err := pdb.SaveTallies(ctx, intervalStart, bucketTallies)
		require.NoError(t, err)
		for _, tally := range actualTallies {
			require.Contains(t, expectedTallies, tally)
		}
	})
}

func TestStorageNodeUsage(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		nodeID := testrand.NodeID()

		var nodes storj.NodeIDList
		nodes = append(nodes, nodeID)
		nodes = append(nodes, testrand.NodeID())
		nodes = append(nodes, testrand.NodeID())
		nodes = append(nodes, testrand.NodeID())

		accountingDB := db.StoragenodeAccounting()

		// create last rollup timestamp
		_, err := accountingDB.LastTimestamp(ctx, accounting.LastRollup)
		require.NoError(t, err)

		rollups := createRollups(nodes)

		storageTallies := make(map[storj.NodeID]float64)
		storageTallies[nodeID] = testrand.Float64n(10000)

		now := time.Now().UTC()

		// run 2 rollups for the same day
		err = accountingDB.SaveRollup(ctx, now, rollups)
		require.NoError(t, err)
		err = accountingDB.SaveRollup(ctx, now.Add(-time.Hour), rollups)
		require.NoError(t, err)

		// create tallies new tallies
		err = accountingDB.SaveTallies(ctx, now, storageTallies)
		require.NoError(t, err)
		err = accountingDB.SaveTallies(ctx, now.Add(time.Minute), storageTallies)
		require.NoError(t, err)

		nodeStorageUsages, err := accountingDB.QueryStorageNodeUsage(ctx, nodeID, time.Time{}, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStorageUsages)
		assert.Equal(t, rollupsCount+1, len(nodeStorageUsages))

		for _, usage := range nodeStorageUsages {
			assert.Equal(t, nodeID, usage.NodeID)
		}

		assert.Equal(t,
			nodeStorageUsages[len(nodeStorageUsages)-1].Timestamp.UTC(),
			time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
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
			ProjectID:      projectID[:],
			InlineSegments: int64(1),
			RemoteSegments: int64(1),
			Files:          int64(1),
			InlineBytes:    int64(1),
			RemoteBytes:    int64(1),
			MetadataSize:   int64(1),
		}
		bucketTallies[bucketID] = &tally
		expectedTallies = append(expectedTallies, tally)

	}
	return bucketTallies, expectedTallies, nil
}

func createRollups(nodes storj.NodeIDList) accounting.RollupStats {
	rollups := make(accounting.RollupStats)
	now := time.Now().UTC()

	for i := 0; i < rollupsCount; i++ {
		startDate := time.Date(now.Year(), now.Month()-1, 1+i, 0, 0, 0, 0, now.Location())
		if rollups[startDate] == nil {
			rollups[startDate] = make(map[storj.NodeID]*accounting.Rollup)
		}

		for _, nodeID := range nodes {
			rollup := &accounting.Rollup{
				NodeID:         nodeID,
				StartTime:      startDate,
				PutTotal:       testrand.Int63n(10000),
				GetTotal:       testrand.Int63n(10000),
				GetAuditTotal:  testrand.Int63n(10000),
				GetRepairTotal: testrand.Int63n(10000),
				PutRepairTotal: testrand.Int63n(10000),
				AtRestTotal:    testrand.Float64n(10000),
			}

			rollups[startDate][nodeID] = rollup
		}
	}

	return rollups
}
