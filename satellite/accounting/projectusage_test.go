// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	satbuckets "storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	snorders "storj.io/storj/storagenode/orders"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

func TestProjectUsageStorage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits.Storage.Free = 1 * memory.MB
				config.Console.UsageLimits.Bandwidth.Free = 1 * memory.MB
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

		// set limit manually to 1MB until column values can be nullable
		accountingDB := planet.Satellites[0].DB.ProjectAccounting()
		err := accountingDB.UpdateProjectUsageLimit(ctx, planet.Uplinks[0].Projects[0].ID, 1*memory.MB)
		require.NoError(t, err)

		// successful upload
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/0", data)
		atomic.StoreUint32(&uploaded, 1)
		require.NoError(t, err)
		planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()

		// upload fails due to storage limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/1", data)
		require.Error(t, err)
		// TODO error should be compared to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "storage limit", err.Error())

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
		expectedError    error
	}{
		{name: "doesn't exceed storage or bandwidth project limit", expectedExceeded: false, expectedError: nil},
		{name: "exceeds bandwidth project limit", expectedExceeded: true, expectedResource: "bandwidth", expectedError: uplink.ErrBandwidthLimitExceeded},
	}

	for _, tt := range cases {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
				Reconfigure: testplanet.Reconfigure{
					Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
						config.LiveAccounting.AsOfSystemInterval = -time.Millisecond
					},
				},
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
				saDB := planet.Satellites[0].DB
				orderDB := saDB.Orders()

				now := time.Now()

				// make sure we don't end up with a flaky test if we are in the beginning of the month as we have to add expired bandwidth allocations
				if now.Day() < 5 {
					now = time.Date(now.Year(), now.Month(), 5, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
				}
				bucket := metabase.BucketLocation{ProjectID: planet.Uplinks[0].Projects[0].ID, BucketName: "testbucket"}
				projectUsage := planet.Satellites[0].Accounting.ProjectUsage

				// Setup: create a BucketBandwidthRollup record to test exceeding bandwidth project limit
				if testCase.expectedResource == "bandwidth" {
					err := setUpBucketBandwidthAllocations(ctx, bucket.ProjectID, orderDB, now)
					require.NoError(t, err)
				}

				// Setup: create a BucketBandwidthRollup record that should not be taken into account as
				// it is expired.
				err := setUpBucketBandwidthAllocations(ctx, bucket.ProjectID, orderDB, now.Add(-72*time.Hour))
				require.NoError(t, err)

				// Setup: create some bytes for the uplink to upload to test the download later
				expectedData := testrand.Bytes(50 * memory.KiB)

				filePath := "test/path"
				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], string(bucket.BucketName), filePath, expectedData)
				require.NoError(t, err)

				projectUsage.SetNow(func() time.Time {
					return now
				})
				actualExceeded, _, err := projectUsage.ExceedsBandwidthUsage(ctx, accounting.ProjectLimits{ProjectID: bucket.ProjectID})
				require.NoError(t, err)
				require.Equal(t, testCase.expectedExceeded, actualExceeded)

				// Execute test: check that the uplink gets an error when they have exceeded bandwidth limits and try to download a file
				_, actualErr := planet.Uplinks[0].Download(ctx, planet.Satellites[0], string(bucket.BucketName), filePath)
				if testCase.expectedResource == "bandwidth" {
					require.True(t, errors.Is(actualErr, testCase.expectedError))
				} else {
					require.NoError(t, actualErr)
				}
			})
		})
	}
}

func TestProjectSegmentLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.MaxSegmentSize = 20 * memory.KiB
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// tally self-corrects live accounting, however, it may cause things to be temporarily off by a few segments.
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		// set limit manually to 10 segments
		accountingDB := planet.Satellites[0].DB.ProjectAccounting()
		err := accountingDB.UpdateProjectSegmentLimit(ctx, planet.Uplinks[0].Projects[0].ID, 10)
		require.NoError(t, err)

		// successful upload
		data := testrand.Bytes(160 * memory.KiB)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/0", data)
		require.NoError(t, err)

		// upload fails due to segment limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/1", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "segments limit")
	})
}

func TestProjectSegmentLimitInline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// tally self-corrects live accounting, however, it may cause things to be temporarily off by a few segments.
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		// set limit manually to 10 segments
		accountingDB := planet.Satellites[0].DB.ProjectAccounting()
		err := accountingDB.UpdateProjectSegmentLimit(ctx, planet.Uplinks[0].Projects[0].ID, 10)
		require.NoError(t, err)

		data := testrand.Bytes(1 * memory.KiB)
		for i := 0; i < 10; i++ {
			// successful upload
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/"+strconv.Itoa(i), data)
			require.NoError(t, err)
		}

		// upload fails due to segment limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/1", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "segments limit")
	})
}

func TestProjectBandwidthLimitWithoutCache(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits.Bandwidth.Free = 15 * memory.KiB
				config.Console.UsageLimits.Bandwidth.Paid = 15 * memory.KiB
				// this effectively disable live accounting cache
				config.LiveAccounting.BandwidthCacheTTL = -1
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(5 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/1", expectedData)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path/1")
			require.NoError(t, err)
			require.Equal(t, data, expectedData)
		}

		// flush allocated bandwidth to DB as we will use DB directly without cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path/1")
		require.Error(t, err)
		require.True(t, errors.Is(err, uplink.ErrBandwidthLimitExceeded))
	})
}

