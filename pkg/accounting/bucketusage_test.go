// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBucketUsage(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		count := 50
		now := time.Now()
		ctx := testcontext.New(t)

		usageDB := db.Console().BucketUsage()
		if usageDB == nil {
			t.Fail()
		}

		bucketID, err := uuid.New()
		if err != nil {
			t.Fail()
		}

		compareTallies := func(t *testing.T, expected *accounting.BucketTally, actual *accounting.BucketTally) {
			assert.Equal(t, expected.BucketID, actual.BucketID)
			assert.Equal(t, expected.TallyEndTime.Unix(), actual.TallyEndTime.Unix())
			assert.Equal(t, expected.RemoteStoredData, actual.RemoteStoredData)
			assert.Equal(t, expected.InlineStoredData, actual.InlineStoredData)
			assert.Equal(t, expected.RemoteSegments, actual.RemoteSegments)
			assert.Equal(t, expected.InlineSegments, actual.InlineSegments)
			assert.Equal(t, expected.Objects, actual.Objects)
			assert.Equal(t, expected.MetadataSize, actual.MetadataSize)
			assert.Equal(t, expected.RepairEgress, actual.RepairEgress)
			assert.Equal(t, expected.GetEgress, actual.GetEgress)
			assert.Equal(t, expected.AuditEgress, actual.AuditEgress)
		}

		var tally *accounting.BucketTally
		t.Run("add tally", func(t *testing.T) {
			var err error
			data := accounting.BucketTally{
				BucketID:         *bucketID,
				TallyEndTime:    now,
				RemoteStoredData: 5,
				InlineStoredData: 6,
				RemoteSegments:   7,
				InlineSegments:   8,
				Objects:          9,
				MetadataSize:     10,
				RepairEgress:     11,
				GetEgress:        12,
				AuditEgress:      13,
			}

			tally, err = usageDB.Create(ctx, data)
			assert.NotNil(t, tally)
			assert.NoError(t, err)
			compareTallies(t, &data, tally)
		})

		t.Run("get tally", func(t *testing.T) {
			result, err := usageDB.Get(ctx, tally.ID)
			assert.NoError(t, err)
			compareTallies(t, tally, result)
		})

		t.Run("delete tally", func(t *testing.T) {
			err := usageDB.Delete(ctx, tally.ID)
			assert.NoError(t, err)
		})

		var addedTallies []accounting.BucketTally
		t.Run("add tallies", func(t *testing.T) {
			for i := 0; i < count; i++ {
				data := accounting.BucketTally{
					BucketID:         *bucketID,
					TallyEndTime:    now.Add(time.Hour * time.Duration(i+1)),
					RemoteStoredData: uint64(i),
					InlineStoredData: uint64(i + 1),
					RemoteSegments:   7,
					InlineSegments:   8,
					Objects:          9,
					MetadataSize:     10,
					RepairEgress:     11,
					GetEgress:        12,
					AuditEgress:      13,
				}

				tally, err := usageDB.Create(ctx, data)
				assert.NotNil(t, tally)
				assert.NoError(t, err)

				addedTallies = append(addedTallies, *tally)
			}
		})

		t.Run("retrieve tally", func(t *testing.T) {
			t.Run("first 30 backward", func(t *testing.T) {
				cursor := &accounting.BucketTallyCursor{
					BucketID: *bucketID,
					Before:   now.Add(time.Hour * 30),
					Order:    accounting.Desc,
					PageSize: 10,
				}

				var pagedTallies []accounting.BucketTally
				for {
					tallies, err := usageDB.GetPaged(ctx, cursor)
					assert.NoError(t, err)
					assert.NotNil(t, tallies)
					assert.True(t, len(tallies) <= 10)

					pagedTallies = append(pagedTallies, tallies...)

					if cursor.Next == nil {
						break
					}
					cursor = cursor.Next
				}

				testSlice := addedTallies[:30]
				for i := range pagedTallies {
					assert.Equal(t, testSlice[i].ID, pagedTallies[29-i].ID)
					compareTallies(t, &testSlice[i], &pagedTallies[29-i])
				}
			})

			t.Run("last 30 forward", func(t *testing.T) {
				cursor := &accounting.BucketTallyCursor{
					BucketID: *bucketID,
					After:    now.Add(time.Hour * 20),
					Before:   now.Add(time.Hour * time.Duration(count+1)),
					Order:    accounting.Asc,
					PageSize: 10,
				}

				var pagedTallies []accounting.BucketTally
				for {
					tallies, err := usageDB.GetPaged(ctx, cursor)
					assert.NoError(t, err)
					assert.NotNil(t, tallies)
					assert.True(t, len(tallies) <= 10)

					pagedTallies = append(pagedTallies, tallies...)

					if cursor.Next == nil {
						break
					}
					cursor = cursor.Next
				}

				testSlice := addedTallies[20:]
				for i := range pagedTallies {
					assert.Equal(t, testSlice[i].ID, pagedTallies[i].ID)
					compareTallies(t, &testSlice[i], &pagedTallies[i])
				}
			})
		})
	})
}
