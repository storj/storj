// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package attribution_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	remoteSize int64 = 5368709120
	inlineSize int64 = 1024
	egressSize int64 = 2048
)

type AttributionTestData struct {
	name       string
	userAgent  []byte
	projectID  uuid.UUID
	bucketName []byte
	bucketID   []byte
	placement  *storj.PlacementConstraint

	start        time.Time
	end          time.Time
	dataInterval time.Time
	bwStart      time.Time

	hours       int64
	hoursOfData int
	padding     int

	remoteSize int64
	inlineSize int64
	egressSize int64

	dataCounter          int
	expectedByteHours    float64
	expectedSegmentHours float64
	expectedObjectHours  float64
	expectedHours        float64

	expectedEgress int64
}

func (testData *AttributionTestData) init() {
	testData.bucketID = []byte(testData.projectID.String() + "/" + string(testData.bucketName))
	testData.hours = int64(testData.end.Sub(testData.start).Hours())
	testData.hoursOfData = int(testData.hours) + (testData.padding * 2)
	testData.dataInterval = time.Date(testData.start.Year(), testData.start.Month(), testData.start.Day(), -testData.padding, -1, 0, 0, testData.start.Location())
	testData.bwStart = time.Date(testData.start.Year(), testData.start.Month(), testData.start.Day(), -testData.padding, 0, 0, 0, testData.start.Location())
}

var p0 = storj.DefaultPlacement
var p1 = storj.PlacementConstraint(1)

func TestDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		attributionDB := db.Attribution()
		project1, project2 := testrand.UUID(), testrand.UUID()
		agent1, agent2 := []byte("agent1"), []byte("agent2")

		infos := []*attribution.Info{
			{project1, []byte("alpha"), agent1, nil, time.Time{}},
			{project1, []byte("beta"), agent2, &p0, time.Time{}},
			{project2, []byte("alpha"), agent2, &p1, time.Time{}},
			{project2, []byte("beta"), agent1, nil, time.Time{}},
			{project2, []byte("gamma"), nil, &p0, time.Time{}},
		}

		for _, info := range infos {
			got, err := attributionDB.Insert(ctx, info)
			require.NoError(t, err)

			got.CreatedAt = time.Time{}
			assert.Equal(t, info, got)
		}

		for _, info := range infos {
			got, err := attributionDB.Get(ctx, info.ProjectID, info.BucketName)
			require.NoError(t, err)
			assert.Equal(t, info.UserAgent, got.UserAgent)
			assert.Equal(t, info.Placement, got.Placement)
		}

		for _, info := range infos {
			err := attributionDB.TestDelete(ctx, info.ProjectID, info.BucketName)
			require.NoError(t, err)
		}
	})
}

func TestQueryAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()

		projectID := testrand.UUID()
		userAgent := []byte("agent1")
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name:       "new userAgent, projectID, alpha",
				userAgent:  []byte("agent2"),
				projectID:  projectID,
				bucketName: alphaBucket,

				remoteSize: remoteSize,
				inlineSize: inlineSize,
				egressSize: egressSize,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "userAgent, new projectID, alpha",
				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: alphaBucket,
				placement:  &p0,

				remoteSize: remoteSize / 2,
				inlineSize: inlineSize / 2,
				egressSize: egressSize / 2,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "new userAgent, projectID, beta",
				userAgent:  []byte("agent3"),
				projectID:  projectID,
				bucketName: betaBucket,
				placement:  &p1,

				remoteSize: remoteSize / 3,
				inlineSize: inlineSize / 3,
				egressSize: egressSize / 3,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "userAgent new projectID, beta",
				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: betaBucket,

				remoteSize: remoteSize / 4,
				inlineSize: inlineSize / 4,
				egressSize: egressSize / 4,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			}}

		for _, td := range testData {
			td := td
			td.init()
			info := attribution.Info{td.projectID, td.bucketName, td.userAgent, td.placement, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)

			for i := 0; i < td.hoursOfData; i++ {
				createData(ctx, t, db, &td)
			}

			verifyData(ctx, t, db.Attribution(), &td)
		}
	})
}

func TestQueryAllAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()

		projectID := testrand.UUID()
		userAgent := []byte("agent1")
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name:       "new userAgent, projectID, alpha",
				userAgent:  []byte("agent2"),
				projectID:  projectID,
				bucketName: alphaBucket,
				placement:  &p0,

				remoteSize: remoteSize,
				inlineSize: inlineSize,
				egressSize: egressSize,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "userAgent, new projectID, alpha",
				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: alphaBucket,
				placement:  &p1,

				remoteSize: remoteSize / 2,
				inlineSize: inlineSize / 2,
				egressSize: egressSize / 2,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "new userAgent, projectID, beta",
				userAgent:  []byte("agent3"),
				projectID:  projectID,
				bucketName: betaBucket,

				remoteSize: remoteSize / 3,
				inlineSize: inlineSize / 3,
				egressSize: egressSize / 3,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "userAgent new projectID, beta",
				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: betaBucket,

				remoteSize: remoteSize / 4,
				inlineSize: inlineSize / 4,
				egressSize: egressSize / 4,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			}}

		for i, td := range testData {
			td := td
			td.init()

			info := attribution.Info{td.projectID, td.bucketName, td.userAgent, td.placement, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)
			for i := 0; i < td.hoursOfData; i++ {
				if i < td.hoursOfData/2 {
					createTallyData(ctx, t, db.ProjectAccounting(), &td)
				} else {
					createBWData(ctx, t, db.Orders(), &td)
				}
			}
			testData[i] = td
		}
		verifyAllData(ctx, t, db.Attribution(), testData, false)
	})
}

func TestQueryAllAttributionNoStorage(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()

		projectID := testrand.UUID()
		userAgent := []byte("agent1")
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name: "new userAgent, projectID, alpha",

				userAgent:  []byte("agent2"),
				projectID:  projectID,
				bucketName: alphaBucket,

				remoteSize: 0,
				inlineSize: 0,
				egressSize: egressSize,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "userAgent, new projectID, alpha",

				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: alphaBucket,

				remoteSize: 0,
				inlineSize: 0,
				egressSize: egressSize / 2,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "new userAgent, projectID, beta",

				userAgent:  []byte("agent3"),
				projectID:  projectID,
				bucketName: betaBucket,

				remoteSize: 0,
				inlineSize: 0,
				egressSize: egressSize / 3,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "userAgent new projectID, beta",

				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: betaBucket,

				remoteSize: 0,
				inlineSize: 0,
				egressSize: egressSize / 4,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			}}

		for i, td := range testData {
			td := td
			td.init()

			info := attribution.Info{td.projectID, td.bucketName, td.userAgent, td.placement, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)

			for i := 0; i < td.hoursOfData; i++ {
				createBWData(ctx, t, db.Orders(), &td)
			}
			testData[i] = td
		}
		verifyAllData(ctx, t, db.Attribution(), testData, false)
	})
}

func TestQueryAllAttributionNoBW(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()

		projectID := testrand.UUID()
		userAgent := []byte("agent1")
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name: "new userAgent, projectID, alpha",

				userAgent:  []byte("agent2"),
				projectID:  projectID,
				bucketName: alphaBucket,

				remoteSize: remoteSize,
				inlineSize: inlineSize,
				egressSize: 0,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "userAgent, new projectID, alpha",

				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: alphaBucket,

				remoteSize: remoteSize / 2,
				inlineSize: inlineSize / 2,
				egressSize: 0,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "new userAgent, projectID, beta",

				userAgent:  []byte("agent3"),
				projectID:  projectID,
				bucketName: betaBucket,

				remoteSize: remoteSize / 3,
				inlineSize: inlineSize / 3,
				egressSize: 0,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name: "userAgent new projectID, beta",

				userAgent:  userAgent,
				projectID:  testrand.UUID(),
				bucketName: betaBucket,

				remoteSize: remoteSize / 4,
				inlineSize: inlineSize / 4,
				egressSize: 0,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			}}

		for i, td := range testData {
			td := td
			td.init()

			info := attribution.Info{td.projectID, td.bucketName, td.userAgent, td.placement, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)

			for i := 0; i < td.hoursOfData; i++ {
				createTallyData(ctx, t, db.ProjectAccounting(), &td)
			}
			testData[i] = td
		}
		verifyAllData(ctx, t, db.Attribution(), testData, true)
	})
}

