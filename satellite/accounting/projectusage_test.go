// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink/private/storage/meta"
)

func TestProjectUsageStorage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Rollup.MaxAlphaUsage = 1 * memory.MB
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var uploaded uint32

		checkctx, checkcancel := context.WithCancel(ctx)
		defer checkcancel()

		var group errgroup.Group
		group.Go(func() error {
			// wait things to be uploaded
			for atomic.LoadUint32(&uploaded) == 0 {
				if !sync2.Sleep(checkctx, time.Microsecond) {
					return nil
				}
			}

			for {
				if !sync2.Sleep(checkctx, time.Microsecond) {
					return nil
				}

				total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, planet.Uplinks[0].Projects[0].ID)
				if err != nil {
					return errs.Wrap(err)
				}
				if total == 0 {
					return errs.New("got 0 from GetProjectStorageTotals")
				}
			}
		})

		data := testrand.Bytes(1 * memory.MB)

		// successful upload
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/0", data)
		atomic.StoreUint32(&uploaded, 1)
		require.NoError(t, err)
		planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()

		// upload fails due to storage limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/1", data)
		require.Error(t, err)
		if !errs2.IsRPC(err, rpcstatus.ResourceExhausted) {
			t.Fatal("Expected resource exhausted error. Got", err.Error())
		}

		checkcancel()
		if err := group.Wait(); err != nil {
			t.Fatal(err)
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
					now := time.Now()
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

func TestProjectBandwidthRollups(t *testing.T) {
	timeBuf := time.Second * 5

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		p1 := testrand.UUID()
		p2 := testrand.UUID()
		b1 := testrand.Bytes(10)
		b2 := testrand.Bytes(20)

		now := time.Now().UTC()
		// could be flaky near next month
		if now.Month() != now.Add(timeBuf).Month() {
			time.Sleep(timeBuf)
			now = time.Now().UTC()
		}

		hour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		// things that should be counted
		err := db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().WithTransaction(ctx, func(ctx context.Context, tx orders.Transaction) error {
			rollups := []orders.BucketBandwidthRollup{
				{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000 /* counted */, Settled: 1000},
				{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000 /* counted */, Settled: 1000},
			}
			return tx.UpdateBucketBandwidthBatch(ctx, hour, rollups)
		})
		require.NoError(t, err)
		// things that shouldn't be counted
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_PUT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_PUT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_PUT_GRACEFUL_EXIT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_PUT_REPAIR, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_GET_AUDIT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_GET_REPAIR, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b1, pb.PieceAction_PUT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b2, pb.PieceAction_PUT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b1, pb.PieceAction_PUT_GRACEFUL_EXIT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b2, pb.PieceAction_PUT_REPAIR, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b1, pb.PieceAction_GET_AUDIT, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p2, b2, pb.PieceAction_GET_REPAIR, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().WithTransaction(ctx, func(ctx context.Context, tx orders.Transaction) error {
			rollups := []orders.BucketBandwidthRollup{
				{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_PUT_GRACEFUL_EXIT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_PUT_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_GET_AUDIT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_GET_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_PUT_GRACEFUL_EXIT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_PUT_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_GET_AUDIT, Inline: 1000, Allocated: 1000, Settled: 1000},
				{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_GET_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000},
			}
			return tx.UpdateBucketBandwidthBatch(ctx, hour, rollups)
		})
		require.NoError(t, err)

		alloc, err := db.ProjectAccounting().GetCurrentBandwidthAllocated(ctx, p1)
		require.NoError(t, err)
		require.EqualValues(t, 4000, alloc)
	})
}

func createBucketID(projectID uuid.UUID, bucket []byte) []byte {
	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, string(bucket))
	return []byte(storj.JoinPaths(entries...))
}

func createBucketBandwidthRollupsForPast4Days(ctx *testcontext.Context, satelliteDB satellite.DB, projectID uuid.UUID) (int64, error) {
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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		pdb := db.ProjectAccounting()
		projectID := testrand.UUID()

		// Setup: create bucket bandwidth rollup records
		expectedTotal, err := createBucketBandwidthRollupsForPast4Days(ctx, db, projectID)
		require.NoError(t, err)

		// Execute test: get project bandwidth total
		since := time.Now().AddDate(0, -1, 0)

		actualBandwidthTotal, err := pdb.GetAllocatedBandwidthTotal(ctx, projectID, since)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, actualBandwidthTotal)
	})
}

func setUpBucketBandwidthAllocations(ctx *testcontext.Context, projectID uuid.UUID, orderDB orders.DB, now time.Time) error {
	// Create many records that sum greater than project usage limit of 25GB
	for i := 0; i < 4; i++ {
		bucketName := fmt.Sprintf("%s%d", "testbucket", i)

		// In order to exceed the project limits, create bandwidth allocation records
		// that sum greater than the maxAlphaUsage
		amount := 10 * memory.GB.Int64()
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
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		// Setup: add data to live accounting to exceed new limit
		err = projectUsage.AddProjectStorageUsage(ctx, project.ID, expectedLimit.Int64())
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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			numBuckets     = 5
			tallyIntervals = 10
			tallyInterval  = time.Hour
		)

		now := time.Now()
		start := now.Add(tallyInterval * time.Duration(-tallyIntervals))

		db := planet.Satellites[0].DB

		project1 := planet.Uplinks[0].Projects[0].ID
		project2 := planet.Uplinks[1].Projects[0].ID

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
			bucketName := fmt.Sprintf("bucket-%d", i)

			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

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

			err = planet.Uplinks[1].CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

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

			bucketsPage, err = usageRollups.GetBucketTotals(ctx, project1, accounting.BucketUsageCursor{Limit: 5, Search: "bucket-0", Page: 1}, start, now)
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

func TestProjectUsage_FreeUsedStorageSpace(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		acctDB := satDB.ProjectAccounting()
		accounting := planet.Satellites[0].Accounting
		satMetainfo := planet.Satellites[0].Metainfo
		projectsDB := satDB.Console().Projects()

		accounting.Tally.Loop.Pause()

		projects, err := projectsDB.GetAll(ctx)
		require.NoError(t, err)

		project := projects[0]

		// set custom usage limit for project
		customLimit := 100 * memory.KiB
		err = acctDB.UpdateProjectUsageLimit(ctx, project.ID, customLimit)
		require.NoError(t, err)

		data := testrand.Bytes(50 * memory.KiB)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path1", data)
		require.NoError(t, err)

		// check if usage is equal to first uploaded file
		prefix, err := metainfo.CreatePath(ctx, project.ID, -1, []byte("testbucket"), []byte{})
		require.NoError(t, err)
		items, _, err := satMetainfo.Service.List(ctx, prefix, "", true, 1, meta.All)
		require.NoError(t, err)

		usage, err := accounting.ProjectUsage.GetProjectStorageTotals(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, items[0].Pointer.SegmentSize, usage)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path2", data)
		require.NoError(t, err)

		// we used limit so we should get error
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path3", data)
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

		// delete object to free some storage space
		err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "testbucket", "test/path2")
		require.NoError(t, err)

		// we need to wait for tally to update storage usage after delete
		accounting.Tally.Loop.TriggerWait()

		// try to upload object as we have free space now
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path3", data)
		require.NoError(t, err)

		// should fail because we once again used space up to limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path2", data)
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))
	})
}