func TestProjectSegmentLimitMultipartUpload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// tally self-corrects live accounting, however, it may cause things to be temporarily off by a few segments.
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		// set limit manually to 10 segments
		accountingDB := planet.Satellites[0].DB.ProjectAccounting()
		err := accountingDB.UpdateProjectSegmentLimit(ctx, planet.Uplinks[0].Projects[0].ID, 4)
		require.NoError(t, err)

		data := testrand.Bytes(1 * memory.KiB)
		for i := 0; i < 4; i++ {
			// successful upload
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path/"+strconv.Itoa(i), data)
			require.NoError(t, err)
		}

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// multipart API upload should call BeginObject and return error on segment limit validation
		_, err = project.BeginUpload(ctx, "testbucket", "test/path/4", &uplink.UploadOptions{})
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "segments limit")
	})
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
		// make sure we don't end up with a flaky test if we are in the beginning of the month as we have to add expired bandwidth allocations
		if now.Day() < 5 {
			now = time.Date(now.Year(), now.Month(), 5, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
		}
		hour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		expired := time.Date(now.Year(), now.Month(), now.Day()-3, now.Hour(), 0, 0, 0, now.Location())

		// things that should be counted
		err := db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)

		rollups := []orders.BucketBandwidthRollup{
			{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000 /* counted */, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000 /* counted */, Settled: 1000, IntervalStart: hour},
		}
		err = db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		// allocated bandwidth should correspond to the sum of bandwidth corresponding to GET action (4000 here)
		alloc, err := db.ProjectAccounting().GetProjectBandwidth(ctx, p1, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		require.EqualValues(t, 4000, alloc)

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
		// these two should not be counted. They are expired and have no corresponding rollup
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_GET, 1000, expired)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_GET, 1000, expired)
		require.NoError(t, err)

		rollups = []orders.BucketBandwidthRollup{
			{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_PUT_GRACEFUL_EXIT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_PUT_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_GET_AUDIT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_GET_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_PUT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_PUT_GRACEFUL_EXIT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_PUT_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b1), Action: pb.PieceAction_GET_AUDIT, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
			{ProjectID: p2, BucketName: string(b2), Action: pb.PieceAction_GET_REPAIR, Inline: 1000, Allocated: 1000, Settled: 1000, IntervalStart: hour},
		}
		err = db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		// things that should be partially counted (settled amount lower than allocated amount)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b1, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)
		err = db.Orders().UpdateBucketBandwidthAllocation(ctx, p1, b2, pb.PieceAction_GET, 1000, hour)
		require.NoError(t, err)

		rollups = []orders.BucketBandwidthRollup{
			{ProjectID: p1, BucketName: string(b1), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000, Settled: 300, Dead: 700, IntervalStart: hour},
			{ProjectID: p1, BucketName: string(b2), Action: pb.PieceAction_GET, Inline: 1000, Allocated: 1000, Settled: 500, Dead: 500, IntervalStart: hour},
		}
		err = db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		alloc, err = db.ProjectAccounting().GetProjectBandwidth(ctx, p1, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		// new allocated bandwidth: 4000 (from previously) + 4000 (from these rollups) - 1200 (dead bandwidth = 700+500)
		require.EqualValues(t, 6800, alloc)
	})
}

func createBucketSettleBandwidthRollupsForPast4Days(ctx *testcontext.Context, satelliteDB satellite.DB, projectID uuid.UUID) (int64, error) {
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

		err := ordersDB.UpdateBucketBandwidthSettle(ctx,
			projectID, []byte(bucketName), pb.PieceAction_GET, amount, 0, intervalStart,
		)
		if err != nil {
			return expectedSum, err
		}
		expectedSum += amount
	}
	return expectedSum, nil
}

func TestGetProjectSettledBandwidthTotal(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		pdb := db.ProjectAccounting()
		projectID := testrand.UUID()

		// Setup: create bucket bandwidth rollup records
		expectedTotal, err := createBucketSettleBandwidthRollupsForPast4Days(ctx, db, projectID)
		require.NoError(t, err)

		// Execute test: get project bandwidth total
		since := time.Now().AddDate(0, -1, 0)

		actualBandwidthTotal, err := pdb.GetProjectSettledBandwidthTotal(ctx, projectID, since)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, actualBandwidthTotal)
	})
}

func setUpBucketBandwidthAllocations(ctx *testcontext.Context, projectID uuid.UUID, orderDB orders.DB, now time.Time) error {
	// Create many records that sum greater than project usage limit of 50GB
	for i := 0; i < 4; i++ {
		bucketName := fmt.Sprintf("%s%d", "testbucket", i)

		// In order to exceed the project limits, create bandwidth allocation records
		// that sum greater than the defaultMaxUsage
		amount := 15 * memory.GB.Int64()
		action := pb.PieceAction_GET
		err := orderDB.UpdateBucketBandwidthAllocation(ctx, projectID, []byte(bucketName), action, amount, now)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestProjectUsageCustomLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		acctDB := satDB.ProjectAccounting()
		projectsDB := satDB.Console().Projects()

		projects, err := projectsDB.GetAll(ctx)
		require.NoError(t, err)

		project := projects[0]
		// set custom usage limit for project
		expectedLimit := memory.GiB.Int64() * 10
		err = acctDB.UpdateProjectUsageLimit(ctx, project.ID, memory.Size(expectedLimit))
		require.NoError(t, err)

		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		// Setup: add data to live accounting to exceed new limit
		err = projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, expectedLimit, 0)
		require.NoError(t, err)

		limit := projectUsage.ExceedsUploadLimits(ctx, 1, 1, accounting.ProjectLimits{
			ProjectID: project.ID,
			Usage:     &expectedLimit,
		})
		require.True(t, limit.ExceedsStorage)
		require.Equal(t, expectedLimit, limit.StorageLimit.Int64())

		// Setup: create some bytes for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Execute test: check that the uplink gets an error when they have exceeded storage limits and try to upload a file
		actualErr := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.Error(t, actualErr)
	})
}

