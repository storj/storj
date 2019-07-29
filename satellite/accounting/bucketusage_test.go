// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
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

		bucketID := testrand.UUID()

		compareRollups := func(t *testing.T, expected *accounting.BucketRollup, actual *accounting.BucketRollup) {
			assert.Equal(t, expected.BucketID, actual.BucketID)
			assert.Equal(t, expected.RollupEndTime.Unix(), actual.RollupEndTime.Unix())
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

		var rollup *accounting.BucketRollup
		t.Run("add rollup", func(t *testing.T) {
			var err error
			data := accounting.BucketRollup{
				BucketID:         bucketID,
				RollupEndTime:    now,
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

			rollup, err = usageDB.Create(ctx, data)
			assert.NotNil(t, rollup)
			assert.NoError(t, err)
			compareRollups(t, &data, rollup)
		})

		t.Run("get rollup", func(t *testing.T) {
			result, err := usageDB.Get(ctx, rollup.ID)
			assert.NoError(t, err)
			compareRollups(t, rollup, result)
		})

		t.Run("delete rollup", func(t *testing.T) {
			err := usageDB.Delete(ctx, rollup.ID)
			assert.NoError(t, err)
		})

		var addedRollups []accounting.BucketRollup
		t.Run("add rollups", func(t *testing.T) {
			for i := 0; i < count; i++ {
				data := accounting.BucketRollup{
					BucketID:         bucketID,
					RollupEndTime:    now.Add(time.Hour * time.Duration(i+1)),
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

				rollup, err := usageDB.Create(ctx, data)
				assert.NotNil(t, rollup)
				assert.NoError(t, err)

				addedRollups = append(addedRollups, *rollup)
			}
		})

		t.Run("retrieve rollup", func(t *testing.T) {
			t.Run("first 30 backward", func(t *testing.T) {
				cursor := &accounting.BucketRollupCursor{
					BucketID: bucketID,
					Before:   now.Add(time.Hour * 30),
					Order:    accounting.Desc,
					PageSize: 10,
				}

				var pagedRollups []accounting.BucketRollup
				for {
					rollups, err := usageDB.GetPaged(ctx, cursor)
					assert.NoError(t, err)
					assert.NotNil(t, rollups)
					assert.True(t, len(rollups) <= 10)

					pagedRollups = append(pagedRollups, rollups...)

					if cursor.Next == nil {
						break
					}
					cursor = cursor.Next
				}

				testSlice := addedRollups[:30]
				for i := range pagedRollups {
					assert.Equal(t, testSlice[i].ID, pagedRollups[29-i].ID)
					compareRollups(t, &testSlice[i], &pagedRollups[29-i])
				}
			})

			t.Run("last 30 forward", func(t *testing.T) {
				cursor := &accounting.BucketRollupCursor{
					BucketID: bucketID,
					After:    now.Add(time.Hour * 20),
					Before:   now.Add(time.Hour * time.Duration(count+1)),
					Order:    accounting.Asc,
					PageSize: 10,
				}

				var pagedRollups []accounting.BucketRollup
				for {
					rollups, err := usageDB.GetPaged(ctx, cursor)
					assert.NoError(t, err)
					assert.NotNil(t, rollups)
					assert.True(t, len(rollups) <= 10)

					pagedRollups = append(pagedRollups, rollups...)

					if cursor.Next == nil {
						break
					}
					cursor = cursor.Next
				}

				testSlice := addedRollups[20:]
				for i := range pagedRollups {
					assert.Equal(t, testSlice[i].ID, pagedRollups[i].ID)
					compareRollups(t, &testSlice[i], &pagedRollups[i])
				}
			})
		})
	})
}
