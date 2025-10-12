// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestCollectBucketTallies(t *testing.T) {
	t.Parallel()
	for _, usePartitionQuery := range []bool{false, true} {
		t.Run(fmt.Sprintf("usePartitionQuery=%v", usePartitionQuery), func(t *testing.T) {
			testCollectBucketTallies(t, usePartitionQuery)
		})
	}
}

func testCollectBucketTallies(t *testing.T, usePartitionQuery bool) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("empty from", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					To: metabase.BucketLocation{
						ProjectID:  testrand.UUID(),
						BucketName: "name does not exist 2",
					},
					UsePartitionQuery: usePartitionQuery,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty to", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  testrand.UUID(),
						BucketName: "name does not exist",
					},
					UsePartitionQuery: usePartitionQuery,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "project ID To is before project ID From",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			randStream := metabasetest.RandObjectStream()

			obj := metabasetest.CreateObject(ctx, t, db, metabase.ObjectStream{
				ProjectID:  randStream.ProjectID,
				BucketName: randStream.BucketName,
				ObjectKey:  randStream.ObjectKey,
				Version:    randStream.Version,
				StreamID:   randStream.StreamID,
			}, 0)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					Version: randStream.Version,
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  randStream.ProjectID,
						BucketName: randStream.BucketName,
						ObjectKey:  randStream.ObjectKey,
					},
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{obj},
				},
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					To: metabase.BucketLocation{
						ProjectID:  randStream.ProjectID,
						BucketName: randStream.BucketName,
					},
					UsePartitionQuery: usePartitionQuery,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:              metabase.BucketLocation{},
					To:                metabase.BucketLocation{},
					UsePartitionQuery: usePartitionQuery,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid bucket name", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectA := uuid.UUID{1}
			projectB := uuid.UUID{2}

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  projectA,
						BucketName: "a\\",
					},
					To: metabase.BucketLocation{
						ProjectID:  projectB,
						BucketName: "b\\",
					},
					UsePartitionQuery: usePartitionQuery,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending and committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.RandObjectStream()
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName + "q"

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      pending,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, committed, 1)

			expected := []metabase.BucketTally{
				{
					BucketLocation: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					ObjectCount:        1,
					PendingObjectCount: 1,
					TotalSegments:      0,
					TotalBytes:         0,
					MetadataSize:       int64(len(userData.EncryptedMetadata) + len(userData.EncryptedETag)),
					BytesByRemainder: map[int64]int64{
						0: 0,
					},
				},
				{
					BucketLocation: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
					ObjectCount:        1,
					PendingObjectCount: 0,
					TotalSegments:      1,
					TotalBytes:         1024,
					MetadataSize:       0,
					BytesByRemainder: map[int64]int64{
						0: 1024,
					},
				},
			}

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					To: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
					UsePartitionQuery: usePartitionQuery,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					To: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
					AsOfSystemInterval: time.Millisecond,
					UsePartitionQuery:  usePartitionQuery,
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projects := []uuid.UUID{}
			for i := 0; i < 10; i++ {
				p := testrand.UUID()
				p[0] = byte(i)
				projects = append(projects, p)
			}
			bucketNames := []metabase.BucketName{"a", "b", "c", "d", "e"}
			bucketLocations := make([]metabase.BucketLocation, 0, len(projects)*len(bucketNames))

			expected := make([]metabase.BucketTally, 0, len(projects)*len(bucketNames))
			for _, projectID := range projects {
				for _, bucketName := range bucketNames {
					bucketLocations = append(bucketLocations, metabase.BucketLocation{
						ProjectID:  projectID,
						BucketName: bucketName,
					})
					rawObjects := createObjects(ctx, t, db, 1, projectID, bucketName)
					for _, obj := range rawObjects {
						expected = append(expected, bucketTallyFromRaw(obj))
					}
				}
			}
			sortBucketLocations(bucketLocations)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:              bucketLocations[0],
					To:                bucketLocations[len(bucketLocations)-1],
					UsePartitionQuery: usePartitionQuery,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:               bucketLocations[0],
					To:                 bucketLocations[len(bucketLocations)-1],
					AsOfSystemInterval: time.Millisecond,
					UsePartitionQuery:  usePartitionQuery,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:               bucketLocations[0],
					To:                 bucketLocations[15],
					AsOfSystemInterval: time.Millisecond,
					UsePartitionQuery:  usePartitionQuery,
				},
				Result: expected[0:16],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:               bucketLocations[16],
					To:                 bucketLocations[34],
					AsOfSystemInterval: time.Millisecond,
					UsePartitionQuery:  usePartitionQuery,
				},
				Result: expected[16:35],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:               bucketLocations[30],
					To:                 bucketLocations[10],
					AsOfSystemInterval: time.Millisecond,
					UsePartitionQuery:  usePartitionQuery,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "project ID To is before project ID From",
			}.Check(ctx, t, db)
		})
	})
}

