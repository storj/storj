// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package attribution_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	remoteSize int64 = 5368709120
	inlineSize int64 = 1024
	egressSize int64 = 2048
)

type AttributionTestData struct {
	name       string
	partnerID  uuid.UUID
	projectID  uuid.UUID
	bucketName []byte
	bucketID   []byte

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

	dataCounter         int
	expectedRemoteBytes int64
	expectedInlineBytes int64
	expectedEgress      int64
}

func (testData *AttributionTestData) init() {
	testData.bucketID = []byte(testData.projectID.String() + "/" + string(testData.bucketName))
	testData.hours = int64(testData.end.Sub(testData.start).Hours())
	testData.hoursOfData = int(testData.hours) + (testData.padding * 2)
	testData.dataInterval = time.Date(testData.start.Year(), testData.start.Month(), testData.start.Day(), -testData.padding, -1, 0, 0, testData.start.Location())
	testData.bwStart = time.Date(testData.start.Year(), testData.start.Month(), testData.start.Day(), -testData.padding, 0, 0, 0, testData.start.Location())
}

func TestDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		attributionDB := db.Attribution()

		project1, project2 := testrand.UUID(), testrand.UUID()
		partner1, partner2 := testrand.UUID(), testrand.UUID()

		infos := []*attribution.Info{
			{project1, []byte("alpha"), partner1, time.Time{}},
			{project1, []byte("beta"), partner2, time.Time{}},
			{project2, []byte("alpha"), partner2, time.Time{}},
			{project2, []byte("beta"), partner1, time.Time{}},
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
			assert.Equal(t, info.PartnerID, got.PartnerID)
		}
	})
}

func TestQueryAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		now := time.Now().UTC()

		projectID := testrand.UUID()
		partnerID := testrand.UUID()
		alphaBucket := []byte("alpha")
		betaBucket := []byte("beta")
		testData := []AttributionTestData{
			{
				name:       "new partnerID, projectID, alpha",
				partnerID:  testrand.UUID(),
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
				name:       "partnerID, new projectID, alpha",
				partnerID:  partnerID,
				projectID:  testrand.UUID(),
				bucketName: alphaBucket,

				remoteSize: remoteSize / 2,
				inlineSize: inlineSize / 2,
				egressSize: egressSize / 2,

				start:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
				end:     time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
				padding: 2,
			},
			{
				name:       "new partnerID, projectID, beta",
				partnerID:  testrand.UUID(),
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
				name:       "partnerID, new projectID, beta",
				partnerID:  partnerID,
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
			info := attribution.Info{td.projectID, td.bucketName, td.partnerID, time.Time{}}
			_, err := db.Attribution().Insert(ctx, &info)
			require.NoError(t, err)

			for i := 0; i < td.hoursOfData; i++ {
				createData(ctx, t, db, &td)
			}

			verifyData(ctx, t, db.Attribution(), &td)
		}
	})
}

func verifyData(ctx *testcontext.Context, t *testing.T, attributionDB attribution.DB, testData *AttributionTestData) {
	results, err := attributionDB.QueryAttribution(ctx, testData.partnerID, testData.start, testData.end)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(results), "Results must not be empty.")
	count := 0
	for _, r := range results {
		projectID, _ := dbutil.BytesToUUID(r.ProjectID)
		// The query returns results by partnerID, so we need to filter out by projectID
		if projectID != testData.projectID {
			continue
		}
		count++

		assert.Equal(t, testData.partnerID[:], r.PartnerID, testData.name)
		assert.Equal(t, testData.projectID[:], r.ProjectID, testData.name)
		assert.Equal(t, testData.bucketName, r.BucketName, testData.name)
		assert.Equal(t, float64(testData.expectedRemoteBytes/testData.hours), r.RemoteBytesPerHour, testData.name)
		assert.Equal(t, float64(testData.expectedInlineBytes/testData.hours), r.InlineBytesPerHour, testData.name)
		assert.Equal(t, testData.expectedEgress, r.EgressData, testData.name)
	}
	require.NotEqual(t, 0, count, "Results were returned, but did not match all of the the projectIDs.")
}

func createData(ctx *testcontext.Context, t *testing.T, db satellite.DB, testData *AttributionTestData) {
	projectAccoutingDB := db.ProjectAccounting()
	orderDB := db.Orders()

	err := orderDB.UpdateBucketBandwidthSettle(ctx, testData.projectID, testData.bucketName, pb.PieceAction_GET, testData.egressSize, testData.bwStart)
	require.NoError(t, err)

	// Only GET should be counted. So this should not effect results
	err = orderDB.UpdateBucketBandwidthSettle(ctx, testData.projectID, testData.bucketName, pb.PieceAction_GET_AUDIT, testData.egressSize, testData.bwStart)
	require.NoError(t, err)

	testData.bwStart = testData.bwStart.Add(time.Hour)

	testData.dataCounter++
	testData.dataInterval = testData.dataInterval.Add(time.Minute * 30)
	_, err = createTallyData(ctx, projectAccoutingDB, testData.projectID, testData.bucketName, testData.remoteSize*int64(testData.dataCounter), testData.inlineSize*int64(testData.dataCounter), testData.dataInterval)
	require.NoError(t, err)

	testData.dataCounter++
	testData.dataInterval = testData.dataInterval.Add(time.Minute * 30)
	tally, err := createTallyData(ctx, projectAccoutingDB, testData.projectID, testData.bucketName, testData.remoteSize*int64(testData.dataCounter), testData.inlineSize*int64(testData.dataCounter), testData.dataInterval)
	require.NoError(t, err)

	if (testData.dataInterval.After(testData.start) || testData.dataInterval.Equal(testData.start)) && testData.dataInterval.Before(testData.end) {
		testData.expectedRemoteBytes += tally.RemoteBytes
		testData.expectedInlineBytes += tally.InlineBytes
		testData.expectedEgress += testData.egressSize
	}
}

func createTallyData(ctx *testcontext.Context, projectAccoutingDB accounting.ProjectAccounting, projectID uuid.UUID, bucket []byte, remote int64, inline int64, dataIntervalStart time.Time) (_ accounting.BucketStorageTally, err error) {
	tally := accounting.BucketStorageTally{
		BucketName:    string(bucket),
		ProjectID:     projectID,
		IntervalStart: dataIntervalStart,

		ObjectCount: 0,

		InlineSegmentCount: 0,
		RemoteSegmentCount: 0,

		InlineBytes:  inline,
		RemoteBytes:  remote,
		MetadataSize: 0,
	}
	err = projectAccoutingDB.CreateStorageTally(ctx, tally)
	if err != nil {
		return accounting.BucketStorageTally{}, err
	}
	return tally, nil
}
