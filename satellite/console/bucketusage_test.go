package console_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBucketUsageRepo(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		busages := db.Console().BucketUsages()

		compareUsages := func(t *testing.T, expected *console.BucketUsage, actual *console.BucketUsage) {
			//assert.Equal(t, expected.ID, actual.ID)
			assert.Equal(t, expected.BucketID, actual.BucketID)
			assert.Equal(t, expected.RemoteStoredData, actual.RemoteStoredData)
			assert.Equal(t, expected.InlineStoredData, actual.InlineStoredData)
			assert.Equal(t, expected.Segments, actual.Segments)
			assert.Equal(t, expected.MetadataSize, actual.MetadataSize)
			assert.Equal(t, expected.RepairEgress, actual.RepairEgress)
			assert.Equal(t, expected.GetEgress, actual.GetEgress)
			assert.Equal(t, expected.AuditEgress, actual.AuditEgress)
			assert.True(t, expected.RollupEndTime.Equal(actual.RollupEndTime))
		}

		bucketID, err := uuid.New()
		if err != nil {
			t.Fatal(err)
		}

		var usage *console.BucketUsage
		t.Run("create", func(t *testing.T) {
			var err error
			usageData := console.BucketUsage{
				BucketID:         *bucketID,
				RollupEndTime:    time.Now(),
				RemoteStoredData: 5,
				InlineStoredData: 6,
				Segments:         7,
				MetadataSize:     8,
				RepairEgress:     9,
				GetEgress:        10,
				AuditEgress:      11,
			}

			usage, err = busages.Create(ctx, usageData)

			assert.NoError(t, err)
			assert.NotNil(t, usage)
			compareUsages(t, &usageData, usage)
		})

		t.Run("get", func(t *testing.T) {
			row, err := busages.Get(ctx, usage.ID)

			assert.NoError(t, err)
			assert.NotNil(t, row)
			assert.Equal(t, usage.ID, row.ID)
			compareUsages(t, usage, row)
		})

		t.Run("delete", func(t *testing.T) {
			err := busages.Delete(ctx, usage.ID)
			assert.NoError(t, err)

			_, err = busages.Get(ctx, usage.ID)
			assert.Error(t, err)
		})

		t.Run("iterate", func(t *testing.T) {
			// keep count/limit to be natural number
			const (
				count = 50
				limit = 5
			)

			now := time.Now()

			var usages []console.BucketUsage
			for i := 0; i < count; i++ {
				usage, err := busages.Create(ctx, console.BucketUsage{
					BucketID:      *bucketID,
					Segments:      uint(i),
					RollupEndTime: now.Add(time.Hour * time.Duration(i)),
				})

				assert.NoError(t, err)
				usages = append(usages, *usage)
			}

			if len(usages) != count {
				t.Fail()
			}

			var usagesAsc []console.BucketUsage
			t.Run("get all asc", func(t *testing.T) {
				// empty cursor field should get us all of the entries
				iterator := &console.UsageIterator{
					BucketID:  *bucketID,
					Cursor:    now.Add(time.Minute * -1),
					Direction: console.Fwd,
					Limit:     limit,
				}

				opcount := 0
				for {
					opcount++

					rows, err := busages.GetByBucketID(ctx, iterator)
					assert.NoError(t, err)

					usagesAsc = append(usagesAsc, rows...)

					if iterator.Next == nil {
						break
					}
					iterator = iterator.Next
				}

				assert.Equal(t, count, len(usagesAsc))
				assert.Equal(t, count/limit+1, opcount)
			})

			var usagesDesc []console.BucketUsage
			t.Run("get all desc", func(t *testing.T) {
				iterator := &console.UsageIterator{
					BucketID:  *bucketID,
					Cursor:    usagesAsc[count-1].RollupEndTime.Add(time.Minute * 1),
					Direction: console.Bkwd,
					Limit:     limit,
				}

				opcount := 0
				for {
					opcount++

					rows, err := busages.GetByBucketID(ctx, iterator)
					assert.NoError(t, err)

					usagesDesc = append(usagesDesc, rows...)

					if iterator.Next == nil {
						break
					}
					iterator = iterator.Next
				}

				assert.Equal(t, count, len(usagesDesc))
				assert.Equal(t, count/limit+1, opcount)
			})

			t.Run("check order", func(t *testing.T) {
				for i := range usagesAsc {
					descIndex := count - 1 - i
					if descIndex < 0 {
						continue
					}

					assert.Equal(t, usagesAsc[i].ID, usagesDesc[descIndex].ID)
				}
			})

			t.Run("get after asc", func(t *testing.T) {
				iterator := &console.UsageIterator{
					BucketID:  *bucketID,
					Cursor:    usagesAsc[limit-1].RollupEndTime,
					Direction: console.Fwd,
					Limit:     limit,
				}

				opcount := 0
				var usagesAfter []console.BucketUsage
				for {
					opcount++

					rows, err := busages.GetByBucketID(ctx, iterator)
					assert.NoError(t, err)

					usagesAfter = append(usagesAfter, rows...)

					if iterator.Next == nil {
						break
					}
					iterator = iterator.Next
				}

				assert.Equal(t, count-limit, len(usagesAfter))
				assert.Equal(t, (count-limit)/limit+1, opcount)

				for i := range usagesAfter {
					assert.Equal(t, usagesAsc[i+limit].ID, usagesAfter[i].ID)
				}
			})
		})
	})
}