func TestUsageRollups(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 3,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
				config.Metainfo.ObjectLockEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			numBuckets     = 5
			tallyIntervals = 10
			tallyInterval  = time.Hour
		)

		now := time.Now()
		since := now.Add(-time.Hour * 24)
		start := now.Add(tallyInterval * -tallyIntervals)

		db := planet.Satellites[0].DB

		project1 := planet.Uplinks[0].Projects[0].ID
		project2 := planet.Uplinks[1].Projects[0].ID
		project3 := planet.Uplinks[2].Projects[0].ID

		base := func(projectID uuid.UUID) uint64 {
			return binary.BigEndian.Uint64(projectID[:8]) >> 48
		}

		p1base := base(project1)
		p2base := base(project2)
		p3base := base(project3)

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

		var rollups []orders.BucketBandwidthRollup

		var buckets []string
		placement := func(projectID uuid.UUID) *storj.PlacementConstraint {
			p := storj.PlacementConstraint(base(projectID))
			return &p
		}
		userAgent := func(projectID uuid.UUID) []byte {
			return []byte(fmt.Sprintf("partner-%s", projectID.String()))
		}
		for i := 0; i < numBuckets; i++ {
			bucketName := fmt.Sprintf("bucket-%d", i)

			require.NoError(t, planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, db.Attribution().UpdateUserAgent(ctx, project1, bucketName, userAgent(project1)))
			require.NoError(t, db.Attribution().UpdatePlacement(ctx, project1, bucketName, placement(project1)))

			// project 1
			for _, action := range actions {
				value := getValue(0, i, p1base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx, project1, []byte(bucketName), action, value*6, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthSettle(ctx, project1, []byte(bucketName), action, value*3, 0, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthInline(ctx, project1, []byte(bucketName), action, value, now)
				require.NoError(t, err)
			}

			require.NoError(t, planet.Uplinks[1].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, db.Attribution().UpdateUserAgent(ctx, project2, bucketName, userAgent(project2)))
			require.NoError(t, db.Attribution().UpdatePlacement(ctx, project2, bucketName, placement(project2)))

			// project 2
			for _, action := range actions {
				value := getValue(1, i, p2base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx, project2, []byte(bucketName), action, value*6, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthSettle(ctx, project2, []byte(bucketName), action, value*3, 0, now)
				require.NoError(t, err)

				err = db.Orders().UpdateBucketBandwidthInline(ctx, project2, []byte(bucketName), action, value, now)
				require.NoError(t, err)
			}

			require.NoError(t, planet.Uplinks[2].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, db.Attribution().UpdateUserAgent(ctx, project3, bucketName, userAgent(project3)))
			require.NoError(t, db.Attribution().UpdatePlacement(ctx, project3, bucketName, placement(project3)))

			// project 3
			for _, action := range actions {
				value := getValue(1, i, p2base)

				rollups = append(rollups, orders.BucketBandwidthRollup{
					ProjectID:     project3,
					BucketName:    bucketName,
					Action:        action,
					IntervalStart: now.Add(-time.Hour * 2),
					Inline:        value,
					Allocated:     value * 6,
					Settled:       value * 3,
				})

				rollups = append(rollups, orders.BucketBandwidthRollup{
					ProjectID:     project3,
					BucketName:    bucketName,
					Action:        action,
					IntervalStart: now,
					Inline:        value,
					Allocated:     value * 6,
					Settled:       value * 3,
				})
			}

			buckets = append(buckets, bucketName)
		}

		err := db.Orders().UpdateBandwidthBatch(ctx, rollups)
		require.NoError(t, err)

		for i := 0; i < tallyIntervals; i++ {
			interval := start.Add(tallyInterval * time.Duration(i))

			bucketTallies := make(map[metabase.BucketLocation]*accounting.BucketTally)
			for j, bucket := range buckets {
				bucketLoc1 := metabase.BucketLocation{
					ProjectID:  project1,
					BucketName: metabase.BucketName(bucket),
				}
				bucketLoc2 := metabase.BucketLocation{
					ProjectID:  project2,
					BucketName: metabase.BucketName(bucket),
				}
				bucketLoc3 := metabase.BucketLocation{
					ProjectID:  project3,
					BucketName: metabase.BucketName(bucket),
				}
				value1 := getValue(i, j, p1base) * 10
				value2 := getValue(i, j, p2base) * 10
				value3 := getValue(i, j, p3base) * 10

				tally1 := &accounting.BucketTally{
					BucketLocation: bucketLoc1,
					ObjectCount:    value1,
					TotalSegments:  value1 + value1,
					TotalBytes:     value1 + value1,
					MetadataSize:   value1,
				}

				tally2 := &accounting.BucketTally{
					BucketLocation: bucketLoc2,
					ObjectCount:    value2,
					TotalSegments:  value2 + value2,
					TotalBytes:     value2 + value2,
					MetadataSize:   value2,
				}

				tally3 := &accounting.BucketTally{
					BucketLocation: bucketLoc3,
					ObjectCount:    value3,
					TotalSegments:  value3 + value3,
					TotalBytes:     value3 + value3,
					MetadataSize:   value3,
				}

				bucketTallies[bucketLoc1] = tally1
				bucketTallies[bucketLoc2] = tally2
				bucketTallies[bucketLoc3] = tally3
			}

			err := db.ProjectAccounting().SaveTallies(ctx, interval, bucketTallies)
			require.NoError(t, err)
		}

		usageRollups := db.ProjectAccounting()

		t.Run("buckets since and before", func(t *testing.T) {
			for _, projectID := range []uuid.UUID{project1, project2, project3} {
				bucketInfos, err := usageRollups.GetBucketsSinceAndBefore(ctx, projectID, start, now, false)
				require.NoError(t, err)
				require.Len(t, bucketInfos, 5)
				for _, info := range bucketInfos {
					require.Nil(t, info.Placement)
					require.Nil(t, info.UserAgent)
				}
			}

			for _, projectID := range []uuid.UUID{project1, project2, project3} {
				bucketInfos, err := usageRollups.GetBucketsSinceAndBefore(ctx, projectID, start, now /*withInfo*/, true)
				require.NoError(t, err)
				require.Len(t, bucketInfos, 5)
				for _, info := range bucketInfos {
					require.NotNil(t, info.Placement)
					require.NotNil(t, info.UserAgent)

					require.Equal(t, placement(projectID), info.Placement)
					require.Equal(t, userAgent(projectID), info.UserAgent)
				}
			}
		})

		t.Run("project total", func(t *testing.T) {
			projTotal1, err := usageRollups.GetProjectTotal(ctx, project1, start, now)
			require.NoError(t, err)
			require.NotNil(t, projTotal1)

			projTotal2, err := usageRollups.GetProjectTotal(ctx, project2, start, now)
			require.NoError(t, err)
			require.NotNil(t, projTotal2)

			projTotal3, err := usageRollups.GetProjectTotal(ctx, project3, start, now)
			require.NoError(t, err)
			require.NotNil(t, projTotal3)

			projTotal3Prev2Hours, err := usageRollups.GetProjectTotal(ctx, project3, now.Add(-time.Hour*2), now.Add(-time.Hour*1))
			require.NoError(t, err)
			require.NotNil(t, projTotal3Prev2Hours)
			require.NotZero(t, projTotal3Prev2Hours.Egress)

			projTotal3Prev3Hours, err := usageRollups.GetProjectTotal(ctx, project3, now.Add(-time.Hour*3), now.Add(-time.Hour*2))
			require.NoError(t, err)
			require.NotNil(t, projTotal3Prev3Hours)
			require.NotZero(t, projTotal3Prev3Hours.Egress)
		})

		t.Run("bucket usage rollups", func(t *testing.T) {
			rollups1, err := usageRollups.GetBucketUsageRollups(ctx, project1, start, now, false)
			require.NoError(t, err)
			require.NotNil(t, rollups1)

			rollups2, err := usageRollups.GetBucketUsageRollups(ctx, project2, start, now, false)
			require.NoError(t, err)
			require.NotNil(t, rollups2)

			rollups3, err := usageRollups.GetBucketUsageRollups(ctx, project3, start, now, false)
			require.NoError(t, err)
			require.NotNil(t, rollups3)

			rollups3Prev2Hours, err := usageRollups.GetBucketUsageRollups(ctx, project3, now.Add(-time.Hour*2), now.Add(-time.Hour*1), false)
			require.NoError(t, err)
			require.NotNil(t, rollups3Prev2Hours)

			rollups3Prev3Hours, err := usageRollups.GetBucketUsageRollups(ctx, project3, now.Add(-time.Hour*3), now.Add(-time.Hour*2), false)
			require.NoError(t, err)
			require.NotNil(t, rollups3Prev3Hours)
		})

		t.Run("versioning & object lock status", func(t *testing.T) {
			client, err := planet.Uplinks[0].Projects[0].DialMetainfo(ctx)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, client.Close())
			}()

			for _, bucket := range buckets {
				err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
					Name:       []byte(bucket),
					Versioning: true,
				})
				require.NoError(t, err)
			}

			cursor := accounting.BucketUsageCursor{Limit: 20, Page: 1}

			totals, err := usageRollups.GetBucketTotals(ctx, project1, cursor, since, now)
			require.NoError(t, err)
			require.NotNil(t, totals)
			require.NotZero(t, len(totals.BucketUsages))

			for _, usage := range totals.BucketUsages {
				require.Equal(t, satbuckets.VersioningEnabled, usage.Versioning)
				require.False(t, usage.ObjectLockEnabled)
			}

			for _, bucket := range buckets {
				err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
					Name:       []byte(bucket),
					Versioning: false,
				})
				require.NoError(t, err)
			}

			totals, err = usageRollups.GetBucketTotals(ctx, project1, cursor, since, now)
			require.NoError(t, err)
			require.NotNil(t, totals)

			for _, usage := range totals.BucketUsages {
				require.Equal(t, satbuckets.VersioningSuspended, usage.Versioning)
			}

			upl := planet.Uplinks[0]
			sat := planet.Satellites[0]
			projectID := upl.Projects[0].ID
			userCtx, err := sat.UserContext(ctx, upl.Projects[0].Owner.ID)
			require.NoError(t, err)

			_, key, err := sat.API.Console.Service.CreateAPIKey(userCtx, projectID, "test key", macaroon.APIKeyVersionObjectLock)
			require.NoError(t, err)

			client.SetRawAPIKey(key.SerializeRaw())

			err = sat.API.DB.Console().Projects().UpdateDefaultVersioning(ctx, projectID, console.VersioningEnabled)
			require.NoError(t, err)

			lockedBucket := "lockedbucket"
			_, err = client.CreateBucket(ctx, metaclient.CreateBucketParams{
				Name:              []byte(lockedBucket),
				ObjectLockEnabled: true,
			})
			require.NoError(t, err)

			setObjectLockConfigReq := &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: key.SerializeRaw(),
				},
				Name: []byte(lockedBucket),
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
					DefaultRetention: &pb.DefaultRetention{
						Mode: pb.Retention_Mode(storj.ComplianceMode),
						Duration: &pb.DefaultRetention_Days{
							Days: 1,
						},
					},
				},
			}

			_, err = sat.API.Metainfo.Endpoint.SetBucketObjectLockConfiguration(ctx, setObjectLockConfigReq)
			require.NoError(t, err)

			cursor.Search = lockedBucket
			totals, err = usageRollups.GetBucketTotals(ctx, project1, cursor, since, now)
			require.NoError(t, err)
			require.NotNil(t, totals)
			require.Len(t, totals.BucketUsages, 1)
			require.True(t, totals.BucketUsages[0].ObjectLockEnabled)
			require.Equal(t, storj.ComplianceMode, totals.BucketUsages[0].DefaultRetentionMode)
			require.Nil(t, totals.BucketUsages[0].DefaultRetentionYears)
			require.NotNil(t, totals.BucketUsages[0].DefaultRetentionDays)
			require.Equal(t, 1, *totals.BucketUsages[0].DefaultRetentionDays)

			_, err = client.DeleteBucket(ctx, metaclient.DeleteBucketParams{
				Name: []byte(lockedBucket),
			})
			require.NoError(t, err)
		})

		t.Run("placement", func(t *testing.T) {
			bList, err := db.Buckets().ListBuckets(ctx, planet.Uplinks[0].Projects[0].ID, satbuckets.ListOptions{Direction: satbuckets.DirectionForward}, macaroon.AllowedBuckets{All: true})
			require.NoError(t, err)
			for _, b := range bList.Items {
				// this is a number. See earlier in test where bucket is created
				elems := strings.Split(b.Name, "-")
				n, err := strconv.Atoi(elems[1])
				require.NoError(t, err)
				b.Placement = storj.PlacementConstraint(n)
				_, err = db.Buckets().UpdateBucket(ctx, b)
				require.NoError(t, err)
			}
			page, err := usageRollups.GetBucketTotals(ctx, planet.Uplinks[0].Projects[0].ID, accounting.BucketUsageCursor{Limit: 100, Page: 1}, since, now)
			require.NoError(t, err)
			require.NotNil(t, page)
			for _, b := range page.BucketUsages {
				// this is a number. See earlier in test where bucket is created
				elems := strings.Split(b.BucketName, "-")
				n, err := strconv.Atoi(elems[1])
				require.NoError(t, err)
				require.True(t, int(b.DefaultPlacement) == n)
			}
		})
	})
}