func TestQueryAllAttributionIgnoresNullUserAgent(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()

		projectID := testrand.UUID()
		userAgent := []byte("agent1")
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name:       "new userAgent, projectID, alpha",
				userAgent:  userAgent,
				projectID:  projectID,
				bucketName: alphaBucket,
				placement:  &p0,

				remoteSize: remoteSize,
				inlineSize: inlineSize,
				egressSize: egressSize,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "nil userAgent, projectID, beta",
				userAgent:  nil,
				projectID:  projectID,
				bucketName: betaBucket,
				placement:  &p1,

				remoteSize: remoteSize / 3,
				inlineSize: inlineSize / 3,
				egressSize: egressSize / 3,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
		}

		for i, td := range testData {
			td := td
			td.init()

			info := attribution.Info{td.projectID, td.bucketName, td.userAgent, td.placement, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)
			for i := 0; i < td.hoursOfData; i++ {
				if i < td.hoursOfData/2 {
					createTallyData(ctx, t, db.ProjectAccounting(), &td)
				} else {
					createBWData(ctx, t, db.Orders(), &td)
				}
			}
			testData[i] = td
		}
		// Only pass in first testData element. Second element is ignored in usage attribution due to nil userAgent,
		// even though usage data was created.
		verifyAllData(ctx, t, db.Attribution(), []AttributionTestData{testData[0]}, false)
	})
}

func verifyData(ctx *testcontext.Context, t *testing.T, attributionDB attribution.DB, testData *AttributionTestData) {
	results, err := attributionDB.QueryAttribution(ctx, testData.userAgent, testData.start, testData.end)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(results), "Results must not be empty.")
	count := 0
	for _, r := range results {
		projectID, err := uuid.FromBytes(r.ProjectID)
		require.NoError(t, err)

		// The query returns results by partnerID, so we need to filter out by projectID
		if projectID != testData.projectID {
			continue
		}
		count++

		assert.Equal(t, testData.userAgent, r.UserAgent, testData.name)
		assert.Equal(t, testData.projectID[:], r.ProjectID, testData.name)
		assert.Equal(t, testData.bucketName, r.BucketName, testData.name)
		assert.Equal(t, testData.expectedByteHours, r.ByteHours, testData.name)
		assert.Equal(t, testData.expectedEgress, r.EgressData, testData.name)
	}
	require.NotEqual(t, 0, count, "Results were returned, but did not match all of the projectIDs.")
}

func verifyAllData(ctx *testcontext.Context, t *testing.T, attributionDB attribution.DB, testData []AttributionTestData, storageInEveryHour bool) {
	results, err := attributionDB.QueryAllAttribution(ctx, testData[0].start, testData[0].end)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(results), "Results must not be empty.")
	count := 0
	for _, tt := range testData {
		for _, r := range results {
			projectID, err := uuid.FromBytes(r.ProjectID)
			require.NoError(t, err)

			// The query returns results by partnerID, so we need to filter out by projectID
			if projectID != tt.projectID || !bytes.Equal(r.BucketName, tt.bucketName) {
				continue
			}
			count++

			assert.Equal(t, tt.userAgent, r.UserAgent, tt.name)
			assert.Equal(t, tt.projectID[:], r.ProjectID, tt.name)
			assert.Equal(t, tt.bucketName, r.BucketName, tt.name)
			assert.Equal(t, tt.expectedByteHours, r.ByteHours, tt.name)
			assert.Equal(t, tt.expectedEgress, r.EgressData, tt.name)
			if storageInEveryHour {
				assert.Equal(t, tt.expectedHours, float64(tt.hours))
			} else {
				assert.Less(t, tt.expectedHours, float64(tt.hours))
			}
		}
	}

	require.Equal(t, len(testData), len(results))
	require.NotEqual(t, 0, count, "Results were returned, but did not match all of the projectIDs.")
}