func bucketTallyFromRaw(m metabase.RawObject) metabase.BucketTally {
	return metabase.BucketTally{
		BucketLocation: metabase.BucketLocation{
			ProjectID:  m.ProjectID,
			BucketName: m.BucketName,
		},
		ObjectCount:   1,
		TotalSegments: int64(m.SegmentCount),
		TotalBytes:    m.TotalEncryptedSize,
		MetadataSize:  int64(len(m.EncryptedMetadata) + len(m.EncryptedETag)),
		BytesByRemainder: map[int64]int64{
			0: m.TotalEncryptedSize,
		},
	}
}

func sortBucketLocations(bc []metabase.BucketLocation) {
	sort.Slice(bc, func(i, j int) bool {
		if bc[i].ProjectID == bc[j].ProjectID {
			return bc[i].BucketName < bc[j].BucketName
		}
		return bc[i].ProjectID.Less(bc[j].ProjectID)
	})
}

func TestCollectBucketTallies_WithRemainder(t *testing.T) {
	t.Parallel()
	for _, usePartitionQuery := range []bool{false, true} {
		t.Run(fmt.Sprintf("usePartitionQuery=%v", usePartitionQuery), func(t *testing.T) {
			testCollectBucketTalliesWithRemainder(t, usePartitionQuery)
		})
	}
}