func TestGetBucketTotals(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		now := time.Now().UTC()

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		previousMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		twoMonthsAgoStart := time.Date(now.Year(), now.Month()-2, 1, 0, 0, 0, 0, time.UTC)

		upl := planet.Uplinks[0]
		projectID := upl.Projects[0].ID

		buckets := []struct {
			name           string
			createdAt      time.Time
			currentBW      int64 // current month bandwidth
			previousBW     int64 // previous month bandwidth
			twoMonthsAgoBW int64 // two months ago bandwidth
			storage        int64
		}{
			{"bucket-0", currentMonthStart.Add(2 * 24 * time.Hour), 1000, 2000, 3000, 10000},
			{"bucket-1", currentMonthStart.Add(-60 * 24 * time.Hour), 4000, 5000, 6000, 11000},
			{"bucket-2", currentMonthStart.Add(-15 * 24 * time.Hour), 7000, 8000, 9000, 12000},
			{"bucket-3", currentMonthStart.Add(-5 * 24 * time.Hour), 0, 0, 0, 0},
		}

		// Create buckets and add test data.
		for _, b := range buckets {
			require.NoError(t, upl.TestingCreateBucket(ctx, sat, b.name))

			require.NoError(t, db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(b.name),
				pb.PieceAction_GET, b.currentBW, 0, currentMonthStart.Add(24*time.Hour)))
			require.NoError(t, db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(b.name),
				pb.PieceAction_GET, b.previousBW, 0, previousMonthStart.Add(24*time.Hour)))
			require.NoError(t, db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(b.name),
				pb.PieceAction_GET, b.twoMonthsAgoBW, 0, twoMonthsAgoStart.Add(24*time.Hour)))

			require.NoError(t, db.Orders().UpdateBucketBandwidthInline(ctx, projectID, []byte(b.name),
				pb.PieceAction_GET, b.currentBW/10, currentMonthStart.Add(24*time.Hour)))

			bucketLoc := metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: metabase.BucketName(b.name),
			}
			tally := &accounting.BucketTally{
				BucketLocation: bucketLoc,
				TotalBytes:     b.storage,
			}
			bucketTallies := map[metabase.BucketLocation]*accounting.BucketTally{bucketLoc: tally}

			require.NoError(t, db.ProjectAccounting().SaveTallies(ctx, currentMonthStart.Add(24*time.Hour), bucketTallies))
		}

		usageRollups := db.ProjectAccounting()

		// If 'now' is within the first 24h of the new month, bump it forward
		// so that all of our “current‐month” test data (which is written at
		// currentMonthStart.Add(24h)) falls before this new 'now'.
		if now.Before(currentMonthStart.Add(24 * time.Hour)) {
			now = currentMonthStart.Add(48 * time.Hour) // bump to day 3.
		}

		listSince, listBefore := twoMonthsAgoStart, now

		t.Run("Basic functionality", func(t *testing.T) {
			cursor := accounting.BucketUsageCursor{Limit: 10, Page: 1}

			totals, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.Equal(t, uint64(4), totals.TotalCount)
			require.Equal(t, uint(1), totals.CurrentPage)
			require.Equal(t, uint(1), totals.PageCount)
			require.Len(t, totals.BucketUsages, 4)

			names := make(map[string]struct{}, 4)
			for _, u := range totals.BucketUsages {
				names[u.BucketName] = struct{}{}
			}
			require.Contains(t, names, "bucket-0")
			require.Contains(t, names, "bucket-1")
			require.Contains(t, names, "bucket-2")
			require.Contains(t, names, "bucket-3")
		})

		t.Run("Pagination", func(t *testing.T) {
			cursor := accounting.BucketUsageCursor{Limit: 2, Page: 1}

			t1, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.Equal(t, uint64(4), t1.TotalCount)
			require.Equal(t, uint(1), t1.CurrentPage)
			require.Equal(t, uint(2), t1.PageCount)
			require.Len(t, t1.BucketUsages, 2)

			cursor.Page = 2
			t2, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.Equal(t, uint64(4), t2.TotalCount)
			require.Equal(t, uint(2), t2.CurrentPage)
			require.Equal(t, uint(2), t2.PageCount)
			require.Len(t, t2.BucketUsages, 2)
		})

		t.Run("Search functionality", func(t *testing.T) {
			cursor := accounting.BucketUsageCursor{Limit: 10, Page: 1, Search: "bucket-2"}

			totals, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.Equal(t, uint64(1), totals.TotalCount)
			require.Len(t, totals.BucketUsages, 1)
			require.Equal(t, "bucket-2", totals.BucketUsages[0].BucketName)

			cursor.Search = "buck%' OR 'x' != '"
			totals, err = usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.Equal(t, uint64(0), totals.TotalCount)
			require.Len(t, totals.BucketUsages, 0)
		})

		t.Run("Usage calculated since beginning of month", func(t *testing.T) {
			cursor := accounting.BucketUsageCursor{Limit: 10, Page: 1}

			// current month: since = first of this month, before = now.
			totals, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, currentMonthStart, now)
			require.NoError(t, err)

			for _, usage := range totals.BucketUsages {
				var expectedBW, expectedInline, expectedSt int64
				for _, b := range buckets {
					if b.name == usage.BucketName {
						expectedBW = b.currentBW
						expectedInline = b.currentBW / 10
						expectedSt = b.storage
					}
				}

				expectedEgress := float64(expectedBW+expectedInline) / float64(1<<30)
				expectedStorage := float64(expectedSt) / float64(1<<30)
				require.InDeltaf(t, expectedEgress, usage.Egress, 1e-6,
					"Current month egress for bucket %s incorrect. Expected: %v, Got: %v",
					usage.BucketName, expectedEgress, usage.Egress)
				require.InDeltaf(t, expectedStorage, usage.Storage, 1e-6,
					"Current month storage for bucket %s incorrect. Expected: %v, Got: %v",
					usage.BucketName, expectedStorage, usage.Storage)
			}

			// previous month: since = first of previous, before = last instant of previous
			prevLast := currentMonthStart.Add(-time.Second)
			t2, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, previousMonthStart, prevLast)
			require.NoError(t, err)

			for _, usage := range t2.BucketUsages {
				var expectedBW int64
				for _, b := range buckets {
					if b.name == usage.BucketName {
						expectedBW = b.previousBW
					}
				}

				expectedEgress := float64(expectedBW) / float64(1<<30)
				require.InDeltaf(t, expectedEgress, usage.Egress, 1e-6,
					"Previous month egress for bucket %s incorrect. Expected: %v, Got: %v",
					usage.BucketName, expectedEgress, usage.Egress)
			}

			// two months ago: since = first of that month, before = last instant of that month
			twoLast := previousMonthStart.Add(-time.Second)
			t3, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, twoMonthsAgoStart, twoLast)

			require.NoError(t, err)
			for _, usage := range t3.BucketUsages {
				var expectedBW int64
				for _, b := range buckets {
					if b.name == usage.BucketName {
						expectedBW = b.twoMonthsAgoBW
					}
				}

				expectedEgress := float64(expectedBW) / float64(1<<30)
				require.InDeltaf(t, expectedEgress, usage.Egress, 1e-6,
					"Two months ago egress for bucket %s incorrect. Expected: %v, Got: %v",
					usage.BucketName, expectedEgress, usage.Egress)
			}
		})

		t.Run("Error conditions", func(t *testing.T) {
			// page zero
			_, err := usageRollups.GetBucketTotals(ctx, projectID, accounting.BucketUsageCursor{Limit: 10, Page: 0}, twoMonthsAgoStart, now)
			require.Error(t, err)
			require.Contains(t, err.Error(), "page can not be 0")

			// page out of range
			_, err = usageRollups.GetBucketTotals(ctx, projectID, accounting.BucketUsageCursor{Limit: 10, Page: 999}, twoMonthsAgoStart, now)
			require.Error(t, err)
			require.Contains(t, err.Error(), "page is out of range")

			// limit capped
			largeCursor := accounting.BucketUsageCursor{Limit: 1000000, Page: 1}
			totals, err := usageRollups.GetBucketTotals(ctx, projectID, largeCursor, twoMonthsAgoStart, now)
			require.NoError(t, err)
			require.Less(t, totals.Limit, uint(1000000))
		})

		t.Run("CreatorEmail visibility and search", func(t *testing.T) {
			memberUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Member User",
				Email:    "member@example.com",
			}, 1)
			require.NoError(t, err)

			member, err := db.Console().ProjectMembers().Insert(ctx, memberUser.ID, projectID, console.RoleMember)
			require.NoError(t, err)

			bucketName := "member-bucket"

			_, err = db.Buckets().CreateBucket(ctx, satbuckets.Bucket{
				Name:       bucketName,
				ProjectID:  projectID,
				Versioning: satbuckets.Unversioned,
				CreatedBy:  memberUser.ID,
			})
			require.NoError(t, err)

			cursor := accounting.BucketUsageCursor{Limit: 10, Page: 1, Search: memberUser.Email}
			totals, err := usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)
			require.NotNil(t, totals)
			require.NotNil(t, totals.BucketUsages)
			require.Len(t, totals.BucketUsages, 1)
			require.Equal(t, bucketName, totals.BucketUsages[0].BucketName)
			require.Equal(t, memberUser.Email, totals.BucketUsages[0].CreatorEmail)

			err = db.Console().ProjectMembers().Delete(ctx, member.MemberID, projectID)
			require.NoError(t, err)

			cursor.Search = ""
			totals, err = usageRollups.GetBucketTotals(ctx, projectID, cursor, listSince, listBefore)
			require.NoError(t, err)

			var found bool
			for _, u := range totals.BucketUsages {
				if u.BucketName == bucketName {
					found = true
					require.Equal(t, "", u.CreatorEmail, "ex-member's email must be hidden")
				}
			}
			require.True(t, found)
		})
	})
}

