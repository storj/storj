// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectBandwidthTotal(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		accountingDB := db.Accounting()
		projectID, err := uuid.New()
		require.NoError(t, err)

		// Setup: create bucket bandwidth rollup records
		expectedTotal, err := createBucketBandwidthRollups(ctx, db, *projectID)
		require.NoError(t, err)

		// Execute test: get project bandwidth total
		bucketID := createBucketID(*projectID, []byte("testbucket"))
		from := time.Now().AddDate(0, 0, -accounting.AverageDaysInMonth) // past 30 days
		actualBandwidthTotal, err := accountingDB.ProjectBandwidthTotal(ctx, bucketID, from)
		require.NoError(t, err)
		require.Equal(t, actualBandwidthTotal, expectedTotal)
	})
}

func createBucketBandwidthRollups(ctx *testcontext.Context, satelliteDB satellite.DB, projectID uuid.UUID) (int64, error) {
	var expectedSum int64
	ordersDB := satelliteDB.Orders()
	amount := int64(1000)
	now := time.Now()

	for i := 0; i < 4; i++ {
		var bucketName string
		var intervalStart time.Time
		if i%2 == 0 {
			// When the bucket name and intervalStart is different, a new record is created
			bucketName = fmt.Sprintf("%s%d", "testbucket", i)
			// Use a intervalStart time in the past to test we get all records in past 30 days
			intervalStart = now.AddDate(0, 0, -i)
		} else {
			// When the bucket name and intervalStart is the same, we update the existing record
			bucketName = "testbucket"
			intervalStart = now
		}

		bucketID := createBucketID(projectID, []byte(bucketName))
		err := ordersDB.UpdateBucketBandwidthAllocation(ctx,
			bucketID, pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		err = ordersDB.UpdateBucketBandwidthSettle(ctx,
			bucketID, pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		err = ordersDB.UpdateBucketBandwidthInline(ctx,
			bucketID, pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		expectedSum += amount
	}
	return expectedSum, nil
}

func createBucketID(projectID uuid.UUID, bucket []byte) []byte {
	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, string(bucket))
	return []byte(storj.JoinPaths(entries...))
}

func TestSaveBucketTallies(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// Setup: create bucket storage tallies
		projectID, err := uuid.New()
		require.NoError(t, err)
		bucketTallies, expectedTallies, err := createBucketStorageTallies(*projectID)
		require.NoError(t, err)

		// Execute test:  retrieve the save tallies and confirm they contains the expected data
		intervalStart := time.Now()
		accountingDB := db.Accounting()
		actualTallies, err := accountingDB.SaveBucketTallies(ctx, intervalStart, bucketTallies)
		require.NoError(t, err)
		for _, tally := range actualTallies {
			require.Contains(t, expectedTallies, tally)
		}
	})
}

func createBucketStorageTallies(projectID uuid.UUID) (map[string]*accounting.BucketTally, []accounting.BucketTally, error) {
	bucketTallies := make(map[string]*accounting.BucketTally)
	var expectedTallies []accounting.BucketTally

	for i := 0; i < 4; i++ {

		bucketName := fmt.Sprintf("%s%d", "testbucket", i)
		bucketID := storj.JoinPaths(projectID.String(), bucketName)
		bucketIDComponents := storj.SplitPath(bucketID)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		tally := accounting.BucketTally{
			BucketName:     []byte(bucketIDComponents[1]),
			ProjectID:      []byte(bucketIDComponents[0]),
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
