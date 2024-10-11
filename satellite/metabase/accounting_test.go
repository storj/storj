// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestCollectBucketTallies(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("empty from", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					To: metabase.BucketLocation{
						ProjectID:  testrand.UUID(),
						BucketName: "name does not exist 2",
					},
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
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{},
					To:   metabase.BucketLocation{},
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

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:                  pending,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
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
					MetadataSize:       1024,
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
					AsOfSystemTime: time.Now(),
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
					From: bucketLocations[0],
					To:   bucketLocations[len(bucketLocations)-1],
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[0],
					To:             bucketLocations[len(bucketLocations)-1],
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[0],
					To:             bucketLocations[15],
					AsOfSystemTime: time.Now(),
				},
				Result: expected[0:16],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[16],
					To:             bucketLocations[34],
					AsOfSystemTime: time.Now(),
				},
				Result: expected[16:35],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[30],
					To:             bucketLocations[10],
					AsOfSystemTime: time.Now(),
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
		MetadataSize:  int64(len(m.EncryptedMetadata)),
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
