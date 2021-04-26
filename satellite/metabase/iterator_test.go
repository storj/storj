// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
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

func TestIterateObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("ProjectID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{},
					BucketName: "sj://mybucket",
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("BucketName missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "",
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("Limit is negative", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					BatchSize:  -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			objects := createObjects(ctx, t, db, 2, uuid.UUID{1}, "mybucket")
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "myemptybucket",
					BatchSize:  10,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("pending and committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			pending := metabasetest.RandObjectStream()
			pending.ObjectKey = metabase.ObjectKey("firstObject")
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName
			committed.ObjectKey = metabase.ObjectKey("secondObject")

			projectID := pending.ProjectID
			bucketName := pending.BucketName

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pending,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				Result: []metabase.ObjectEntry{
					{
						ObjectKey:  pending.ObjectKey,
						Version:    pending.Version,
						StreamID:   pending.StreamID,
						CreatedAt:  now,
						Status:     metabase.Pending,
						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectKey:                     committed.ObjectKey,
						Version:                       committed.Version,
						StreamID:                      committed.StreamID,
						CreatedAt:                     now,
						Status:                        metabase.Committed,
						Encryption:                    metabasetest.DefaultEncryption,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					BatchSize:  limit,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					BatchSize:  limit,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket in project with 2 buckets", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsBucketA := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-a")
			objectsBucketB := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-b")
			for i, obj := range objectsBucketA {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "bucket-a",
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: append(objectsBucketA, objectsBucketB...)}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket with same bucketName in another project", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsProject1 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "mybucket")
			objectsProject2 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{2}, "mybucket")
			for i, obj := range objectsProject1 {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: append(objectsProject1, objectsProject2...)}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"a",
				"b/1",
				"b/2",
				"b/3",
				"c",
				"c/",
				"c//",
				"c/1",
				"g",
			})

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Cursor: metabase.IterateCursor{Key: "a", Version: 10},
				},
				Result: []metabase.ObjectEntry{
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: []metabase.ObjectEntry{
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "a"},
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: -3},
				},
				Result: withoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)
		})

		t.Run("boundaries", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			queries := []metabase.ObjectKey{""}
			for a := 0; a <= 0xFF; a++ {
				if 5 < a && a < 250 {
					continue
				}
				queries = append(queries, metabase.ObjectKey([]byte{byte(a)}))
				for b := 0; b <= 0xFF; b++ {
					if 5 < b && b < 250 {
						continue
					}
					queries = append(queries, metabase.ObjectKey([]byte{byte(a), byte(b)}))
				}
			}

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, queries[1:])

			for _, cursor := range queries {
				for _, prefix := range queries {
					var collector metabasetest.IterateCollector
					err := db.IterateObjectsAllVersions(ctx, metabase.IterateObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor,
							Version: -1,
						},
						Prefix: prefix,
					}, collector.Add)
					require.NoError(t, err)
				}
			}
		})

		t.Run("verify-iterator-boundary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"
			queries := []metabase.ObjectKey{"\x00\xFF"}
			createObjectsWithKeys(ctx, t, db, projectID, bucketName, queries)
			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersions(ctx, metabase.IterateObjects{
				ProjectID:  projectID,
				BucketName: bucketName,
				Cursor: metabase.IterateCursor{
					Key:     metabase.ObjectKey([]byte{}),
					Version: -1,
				},
				Prefix: metabase.ObjectKey([]byte{1}),
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"1",
				"a/a",
				"a/0",
			})

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersions(ctx, metabase.IterateObjects{
				ProjectID:  projectID,
				BucketName: bucketName,
				Prefix:     metabase.ObjectKey("a/"),
				BatchSize:  1,
			}, collector.Add)
			require.NoError(t, err)
		})
	})
}