func TestProjectUsage_FreeUsedStorageSpace(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		accounting := planet.Satellites[0].Accounting
		project := planet.Uplinks[0].Projects[0]

		accounting.Tally.Loop.Pause()

		// set custom usage limit for project
		customLimit := 100 * memory.KiB
		err := satDB.ProjectAccounting().UpdateProjectUsageLimit(ctx, project.ID, customLimit)
		require.NoError(t, err)

		data := testrand.Bytes(50 * memory.KiB)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "1", data)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)

		usage, err := accounting.ProjectUsage.GetProjectStorageTotals(ctx, project.ID)
		require.NoError(t, err)
		require.EqualValues(t, segments[0].EncryptedSize, usage)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "2", data)
		require.NoError(t, err)

		// we used limit so we should get error
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "3", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "storage limit")

		// delete object to free some storage space
		err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "bucket", "2")
		require.NoError(t, err)

		// we need to wait for tally to update storage usage after delete
		accounting.Tally.Loop.TriggerWait()

		// try to upload object as we have free space now
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "4", data)
		require.NoError(t, err)

		// should fail because we once again used space up to limit
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "2", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "storage limit")
	})
}

func TestProjectUsageBandwidthResetAfter3days(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits.Storage.Free = 1 * memory.MB
				config.Console.UsageLimits.Bandwidth.Free = 1 * memory.MB
				config.LiveAccounting.AsOfSystemInterval = -time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		orderDB := planet.Satellites[0].DB.Orders()
		bucket := metabase.BucketLocation{ProjectID: planet.Uplinks[0].Projects[0].ID, BucketName: "testbucket"}
		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		now := time.Now()

		allocationTime := time.Date(now.Year(), now.Month(), 2, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())

		amount := 1 * memory.MB.Int64()
		err := orderDB.UpdateBucketBandwidthAllocation(ctx, bucket.ProjectID, []byte(bucket.BucketName), pb.PieceAction_GET, amount, allocationTime)
		require.NoError(t, err)

		beforeResetDay := allocationTime.Add(2 * time.Hour * 24)
		resetDay := allocationTime.Add(3 * time.Hour * 24)
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())

		for _, tt := range []struct {
			description     string
			now             time.Time
			expectedExceeds bool
		}{
			{"allocation day", allocationTime, true},
			{"day before reset", beforeResetDay, true},
			{"reset day", resetDay, false},
			{"end of month", endOfMonth, false},
		} {
			projectUsage.SetNow(func() time.Time {
				return tt.now
			})

			actualExceeded, _, err := projectUsage.ExceedsBandwidthUsage(ctx, accounting.ProjectLimits{ProjectID: bucket.ProjectID})
			require.NoError(t, err)
			require.Equal(t, tt.expectedExceeds, actualExceeded, tt.description)
		}
	})

}

