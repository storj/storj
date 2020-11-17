// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestIterateObjects(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("BucketName missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			t.Run("ProjectID missing", func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				IterateObjects{
					Opts: metabase.IterateObjects{
						ProjectID:  uuid.UUID{},
						BucketName: "sj://mybucket",
						Recursive:  true,
						Status:     metabase.Committed,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ProjectID missing",
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})
		t.Run("Limit is negative", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					BatchSize:  -1,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})
		t.Run("Status is invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "test",
					Recursive:  true,
					Status:     255,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Status 255 is not supported",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			objects := createObjects(ctx, t, db, 2, uuid.UUID{1}, "mybucket")
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "myemptybucket",
					BatchSize:  10,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: nil,
			}.Check(ctx, t, db)
			Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("based on status", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			pending := randObjectStream()
			committed := randObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName

			projectID := pending.ProjectID
			bucketName := pending.BucketName

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pending,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: []metabase.ObjectEntry{{
					ObjectStream:                  committed,
					CreatedAt:                     now,
					Status:                        metabase.Committed,
					Encryption:                    defaultTestEncryption,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				}},
			}.Check(ctx, t, db)

			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,
					Status:     metabase.Pending,
				},
				Result: []metabase.ObjectEntry{{
					ObjectStream: pending,
					CreatedAt:    now,
					Status:       metabase.Pending,
					Encryption:   defaultTestEncryption,
				}},
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = metabase.ObjectEntry(obj)
			}
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					BatchSize:  limit,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = metabase.ObjectEntry(obj)
			}
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					BatchSize:  limit,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket in project with 2 buckets", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsBucketA := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-a")
			objectsBucketB := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-b")
			for i, obj := range objectsBucketA {
				expected[i] = metabase.ObjectEntry(obj)
			}
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "bucket-a",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			Verify{Objects: append(objectsBucketA, objectsBucketB...)}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket with same bucketName in another project", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5
			expected := make([]metabase.ObjectEntry, numberOfObjectsPerBucket)
			objectsProject1 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "mybucket")
			objectsProject2 := createObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{2}, "mybucket")
			for i, obj := range objectsProject1 {
				expected[i] = metabase.ObjectEntry(obj)
			}
			IterateObjects{
				Opts: metabase.IterateObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "mybucket",
					Recursive:  true,
					Status:     metabase.Committed,
				},
				Result: expected,
			}.Check(ctx, t, db)
			Verify{Objects: append(objectsProject1, objectsProject2...)}.Check(ctx, t, db)
		})

		t.Run("options", func(t *testing.T) {
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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

			IterateObjects{
				Opts: metabase.IterateObjects{
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
	})
}

func createObjects(ctx *testcontext.Context, t *testing.T, db *metabase.DB, numberOfObjects int, projectID uuid.UUID, bucketName string) []metabase.RawObject {
	objects := make([]metabase.RawObject, numberOfObjects)
	for i := 0; i < numberOfObjects; i++ {
		obj := randObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		now := time.Now()

		createObject(ctx, t, db, obj, 0)

		objects[i] = metabase.RawObject{
			ObjectStream: obj,
			CreatedAt:    now,
			Status:       metabase.Committed,
			Encryption:   defaultTestEncryption,
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
		obj := randObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key
		now := time.Now()

		createObject(ctx, t, db, obj, 0)

		objects[key] = metabase.ObjectEntry{
			ObjectStream: obj,
			CreatedAt:    now,
			Status:       metabase.Committed,
			Encryption:   defaultTestEncryption,
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