func createData(ctx *testcontext.Context, t *testing.T, db satellite.DB, testData *AttributionTestData) {
	createBWData(ctx, t, db.Orders(), testData)
	createTallyData(ctx, t, db.ProjectAccounting(), testData)
}

func createBWData(ctx *testcontext.Context, t *testing.T, orderDB orders.DB, testData *AttributionTestData) {
	err := orderDB.UpdateBucketBandwidthSettle(ctx, testData.projectID, testData.bucketName, pb.PieceAction_GET, testData.egressSize, 0, testData.bwStart)
	require.NoError(t, err)

	// Only GET should be counted. So this should not effect results
	err = orderDB.UpdateBucketBandwidthSettle(ctx, testData.projectID, testData.bucketName, pb.PieceAction_GET_AUDIT, testData.egressSize, 0, testData.bwStart)
	require.NoError(t, err)

	if (testData.bwStart.After(testData.start) || testData.bwStart.Equal(testData.start)) && testData.bwStart.Before(testData.end) {
		testData.expectedEgress += testData.egressSize
	}
	testData.bwStart = testData.bwStart.Add(time.Hour)
}

func createTallyData(ctx *testcontext.Context, t *testing.T, projectAccoutingDB accounting.ProjectAccounting, testData *AttributionTestData) {
	testData.dataCounter++
	testData.dataInterval = testData.dataInterval.Add(time.Minute * 30)
	_, err := insertTallyData(ctx, projectAccoutingDB, testData.projectID, testData.bucketName, testData.remoteSize*int64(testData.dataCounter), testData.inlineSize*int64(testData.dataCounter), testData.dataInterval)
	require.NoError(t, err)

	// I think inserting into tallies twice is testing that only the latest row in the hour is used in the VA calculation.
	testData.dataCounter++
	testData.dataInterval = testData.dataInterval.Add(time.Minute * 30)
	tally, err := insertTallyData(ctx, projectAccoutingDB, testData.projectID, testData.bucketName, testData.remoteSize*int64(testData.dataCounter), testData.inlineSize*int64(testData.dataCounter), testData.dataInterval)
	require.NoError(t, err)

	if (testData.dataInterval.After(testData.start) || testData.dataInterval.Equal(testData.start)) && testData.dataInterval.Before(testData.end) {
		testData.expectedByteHours += float64(tally.TotalBytes)
		testData.expectedHours++
		testData.expectedSegmentHours += float64(tally.TotalSegmentCount)
		testData.expectedObjectHours += float64(tally.ObjectCount)
	}
}

func insertTallyData(ctx *testcontext.Context, projectAccoutingDB accounting.ProjectAccounting, projectID uuid.UUID, bucket []byte, remote int64, inline int64, dataIntervalStart time.Time) (_ accounting.BucketStorageTally, err error) {
	tally := accounting.BucketStorageTally{
		BucketName:    string(bucket),
		ProjectID:     projectID,
		IntervalStart: dataIntervalStart,

		ObjectCount: 1,

		TotalSegmentCount: 1,

		TotalBytes:   inline + remote,
		MetadataSize: 0,
	}
	err = projectAccoutingDB.CreateStorageTally(ctx, tally)
	if err != nil {
		return accounting.BucketStorageTally{}, err
	}
	return tally, nil
}
