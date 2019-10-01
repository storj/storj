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

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectUsageStorage(t *testing.T) {
	cases := []struct {
		name             string
		expectedExceeded bool
		expectedResource string
		expectedStatus   rpcstatus.StatusCode
	}{
		{name: "doesn't exceed storage or bandwidth project limit", expectedExceeded: false, expectedStatus: 0},
		{name: "exceeds storage project limit", expectedExceeded: true, expectedResource: "storage", expectedStatus: rpcstatus.ResourceExhausted},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saDB := planet.Satellites[0].DB
		acctDB := saDB.ProjectAccounting()

		// Setup: create a new project to use the projectID
		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		projectID := projects[0].ID

		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		for _, ttc := range cases {
			testCase := ttc
			t.Run(testCase.name, func(t *testing.T) {

				// Setup: create BucketStorageTally records to test exceeding storage project limit
				if testCase.expectedResource == "storage" {
					now := time.Now()
					err := setUpStorageTallies(ctx, projectID, acctDB, 25, now)
					require.NoError(t, err)
				}

				actualExceeded, _, err := projectUsage.ExceedsStorageUsage(ctx, projectID)
				require.NoError(t, err)
				require.Equal(t, testCase.expectedExceeded, actualExceeded)

				// Setup: create some bytes for the uplink to upload
				expectedData := testrand.Bytes(50 * memory.KiB)

				// Execute test: check that the uplink gets an error when they have exceeded storage limits and try to upload a file
				actualErr := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
				if testCase.expectedResource == "storage" {
					require.True(t, errs2.IsRPC(actualErr, testCase.expectedStatus))
				} else {
					require.NoError(t, actualErr)
				}
			})
		}
	})
}

func TestProjectUsageBandwidth(t *testing.T) {
	cases := []struct {
		name             string
		expectedExceeded bool
		expectedResource string
		expectedStatus   rpcstatus.StatusCode
	}{
		{name: "doesn't exceed storage or bandwidth project limit", expectedExceeded: false, expectedStatus: 0},
		{name: "exceeds bandwidth project limit", expectedExceeded: true, expectedResource: "bandwidth", expectedStatus: rpcstatus.ResourceExhausted},
	}

	for _, tt := range cases {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
				saDB := planet.Satellites[0].DB
				orderDB := saDB.Orders()

				// Setup: get projectID and create bucketID
				projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
				projectID := projects[0].ID
				require.NoError(t, err)
				bucketName := "testbucket"
				bucketID := createBucketID(projectID, []byte(bucketName))

				projectUsage := planet.Satellites[0].Accounting.ProjectUsage

				// Setup: create a BucketBandwidthRollup record to test exceeding bandwidth project limit
				if testCase.expectedResource == "bandwidth" {
					now := time.Now().UTC()
					err := setUpBucketBandwidthAllocations(ctx, projectID, orderDB, now)
					require.NoError(t, err)
				}

				// Setup: create some bytes for the uplink to upload to test the download later
				expectedData := testrand.Bytes(50 * memory.KiB)

				filePath := "test/path"
				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, filePath, expectedData)
				require.NoError(t, err)

				actualExceeded, _, err := projectUsage.ExceedsBandwidthUsage(ctx, projectID, bucketID)
				require.NoError(t, err)
				require.Equal(t, testCase.expectedExceeded, actualExceeded)

				// Execute test: check that the uplink gets an error when they have exceeded bandwidth limits and try to download a file
				_, actualErr := planet.Uplinks[0].Download(ctx, planet.Satellites[0], bucketName, filePath)
				if testCase.expectedResource == "bandwidth" {
					require.True(t, errs2.IsRPC(actualErr, testCase.expectedStatus))
				} else {
					require.NoError(t, actualErr)
				}
			})
		})
	}
}

func createBucketID(projectID uuid.UUID, bucket []byte) []byte {
	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, string(bucket))
	return []byte(storj.JoinPaths(entries...))
}

func setUpStorageTallies(ctx *testcontext.Context, projectID uuid.UUID, acctDB accounting.ProjectAccounting, numberOfGB int, time time.Time) error {

	// Create many records that sum greater than project usage limit of 25GB
	for i := 0; i < numberOfGB; i++ {
		bucketName := fmt.Sprintf("%s%d", "testbucket", i)
		tally := accounting.BucketStorageTally{
			BucketName:    bucketName,
			ProjectID:     projectID,
			IntervalStart: time,

			// In order to exceed the project limits, create storage tally records
			// that sum greater than the maxAlphaUsage * expansionFactor
			RemoteBytes: memory.GB.Int64() * accounting.ExpansionFactor,
		}
		err := acctDB.CreateStorageTally(ctx, tally)
		if err != nil {
			return err
		}
	}
	return nil
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

		err := ordersDB.UpdateBucketBandwidthAllocation(ctx,
			projectID, []byte(bucketName), pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		err = ordersDB.UpdateBucketBandwidthSettle(ctx,
			projectID, []byte(bucketName), pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		err = ordersDB.UpdateBucketBandwidthInline(ctx,
			projectID, []byte(bucketName), pb.PieceAction_GET, amount, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		expectedSum += amount
	}
	return expectedSum, nil
}

func TestProjectBandwidthTotal(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		pdb := db.ProjectAccounting()
		projectID := testrand.UUID()

		// Setup: create bucket bandwidth rollup records
		expectedTotal, err := createBucketBandwidthRollups(ctx, db, projectID)
		require.NoError(t, err)

		// Execute test: get project bandwidth total
		from := time.Now().AddDate(0, 0, -accounting.AverageDaysInMonth) // past 30 days
		actualBandwidthTotal, err := pdb.GetAllocatedBandwidthTotal(ctx, projectID, from)
		require.NoError(t, err)
		require.Equal(t, actualBandwidthTotal, expectedTotal)
	})
}

func setUpBucketBandwidthAllocations(ctx *testcontext.Context, projectID uuid.UUID, orderDB orders.DB, now time.Time) error {

	// Create many records that sum greater than project usage limit of 25GB
	for i := 0; i < 4; i++ {
		bucketName := fmt.Sprintf("%s%d", "testbucket", i)

		// In order to exceed the project limits, create bandwidth allocation records
		// that sum greater than the maxAlphaUsage * expansionFactor
		amount := 10 * memory.GB.Int64() * accounting.ExpansionFactor
		action := pb.PieceAction_GET
		intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		err := orderDB.UpdateBucketBandwidthAllocation(ctx, projectID, []byte(bucketName), action, amount, intervalStart)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestProjectUsageCustomLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		acctDB := satDB.ProjectAccounting()

		projectsDB := satDB.Console().Projects()
		projects, err := projectsDB.GetAll(ctx)
		require.NoError(t, err)

		project := projects[0]
		// set custom usage limit for project
		project.UsageLimit = memory.GiB.Int64() * 10
		err = projectsDB.Update(ctx, &project)
		require.NoError(t, err)

		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		// Setup: create BucketStorageTally records to test exceeding storage project limit
		now := time.Now()
		err = setUpStorageTallies(ctx, project.ID, acctDB, 11, now)
		require.NoError(t, err)

		actualExceeded, limit, err := projectUsage.ExceedsStorageUsage(ctx, project.ID)
		require.NoError(t, err)
		require.True(t, actualExceeded)
		require.Equal(t, project.UsageLimit, limit.Int64())

		// Setup: create some bytes for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Execute test: check that the uplink gets an error when they have exceeded storage limits and try to upload a file
		actualErr := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		assert.Error(t, actualErr)
	})
}
