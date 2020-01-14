// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
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
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

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
				planet.Satellites[0].Accounting.Tally.Loop.Pause()

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
		expectedLimit := memory.Size(memory.GiB.Int64() * 10)

		err = acctDB.UpdateProjectUsageLimit(ctx, project.ID, expectedLimit)
		require.NoError(t, err)

		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		// Setup: create BucketStorageTally records to test exceeding storage project limit
		now := time.Now()
		err = setUpStorageTallies(ctx, project.ID, acctDB, 11, now)
		require.NoError(t, err)

		actualExceeded, limit, err := projectUsage.ExceedsStorageUsage(ctx, project.ID)
		require.NoError(t, err)
		require.True(t, actualExceeded)
		require.Equal(t, expectedLimit.Int64(), limit.Int64())

		// Setup: create some bytes for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Execute test: check that the uplink gets an error when they have exceeded storage limits and try to upload a file
		actualErr := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.Error(t, actualErr)
	})
}

func TestUsageRollups(t *testing.T) {
	const (
		numBuckets     = 5
		tallyIntervals = 10
		tallyInterval  = time.Hour
	)

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		now := time.Now()
		start := now.Add(tallyInterval * time.Duration(-tallyIntervals))

		project1 := testrand.UUID()
		project2 := testrand.UUID()

		p1base := binary.BigEndian.Uint64(project1[:8]) >> 48
		p2base := binary.BigEndian.Uint64(project2[:8]) >> 48

		getValue := func(i, j int, base uint64) int64 {
			a := uint64((i+1)*(j+1)) ^ base
			a &^= (1 << 63)
			return int64(a)
		}

		actions := []pb.PieceAction{
			pb.PieceAction_GET,
			pb.PieceAction_GET_AUDIT,
			pb.PieceAction_GET_REPAIR,
		}

		var buckets []string
		for i := 0; i < numBuckets; i++ {
			bucketName := fmt.Sprintf("bucket_%d", i)

			// project 1
			for _, action := range actions {
				value := getValue(0, i, p1base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx, project1, []byte(bucketName), action, value*6, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthSettle(ctx, project1, []byte(bucketName), action, value*3, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthInline(ctx, project1, []byte(bucketName), action, value, now)
				require.NoError(t, err)
			}

			// project 2
			for _, action := range actions {
				value := getValue(1, i, p2base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx, project2, []byte(bucketName), action, value*6, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthSettle(ctx, project2, []byte(bucketName), action, value*3, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthInline(ctx, project2, []byte(bucketName), action, value, now)
				require.NoError(t, err)
			}

			buckets = append(buckets, bucketName)
		}

		for i := 0; i < tallyIntervals; i++ {
			interval := start.Add(tallyInterval * time.Duration(i))

			bucketTallies := make(map[string]*accounting.BucketTally)
			for j, bucket := range buckets {
				bucketID1 := project1.String() + "/" + bucket
				bucketID2 := project2.String() + "/" + bucket
				value1 := getValue(i, j, p1base) * 10
				value2 := getValue(i, j, p2base) * 10

				tally1 := &accounting.BucketTally{
					BucketName:     []byte(bucket),
					ProjectID:      project1,
					ObjectCount:    value1,
					InlineSegments: value1,
					RemoteSegments: value1,
					InlineBytes:    value1,
					RemoteBytes:    value1,
					MetadataSize:   value1,
				}

				tally2 := &accounting.BucketTally{
					BucketName:     []byte(bucket),
					ProjectID:      project2,
					ObjectCount:    value2,
					InlineSegments: value2,
					RemoteSegments: value2,
					InlineBytes:    value2,
					RemoteBytes:    value2,
					MetadataSize:   value2,
				}

				bucketTallies[bucketID1] = tally1
				bucketTallies[bucketID2] = tally2
			}

			err := db.ProjectAccounting().SaveTallies(ctx, interval, bucketTallies)
			require.NoError(t, err)
		}

		usageRollups := db.ProjectAccounting()

		t.Run("test project total", func(t *testing.T) {
			projTotal1, err := usageRollups.GetProjectTotal(ctx, project1, start, now)
			require.NoError(t, err)
			require.NotNil(t, projTotal1)

			projTotal2, err := usageRollups.GetProjectTotal(ctx, project2, start, now)
			require.NoError(t, err)
			require.NotNil(t, projTotal2)
		})

		t.Run("test bucket usage rollups", func(t *testing.T) {
			rollups1, err := usageRollups.GetBucketUsageRollups(ctx, project1, start, now)
			require.NoError(t, err)
			require.NotNil(t, rollups1)

			rollups2, err := usageRollups.GetBucketUsageRollups(ctx, project2, start, now)
			require.NoError(t, err)
			require.NotNil(t, rollups2)
		})

		t.Run("test bucket totals", func(t *testing.T) {
			cursor := accounting.BucketUsageCursor{
				Limit: 20,
				Page:  1,
			}

			totals1, err := usageRollups.GetBucketTotals(ctx, project1, cursor, start, now)
			require.NoError(t, err)
			require.NotNil(t, totals1)

			totals2, err := usageRollups.GetBucketTotals(ctx, project2, cursor, start, now)
			require.NoError(t, err)
			require.NotNil(t, totals2)
		})

		t.Run("Get paged", func(t *testing.T) {
			// sql injection test. F.E '%SomeText%' = > ''%SomeText%' OR 'x' != '%'' will be true
			bucketsPage, err := usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 5, Search: "buck%' OR 'x' != '", Page: 1}, start, now)
			require.NoError(t, err)
			require.NotNil(t, bucketsPage)
			assert.Equal(t, uint64(0), bucketsPage.TotalCount)
			assert.Equal(t, uint(0), bucketsPage.CurrentPage)
			assert.Equal(t, uint(0), bucketsPage.PageCount)
			assert.Equal(t, 0, len(bucketsPage.BucketUsages))

			bucketsPage, err = usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 3, Search: "", Page: 1}, start, now)
			require.NoError(t, err)
			require.NotNil(t, bucketsPage)
			assert.Equal(t, uint64(5), bucketsPage.TotalCount)
			assert.Equal(t, uint(1), bucketsPage.CurrentPage)
			assert.Equal(t, uint(2), bucketsPage.PageCount)
			assert.Equal(t, 3, len(bucketsPage.BucketUsages))

			bucketsPage, err = usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 5, Search: "buck", Page: 1}, start, now)
			require.NoError(t, err)
			require.NotNil(t, bucketsPage)
			assert.Equal(t, uint64(5), bucketsPage.TotalCount)
			assert.Equal(t, uint(1), bucketsPage.CurrentPage)
			assert.Equal(t, uint(1), bucketsPage.PageCount)
			assert.Equal(t, 5, len(bucketsPage.BucketUsages))

			bucketsPage, err = usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 5, Search: "bucket_0", Page: 1}, start, now)
			require.NoError(t, err)
			require.NotNil(t, bucketsPage)
			assert.Equal(t, uint64(1), bucketsPage.TotalCount)
			assert.Equal(t, uint(1), bucketsPage.CurrentPage)
			assert.Equal(t, uint(1), bucketsPage.PageCount)
			assert.Equal(t, 1, len(bucketsPage.BucketUsages))

			bucketsPage, err = usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 5, Search: "buck\xff", Page: 1}, start, now)
			require.NoError(t, err)
			require.NotNil(t, bucketsPage)
			assert.Equal(t, uint64(0), bucketsPage.TotalCount)
			assert.Equal(t, uint(0), bucketsPage.CurrentPage)
			assert.Equal(t, uint(0), bucketsPage.PageCount)
			assert.Equal(t, 0, len(bucketsPage.BucketUsages))
		})
	})
}
