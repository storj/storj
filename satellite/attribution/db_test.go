// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package attribution_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		attributionDB := db.Attribution()

		newUUID := func() uuid.UUID {
			v, err := uuid.New()
			require.NoError(t, err)
			return *v
		}

		project1, project2 := newUUID(), newUUID()
		partner1, partner2 := newUUID(), newUUID()

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

func TestQueryValueAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		attributionDB := db.Attribution()
		projectAccoutingDB := db.ProjectAccounting()

		now := time.Now().UTC()
		hoursOfData := 28
		var remoteSize int64 = 5368709120 // 5GB
		var inlineSize int64 = 1024
		dataStart := time.Date(now.Year(), now.Month(), now.Day(), -2, 0, 0, 0, now.Location())
		dataInterval := time.Date(now.Year(), now.Month(), now.Day(), -2, -1, 0, 0, now.Location())
		bwStart := time.Date(now.Year(), now.Month(), now.Day(), -2, 0, 0, 0, now.Location())
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		hours := int64(end.Sub(start).Hours())

		fmt.Printf("Dates %v %v %v\n", start, dataStart, bwStart)

		newUUID := func() uuid.UUID {
			v, err := uuid.New()
			require.NoError(t, err)
			return *v
		}

		project1 := newUUID()
		partner1 := newUUID()
		bucket1 := []byte("alpha")

		infos := []*attribution.Info{
			{project1, bucket1, partner1, time.Time{}},
		}

		for _, info := range infos {
			_, err := attributionDB.Insert(ctx, info)
			require.NoError(t, err)
		}

		dataCounter := 0
		var expectedRemoteBytes int64 = 0
		var expectedInlineBytes int64 = 0
		for i := 0; i < hoursOfData; i++ {

			dataCounter++
			dataInterval = dataInterval.Add(time.Minute * 30)
			//fmt.Printf("EEEE dataInterval1 %v\n", dataInterval)
			tally1, err := createTallyData(ctx, projectAccoutingDB, project1, bucket1, remoteSize*int64(dataCounter), inlineSize*int64(dataCounter), dataInterval)
			require.NoError(t, err)

			dataCounter++
			dataInterval = dataInterval.Add(time.Minute * 30)
			//fmt.Printf("EEEE dataInterval2 %v\n", dataInterval)
			tally2, err := createTallyData(ctx, projectAccoutingDB, project1, bucket1, remoteSize*int64(dataCounter), inlineSize*int64(dataCounter), dataInterval)
			require.NoError(t, err)

			if (dataInterval.After(start) || dataInterval.Equal(start)) && dataInterval.Before(end) {
				expectedRemoteBytes += tally2.RemoteBytes
				expectedInlineBytes += tally2.InlineBytes
			}
			fmt.Printf("1: %v, %v %v\n2: %v, %v%v\n", tally1.RemoteBytes, tally1.InlineBytes, tally1.IntervalStart, tally2.RemoteBytes, tally2.InlineBytes, tally2.IntervalStart)
		}

		results, err := attributionDB.QueryValueAttribution(ctx, partner1, start, end)
		require.NoError(t, err)
		for _, r := range results {
			projectID, _ := bytesToUUID(r.ProjectID)
			fmt.Printf("EEEE: %v, %v, %v, %v\n", projectID, string(r.BucketName), r.RemoteBytesPerHour, r.InlineBytesPerHour)
			assert.Equal(t, project1[:], r.ProjectID)
			assert.Equal(t, bucket1, r.BucketName)
			assert.Equal(t, float64(expectedRemoteBytes/hours), r.RemoteBytesPerHour)
			assert.Equal(t, float64(expectedInlineBytes/hours), r.InlineBytesPerHour)
			fmt.Printf("EEEE: %v, %v\n", float64(expectedRemoteBytes/hours) == r.RemoteBytesPerHour, float64(expectedInlineBytes/hours) == r.InlineBytesPerHour)

		}
	})
}

func createTallyData(ctx *testcontext.Context, projectAccoutingDB accounting.ProjectAccounting, projectID uuid.UUID, bucket []byte, remote int64, inline int64, dataIntervalStart time.Time) (_ accounting.BucketStorageTally, err error) {
	tally := accounting.BucketStorageTally{
		BucketName:    string(bucket),
		ProjectID:     projectID,
		IntervalStart: dataIntervalStart,

		InlineSegmentCount: 0,
		RemoteSegmentCount: 0,

		ObjectCount: 0,

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

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}