func TestProjectUsage_ResetLimitsFirstDayOfNextMonth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		project := planet.Uplinks[0].Projects[0]

		planet.Satellites[0].Orders.Chore.Loop.Pause()

		// set custom usage limit for project
		customLimit := 100 * memory.KiB
		err := satDB.ProjectAccounting().UpdateProjectUsageLimit(ctx, project.ID, customLimit)
		require.NoError(t, err)
		err = satDB.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, project.ID, customLimit)
		require.NoError(t, err)

		data := testrand.Bytes(customLimit)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path1", data)
		require.NoError(t, err)

		// verify that storage limit is all used
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path2", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "storage limit")

		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
		tomorrow := time.Now().Add(24 * time.Hour)
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)
		}

		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		// verify that bandwidth limit is all used
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.Error(t, err)
		if !errors.Is(err, uplink.ErrBandwidthLimitExceeded) {
			t.Fatal("Expected resource exhausted error. Got", err.Error())
		}

		now := time.Now()
		planet.Satellites[0].API.Accounting.ProjectUsage.SetNow(func() time.Time {
			return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})

		// verify that storage limit is all used even at the new billing cycle
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path3", data)
		require.Error(t, err)
		// TODO should compare to uplink API error when exposed
		require.Contains(t, strings.ToLower(err.Error()), "storage limit")

		// verify that new billing cycle reset bandwidth limit
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.NoError(t, err)
	})
}