func TestIterateObjectsWithStatus(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("BucketName missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			t.Run("ProjectID missing", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.IterateObjectsWithStatus{
					Opts: metabase.IterateObjectsWithStatus{
						ProjectID:  uuid.UUID{},
						BucketName: "sj://mybucket",
						Recursive:  true,
						Status:     metabase.Committed,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ProjectID missing",
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("Limit is negative", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					BatchSize:  -1,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("Status is invalid", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "test",
					Recursive:  true,
					Status:     255,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Status 255 is not supported",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			objects := createObjects(ctx, t, db, 2, uuid.UUID{1}, "mybucket")
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "myemptybucket",
					BatchSize:  10,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("based on status", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			pending := metabasetest.RandObjectStream()
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName

			projectID := pending.ProjectID
			bucketName := pending.BucketName

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pending,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: []metabase.ObjectEntry{{
					ObjectKey:                     committed.ObjectKey,
					Version:                       committed.Version,
					StreamID:                      committed.StreamID,
					CreatedAt:                     now,
					Status:                        metabase.Committed,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				}},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Pending,
				},
				Result: []metabase.ObjectEntry{{
					ObjectKey:  pending.ObjectKey,
					Version:    pending.Version,
					StreamID:   pending.StreamID,
					CreatedAt:  now,
					Status:     metabase.Pending,
					Encryption: metabasetest.DefaultEncryption,
				}},
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					BatchSize:  limit,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					BatchSize:  limit,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket in project with 2 buckets", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsBucketA := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-a")
			objectsBucketB := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-b")
			for i, obj := range objectsBucketA {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "bucket-a",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: append(objectsBucketA, objectsBucketB...)}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket with same bucketName in another project", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsProject1 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "mybucket")
			objectsProject2 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{2}, "mybucket")
			for i, obj := range objectsProject1 {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: append(objectsProject1, objectsProject2...)}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"a",
				"b/1",
				"b/2",
				"b/3",
				"c",
				"c/",
				"c//",
				"c/1",
				"g",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Cursor: metabase.IterateCursor{Key: "a", Version: 10},
				},
				Result: []metabase.ObjectEntry{
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: []metabase.ObjectEntry{
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "a"},
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: -3},
				},
				Result: withoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)
		})

		t.Run("non-recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"a",
				"b/1",
				"b/2",
				"b/3",
				"c",
				"c/",
				"c//",
				"c/1",
				"g",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					prefixEntry("b/", metabase.Committed),
					objects["c"],
					prefixEntry("c/", metabase.Committed),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Cursor: metabase.IterateCursor{Key: "a", Version: 10},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/", metabase.Committed),
					objects["c"],
					prefixEntry("c/", metabase.Committed),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/", metabase.Committed),
					objects["c"],
					prefixEntry("c/", metabase.Committed),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "a"},
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: -3},
				},
				Result: withoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "c/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: withoutPrefix("c/",
					objects["c/"],
					prefixEntry("c//", metabase.Committed),
					objects["c/1"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Status:     metabase.Committed,

					Prefix: "c//",
				},
				Result: withoutPrefix("c//",
					objects["c//"],
				),
			}.Check(ctx, t, db)
		})

		t.Run("boundaries", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			queries := []metabase.ObjectKey{""}
			for a := 0; a <= 0xFF; a++ {
				if 5 < a && a < 250 {
					continue
				}
				queries = append(queries, metabase.ObjectKey([]byte{byte(a)}))
				for b := 0; b <= 0xFF; b++ {
					if 5 < b && b < 250 {
						continue
					}
					queries = append(queries, metabase.ObjectKey([]byte{byte(a), byte(b)}))
				}
			}

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, queries[1:])

			for _, cursor := range queries {
				for _, prefix := range queries {
					var collector metabasetest.IterateCollector
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor,
							Version: -1,
						},
						Prefix: prefix,
						Status: metabase.Committed,
					}, collector.Add)
					require.NoError(t, err)

					err = db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor,
							Version: -1,
						},
						Prefix:    prefix,
						Recursive: true,
						Status:    metabase.Committed,
					}, collector.Add)
					require.NoError(t, err)
				}
			}
		})

		t.Run("verify-iterator-boundary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"
			queries := []metabase.ObjectKey{"\x00\xFF"}
			createObjectsWithKeys(ctx, t, db, projectID, bucketName, queries)
			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:  projectID,
				BucketName: bucketName,
				Cursor: metabase.IterateCursor{
					Key:     metabase.ObjectKey([]byte{}),
					Version: -1,
				},
				Prefix: metabase.ObjectKey([]byte{1}),
				Status: metabase.Committed,
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"1",
				"a/a",
				"a/0",
			})

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:  projectID,
				BucketName: bucketName,
				Prefix:     metabase.ObjectKey("a/"),
				BatchSize:  1,
				Status:     metabase.Committed,
			}, collector.Add)
			require.NoError(t, err)
		})
	})
}