func testCollectBucketTalliesWithRemainder(t *testing.T, usePartitionQuery bool) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		bucketName := metabase.BucketName("test-bucket")

		// Create objects of various sizes.
		// Each segment in CreateObject is 1024 bytes (1KB) encrypted size.
		smallObj := metabasetest.RandObjectStream()
		smallObj.ProjectID = projectID
		smallObj.BucketName = bucketName
		smallObj.ObjectKey = "small-1kb"

		mediumObj := metabasetest.RandObjectStream()
		mediumObj.ProjectID = projectID
		mediumObj.BucketName = bucketName
		mediumObj.ObjectKey = "medium-5kb"

		largeObj := metabasetest.RandObjectStream()
		largeObj.ProjectID = projectID
		largeObj.BucketName = bucketName
		largeObj.ObjectKey = "large-100kb"

		// Create objects: 1 segment (1KB), 5 segments (5KB), 100 segments (100KB).
		obj1 := metabasetest.CreateObject(ctx, t, db, smallObj, 1)
		obj2 := metabasetest.CreateObject(ctx, t, db, mediumObj, 5)
		obj3 := metabasetest.CreateObject(ctx, t, db, largeObj, 100)

		// Verify actual object sizes.
		require.EqualValues(t, 1024, obj1.TotalEncryptedSize)   // 1KB
		require.EqualValues(t, 5120, obj2.TotalEncryptedSize)   // 5KB
		require.EqualValues(t, 102400, obj3.TotalEncryptedSize) // 100KB

		// Actual total bytes: 1KB + 5KB + 100KB = 108544 bytes.
		actualTotalBytes := int64(108544)

		t.Run("nil remainders (default to 0)", func(t *testing.T) {
			tallies, err := db.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				To: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName + "z",
				},
				Now:               time.Now(),
				UsePartitionQuery: usePartitionQuery,
				StorageRemainders: nil, // Should default to []int64{0}.
			})
			require.NoError(t, err)
			require.Len(t, tallies, 1)

			tally := tallies[0]

			// Should have actual bytes (no remainder applied).
			require.Equal(t, actualTotalBytes, tally.BytesByRemainder[0], "BytesByRemainder[0] should equal actual bytes")
			require.Equal(t, actualTotalBytes, tally.TotalBytes, "TotalBytes should equal actual bytes (first remainder)")
			require.EqualValues(t, 3, tally.ObjectCount, "should have 3 objects")
			require.EqualValues(t, 106, tally.TotalSegments, "should have 106 total segments")
		})

		t.Run("explicit remainder 0", func(t *testing.T) {
			tallies, err := db.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				To: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName + "z",
				},
				Now:               time.Now(),
				UsePartitionQuery: usePartitionQuery,
				StorageRemainders: []int64{0},
			})
			require.NoError(t, err)
			require.Len(t, tallies, 1)

			tally := tallies[0]

			// Should have actual bytes (no remainder applied).
			require.Equal(t, actualTotalBytes, tally.BytesByRemainder[0], "BytesByRemainder[0] should equal actual bytes")
			require.Equal(t, actualTotalBytes, tally.TotalBytes, "TotalBytes should equal actual bytes")
		})

		t.Run("single non-zero remainder", func(t *testing.T) {
			remainder := int64(51200) // 50KB

			tallies, err := db.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				To: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName + "z",
				},
				Now:               time.Now(),
				UsePartitionQuery: usePartitionQuery,
				StorageRemainders: []int64{remainder},
			})
			require.NoError(t, err)
			require.Len(t, tallies, 1)

			tally := tallies[0]

			// With remainder of 50KB:
			// - Small object (1KB) → counted as 50KB (remainder applies)
			// - Medium object (5KB) → counted as 50KB (remainder applies)
			// - Large object (100KB) → counted as 100KB (already larger than remainder)
			// Total = 50KB + 50KB + 100KB = 204800 bytes
			expectedBytes := int64(204800)

			require.Equal(t, expectedBytes, tally.BytesByRemainder[remainder], "BytesByRemainder[51200] should equal 200KB")

			// TotalBytes should equal actual bytes (remainder=0 is always auto-added)
			require.Equal(t, actualTotalBytes, tally.TotalBytes, "TotalBytes should equal actual bytes (always includes remainder=0)")
		})

		t.Run("multiple remainders including 0", func(t *testing.T) {
			remainder50KB := int64(51200)   // 50KB
			remainder100KB := int64(102400) // 100KB

			tallies, err := db.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				To: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName + "z",
				},
				Now:               time.Now(),
				UsePartitionQuery: usePartitionQuery,
				StorageRemainders: []int64{0, remainder50KB, remainder100KB},
			})
			require.NoError(t, err)
			require.Len(t, tallies, 1)

			tally := tallies[0]

			// With remainder=0: actual bytes.
			require.Equal(t, actualTotalBytes, tally.BytesByRemainder[0], "BytesByRemainder[0] should equal actual bytes")

			// With remainder=50KB: 50KB + 50KB + 100KB = 200KB.
			expectedBytes50KB := int64(204800)
			require.Equal(t, expectedBytes50KB, tally.BytesByRemainder[remainder50KB], "BytesByRemainder[51200] should equal 200KB")

			// With remainder=100KB: 100KB + 100KB + 100KB = 300KB.
			expectedBytes100KB := int64(307200)
			require.Equal(t, expectedBytes100KB, tally.BytesByRemainder[remainder100KB], "BytesByRemainder[102400] should equal 300KB")

			// TotalBytes should be the first remainder value (remainder=0).
			require.Equal(t, actualTotalBytes, tally.TotalBytes, "TotalBytes should equal actual bytes (first remainder)")
		})

		t.Run("multiple remainders without 0", func(t *testing.T) {
			remainder50KB := int64(51200)   // 50KB
			remainder100KB := int64(102400) // 100KB

			tallies, err := db.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
				From: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				To: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: bucketName + "z",
				},
				Now:               time.Now(),
				UsePartitionQuery: usePartitionQuery,
				StorageRemainders: []int64{remainder50KB, remainder100KB},
			})
			require.NoError(t, err)
			require.Len(t, tallies, 1)

			tally := tallies[0]

			// BytesByRemainder SHOULD have 0 key (auto-added for backward compatibility).
			require.Equal(t, actualTotalBytes, tally.BytesByRemainder[0], "BytesByRemainder[0] should equal actual bytes (auto-added)")

			// With remainder=50KB: 50KB + 50KB + 100KB = 200KB
			expectedBytes50KB := int64(204800)
			require.Equal(t, expectedBytes50KB, tally.BytesByRemainder[remainder50KB], "BytesByRemainder[51200] should equal 200KB")

			// With remainder=100KB: 100KB + 100KB + 100KB = 300KB
			expectedBytes100KB := int64(307200)
			require.Equal(t, expectedBytes100KB, tally.BytesByRemainder[remainder100KB], "BytesByRemainder[102400] should equal 300KB")

			// TotalBytes should always equal actual bytes (remainder=0 is always included)
			require.Equal(t, actualTotalBytes, tally.TotalBytes, "TotalBytes should equal actual bytes (always includes remainder=0)")
		})
	})
}