func TestProjectUsage_BandwidthCache(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project := planet.Uplinks[0].Projects[0]
		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		badwidthUsed := int64(42)

		err := projectUsage.UpdateProjectBandwidthUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, badwidthUsed)
		require.NoError(t, err)

		// verify cache key creation.
		fromCache, err := projectUsage.GetProjectBandwidthUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, badwidthUsed, fromCache)

		// verify cache key increment.
		increment := int64(10)
		err = projectUsage.UpdateProjectBandwidthUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, increment)
		require.NoError(t, err)
		fromCache, err = projectUsage.GetProjectBandwidthUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, badwidthUsed+increment, fromCache)
	})
}

func TestProjectUsage_StorageSegmentCache(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project := planet.Uplinks[0].Projects[0]
		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		segmentsUsed := int64(42)

		storage, segments, err := projectUsage.GetProjectStorageAndSegmentUsage(ctx, testrand.UUID())
		require.NoError(t, err)
		require.Zero(t, storage)
		require.Zero(t, segments)

		err = projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, 0, segmentsUsed)
		require.NoError(t, err)

		// verify cache key creation.
		_, fromCache, err := projectUsage.GetProjectStorageAndSegmentUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, segmentsUsed, fromCache)

		// verify cache key increment.
		increment := int64(10)
		err = projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, 0, increment)
		require.NoError(t, err)
		_, fromCache, err = projectUsage.GetProjectStorageAndSegmentUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, segmentsUsed+increment, fromCache)
	})
}

func TestProjectUsage_UpdateStorageAndSegmentCache(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project := planet.Uplinks[0].Projects[0]
		projectUsage := planet.Satellites[0].Accounting.ProjectUsage

		storage, segments, err := projectUsage.GetProjectStorageAndSegmentUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Zero(t, storage)
		require.Zero(t, segments)

		storageUsed := int64(100)
		segmentsUsed := int64(2)

		err = projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, storageUsed, segmentsUsed)
		require.NoError(t, err)

		storage, segments, err = projectUsage.GetProjectStorageAndSegmentUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, storageUsed, storage)
		require.Equal(t, segmentsUsed, segments)

		increment := int64(10)

		err = projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, accounting.ProjectLimits{ProjectID: project.ID}, increment, increment)
		require.NoError(t, err)

		storage, segments, err = projectUsage.GetProjectStorageAndSegmentUsage(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, storageUsed+increment, storage)
		require.Equal(t, segmentsUsed+increment, segments)
	})
}