func TestIteratePendingObjectsWithObjectKey(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.IteratePendingObjectsByKey{
					Opts: metabase.IteratePendingObjectsByKey{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()

			metabasetest.CreateObject(ctx, t, db, obj, 0)
			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: obj.Location(),
					BatchSize:      10,
				},
				Result: nil,
			}.Check(ctx, t, db)
		})
		t.Run("non existing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			pending := metabasetest.RandObjectStream()
			metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			object := metabase.RawObject{
				ObjectStream: pending,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption: metabasetest.DefaultEncryption,
			}

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
						ObjectKey:  pending.Location().ObjectKey + "other",
					},
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{object}}.Check(ctx, t, db)
		})

		t.Run("less and more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := []metabase.ObjectStream{metabasetest.RandObjectStream(), metabasetest.RandObjectStream(), metabasetest.RandObjectStream()}

			location := pending[0].Location()
			objects := make([]metabase.RawObject, 3)
			expected := make([]metabase.ObjectEntry, 3)

			for i, obj := range pending {
				obj.ProjectID = location.ProjectID
				obj.BucketName = location.BucketName
				obj.ObjectKey = location.ObjectKey
				obj.Version = metabase.Version(i + 1)

				metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

				objects[i] = metabase.RawObject{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Pending,

					Encryption: metabasetest.DefaultEncryption,
				}
				expected[i] = objectEntryFromRaw(objects[i])
			}

			sort.Slice(expected, func(i, j int) bool {
				return expected[i].StreamID.Less(expected[j].StreamID)
			})

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      10,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      2,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("prefixed object key", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.RandObjectStream()
			pending.ObjectKey = metabase.ObjectKey("a/prefixed/" + string(location.ObjectKey))
			metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			object := metabase.RawObject{
				ObjectStream: pending,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption: metabasetest.DefaultEncryption,
			}

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: pending.Location(),
				},
				Result: []metabase.ObjectEntry{objectEntryFromRaw(object)},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{object}}.Check(ctx, t, db)
		})

		t.Run("using streamID cursor", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			pending := []metabase.ObjectStream{metabasetest.RandObjectStream(), metabasetest.RandObjectStream(), metabasetest.RandObjectStream()}

			location := pending[0].Location()
			objects := make([]metabase.RawObject, 3)
			expected := make([]metabase.ObjectEntry, 3)

			for i, obj := range pending {
				obj.ProjectID = location.ProjectID
				obj.BucketName = location.BucketName
				obj.ObjectKey = location.ObjectKey
				obj.Version = metabase.Version(i + 1)

				metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

				objects[i] = metabase.RawObject{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Pending,

					Encryption: metabasetest.DefaultEncryption,
				}
				expected[i] = objectEntryFromRaw(objects[i])
			}

			sort.Slice(expected, func(i, j int) bool {
				return expected[i].StreamID.Less(expected[j].StreamID)
			})

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      10,
					Cursor: metabase.StreamIDCursor{
						StreamID: expected[0].StreamID,
					},
				},
				Result: expected[1:],
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})
	})
}

func createObjects(ctx *testcontext.Context, t *testing.T, db *metabase.DB, numberOfObjects int, projectID uuid.UUID, bucketName string) []metabase.RawObject {
	objects := make([]metabase.RawObject, numberOfObjects)
	for i := 0; i < numberOfObjects; i++ {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		now := time.Now()

		metabasetest.CreateObject(ctx, t, db, obj, 0)

		objects[i] = metabase.RawObject{
			ObjectStream: obj,
			CreatedAt:    now,
			Status:       metabase.Committed,
			Encryption:   metabasetest.DefaultEncryption,
		}
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return objects[i].ObjectKey < objects[j].ObjectKey
	})
	return objects
}

func createObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.ObjectEntry, len(keys))
	for _, key := range keys {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key
		now := time.Now()

		metabasetest.CreateObject(ctx, t, db, obj, 0)

		objects[key] = metabase.ObjectEntry{
			ObjectKey:  obj.ObjectKey,
			Version:    obj.Version,
			StreamID:   obj.StreamID,
			CreatedAt:  now,
			Status:     metabase.Committed,
			Encryption: metabasetest.DefaultEncryption,
		}
	}

	return objects
}

func withoutPrefix(prefix metabase.ObjectKey, entries ...metabase.ObjectEntry) []metabase.ObjectEntry {
	xs := make([]metabase.ObjectEntry, len(entries))
	for i, e := range entries {
		xs[i] = e
		xs[i].ObjectKey = entries[i].ObjectKey[len(prefix):]
	}
	return xs
}

func prefixEntry(key metabase.ObjectKey, status metabase.ObjectStatus) metabase.ObjectEntry {
	return metabase.ObjectEntry{
		IsPrefix:  true,
		ObjectKey: key,
		Status:    status,
	}
}

func objectEntryFromRaw(m metabase.RawObject) metabase.ObjectEntry {
	return metabase.ObjectEntry{
		IsPrefix:                      false,
		ObjectKey:                     m.ObjectKey,
		Version:                       m.Version,
		StreamID:                      m.StreamID,
		CreatedAt:                     m.CreatedAt,
		ExpiresAt:                     m.ExpiresAt,
		Status:                        m.Status,
		SegmentCount:                  m.SegmentCount,
		EncryptedMetadataNonce:        m.EncryptedMetadataNonce,
		EncryptedMetadata:             m.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: m.EncryptedMetadataEncryptedKey,
		TotalEncryptedSize:            m.TotalEncryptedSize,
		FixedSegmentSize:              m.FixedSegmentSize,
		Encryption:                    m.Encryption,
		ZombieDeletionDeadline:        m.ZombieDeletionDeadline,
	}
}