func TestProjectUsage_BandwidthDownloadLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDB := planet.Satellites[0].DB
		acctDB := satDB.ProjectAccounting()

		now := time.Now()
		project := planet.Uplinks[0].Projects[0]

		// set custom bandwidth limit for project 512 Kb
		bandwidthLimit := 500 * memory.KiB
		err := acctDB.UpdateProjectBandwidthLimit(ctx, project.ID, bandwidthLimit)
		require.NoError(t, err)

		dataSize := 100 * memory.KiB
		data := testrand.Bytes(dataSize)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path1", data)
		require.NoError(t, err)

		// Let's calculate the maximum number of iterations
		// Bandwidth limit: 512 Kb
		// Pointer Segment size: 103.936 Kb
		// (5 x 103.936) = 519.68
		// We'll be able to download 5X before reach the limit.
		for i := 0; i < 5; i++ {
			_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
			require.NoError(t, err)
		}

		// send orders so we get the egress settled
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
		tomorrow := time.Now().Add(24 * time.Hour)
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, tomorrow)
		}

		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		// An extra download should return 'Exceeded Usage Limit' error
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.Error(t, err)
		require.True(t, errors.Is(err, uplink.ErrBandwidthLimitExceeded))
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		// Simulate new billing cycle (next month)
		planet.Satellites[0].API.Accounting.ProjectUsage.SetNow(func() time.Time {
			return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})

		// Should not return an error since it's a new month
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.NoError(t, err)
	})
}

func TestProjectUsage_BandwidthDeadAllocation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Orders.Chore.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		now := time.Now()
		project := planet.Uplinks[0].Projects[0]

		dataSize := 4 * memory.MiB
		data := testrand.Bytes(dataSize)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path1", data)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		pieceSize := segments[0].PieceSize()

		reader, cleanFn, err := planet.Uplinks[0].DownloadStream(ctx, planet.Satellites[0], "testbucket", "test/path1")
		require.NoError(t, err)

		// partially download the object
		p := make([]byte, 1*memory.MiB)
		total, err := io.ReadFull(reader, p)
		require.NoError(t, err)
		require.Equal(t, total, len(p))
		require.NoError(t, reader.Close())
		require.NoError(t, cleanFn())

		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		bandwidthUsage, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx,
			project.ID, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)

		require.Equal(t, int64(planet.Satellites[0].Orders.Service.DownloadNodes(segments[0].Redundancy))*pieceSize, bandwidthUsage)

		initialBandwidthUsage := bandwidthUsage
		var updatedBandwidthUsage int64
		deadSum := int64(0)
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, now.Add(2*time.Hour))
			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			archivedOrders, err := storageNode.OrdersStore.ListArchived()
			require.NoError(t, err)

			// submit orders storage node by storage node
			for _, order := range archivedOrders {
				if order.Status == snorders.StatusAccepted && order.Limit.Action == pb.PieceAction_GET {
					deadSum += order.Limit.Limit - order.Order.Amount
				}
			}
			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

			// new bandwidth allocation should be decreased by dead amount
			updatedBandwidthUsage, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx,
				project.ID, now.Year(), now.Month(), now.Day(), 0)
			require.NoError(t, err)

			require.Equal(t, bandwidthUsage-deadSum, updatedBandwidthUsage)
		}

		_, _, dead, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectDailyBandwidth(ctx,
			project.ID, now.Year(), now.Month(), now.Day())
		require.NoError(t, err)
		require.NotZero(t, dead)
		require.Equal(t, initialBandwidthUsage, updatedBandwidthUsage+dead)
	})
}

// Check if doing a copy is not affecting monthly bandwidth usage. Doing
// server-side copy should not reduce users monthly bandwidth limit.
func TestProjectBandwidthUsageWithCopies(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// this effectively disable live accounting cache
				config.LiveAccounting.BandwidthCacheTTL = -1
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		testObjects := map[string]memory.Size{
			"inline": 1 * memory.KiB,
			"remote": 10 * memory.KiB,
		}

		for name, size := range testObjects {
			expectedData := testrand.Bytes(size)

			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", name, expectedData)
			require.NoError(t, err)

			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", name)
			require.NoError(t, err)
			require.Equal(t, data, expectedData)
		}

		// flush allocated bandwidth to DB as we will use DB directly without cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		projectID := planet.Uplinks[0].Projects[0].ID
		now := time.Now()
		expectedBandwidth, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		require.NotZero(t, expectedBandwidth)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// make copies
		for name := range testObjects {
			_, err = project.CopyObject(ctx, "testbucket", name, "testbucket", name+"copy", nil)
			require.NoError(t, err)
		}

		// flush allocated bandwidth to DB as we will use DB directly without cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		bandwidth, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		require.Equal(t, expectedBandwidth, bandwidth)

		// download copies to verify that used bandwidth will be increased
		for name := range testObjects {
			_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", name+"copy")
			require.NoError(t, err)
		}

		// flush allocated bandwidth to DB as we will use DB directly without cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		bandwidth, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		require.Less(t, expectedBandwidth, bandwidth)

		// last bandwidth read becomes new expectedBandwidth
		expectedBandwidth = bandwidth

		// delete ancestors
		for name := range testObjects {
			_, err = project.DeleteObject(ctx, "testbucket", name)
			require.NoError(t, err)
		}

		// flush allocated bandwidth to DB as we will use DB directly without cache
		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

		bandwidth, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day(), 0)
		require.NoError(t, err)
		require.Equal(t, expectedBandwidth, bandwidth)
	})
}
