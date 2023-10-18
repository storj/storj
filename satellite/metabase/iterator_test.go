// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestIterateObjectsWithStatus(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("invalid arguments", func(t *testing.T) {
			t.Run("ProjectID missing", func(t *testing.T) {
				metabasetest.IterateObjectsWithStatus{
					Opts: metabase.IterateObjectsWithStatus{
						ProjectID:  uuid.UUID{},
						BucketName: "sj://mybucket",
						Recursive:  true,
						Pending:    false,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ProjectID missing",
				}.Check(ctx, t, db)
			})
			t.Run("BucketName missing", func(t *testing.T) {
				metabasetest.IterateObjectsWithStatus{
					Opts: metabase.IterateObjectsWithStatus{
						ProjectID:  uuid.UUID{1},
						BucketName: "",
						Recursive:  true,
						Pending:    false,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "BucketName missing",
				}.Check(ctx, t, db)
			})
			t.Run("Limit is negative", func(t *testing.T) {
				metabasetest.IterateObjectsWithStatus{
					Opts: metabase.IterateObjectsWithStatus{
						ProjectID:  uuid.UUID{1},
						BucketName: "mybucket",
						BatchSize:  -1,
						Recursive:  true,
						Pending:    false,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "BatchSize is negative",
				}.Check(ctx, t, db)
			})
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
					Pending:    false,
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
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{{
					ObjectKey:                     committed.ObjectKey,
					Version:                       committed.Version,
					StreamID:                      committed.StreamID,
					CreatedAt:                     now,
					Status:                        metabase.CommittedUnversioned,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				}},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
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
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
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
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
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
					ProjectID:             uuid.UUID{1},
					BucketName:            "bucket-a",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: append(objectsBucketA, objectsBucketB...),
			}.Check(ctx, t, db)
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
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: append(objectsProject1, objectsProject2...),
			}.Check(ctx, t, db)
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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "a", Version: objects["a"].Version + 1},
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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					prefixEntry("b/", metabase.CommittedUnversioned),
					objects["c"],
					prefixEntry("c/", metabase.CommittedUnversioned),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "a", Version: objects["a"].Version + 1},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/", metabase.CommittedUnversioned),
					objects["c"],
					prefixEntry("c/", metabase.CommittedUnversioned),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/", metabase.CommittedUnversioned),
					objects["c"],
					prefixEntry("c/", metabase.CommittedUnversioned),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: withoutPrefix("c/",
					objects["c/"],
					prefixEntry("c//", metabase.CommittedUnversioned),
					objects["c/1"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

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
				if 3 < a && a < 252 {
					continue
				}
				queries = append(queries, metabase.ObjectKey([]byte{byte(a)}))
				for b := 0; b <= 0xFF; b++ {
					if 4 < b && b < 251 {
						continue
					}
					queries = append(queries, metabase.ObjectKey([]byte{byte(a), byte(b)}))
				}
			}

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, queries[1:])

			var collector metabasetest.IterateCollector
			for _, cursor := range queries {
				for _, prefix := range queries {
					collector = collector[:0]
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor,
							Version: -1,
						},
						Prefix:                prefix,
						Pending:               false,
						IncludeCustomMetadata: true,
					}, collector.Add)
					require.NoError(t, err)

					collector = collector[:0]
					err = db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor,
							Version: -1,
						},
						Prefix:                prefix,
						Recursive:             true,
						Pending:               false,
						IncludeCustomMetadata: true,
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
				Prefix:                metabase.ObjectKey([]byte{1}),
				Pending:               false,
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: true,
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
				ProjectID:             projectID,
				BucketName:            bucketName,
				Prefix:                metabase.ObjectKey("a/"),
				BatchSize:             1,
				Pending:               false,
				IncludeCustomMetadata: true,
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("include metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj1,
					Encryption:                    metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             []byte{3},
					EncryptedMetadataEncryptedKey: []byte{4},
					EncryptedMetadataNonce:        []byte{5},
				},
			}.Run(ctx, t, db, obj1, 4)

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				Pending:               false,
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: true,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Equal(t, entry.EncryptedMetadata, []byte{3})
				require.Equal(t, entry.EncryptedMetadataEncryptedKey, []byte{4})
				require.Equal(t, entry.EncryptedMetadataNonce, []byte{5})
			}
		})

		t.Run("exclude custom metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj1,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadata:             []byte{3},
					EncryptedMetadataEncryptedKey: []byte{4},
					EncryptedMetadataNonce:        []byte{5},
				},
			}.Run(ctx, t, db, obj1, 4)

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				Pending:               false,
				IncludeCustomMetadata: false,
				IncludeSystemMetadata: true,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Nil(t, entry.EncryptedMetadataNonce)
				require.Nil(t, entry.EncryptedMetadata)
				require.Nil(t, entry.EncryptedMetadataEncryptedKey)
			}
		})

		t.Run("exclude system metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj1,
					Encryption:                    metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             []byte{3},
					EncryptedMetadataEncryptedKey: []byte{4},
					EncryptedMetadataNonce:        []byte{5},
				},
			}.Run(ctx, t, db, obj1, 4)

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				Pending:               false,
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: false,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				// fields that should always be set
				require.NotEmpty(t, entry.ObjectKey)
				require.NotEmpty(t, entry.StreamID)
				require.NotZero(t, entry.Version)
				require.Equal(t, metabase.CommittedUnversioned, entry.Status)
				require.False(t, entry.Encryption.IsZero())

				require.True(t, entry.CreatedAt.IsZero())
				require.Nil(t, entry.ExpiresAt)

				require.Zero(t, entry.SegmentCount)
				require.Zero(t, entry.TotalPlainSize)
				require.Zero(t, entry.TotalEncryptedSize)
				require.Zero(t, entry.FixedSegmentSize)

				require.NotNil(t, entry.EncryptedMetadataNonce)
				require.NotNil(t, entry.EncryptedMetadata)
				require.NotNil(t, entry.EncryptedMetadataEncryptedKey)
			}
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
				Pending:    false,
				BatchSize:  1,
			}, collector.Add)
			require.NoError(t, err)
			require.Equal(t, 2, len(collector))
		})
		t.Run("skip-expired-objects", func(t *testing.T) {
			now := time.Now()
			type test struct {
				notExpired []metabase.ObjectKey
				expired    []metabase.ObjectKey
			}
			testCases := []test{
				{
					notExpired: []metabase.ObjectKey{"1"},
					expired:    []metabase.ObjectKey{"2"},
				},
				{
					notExpired: []metabase.ObjectKey{"2"},
					expired:    []metabase.ObjectKey{"1"},
				},
				{
					notExpired: []metabase.ObjectKey{"2"},
					expired:    []metabase.ObjectKey{"1", "3"},
				},
				{
					notExpired: []metabase.ObjectKey{"2", "4"},
					expired:    []metabase.ObjectKey{"1", "3"},
				},
				{
					expired: []metabase.ObjectKey{"1", "2", "3", "4"},
				},
			}
			stream := metabase.ObjectStream{
				ProjectID:  uuid.UUID{1},
				BucketName: "bucket",
				Version:    1,
				StreamID:   testrand.UUID(),
			}
			for i, tc := range testCases {
				tc := tc
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					expectedResult := []metabase.ObjectEntry{}
					if len(tc.notExpired) == 0 {
						expectedResult = nil
					}
					for _, key := range tc.notExpired {
						stream.ObjectKey = key
						object := metabasetest.CreateObject(ctx, t, db, stream, 0)
						expectedResult = append(expectedResult, objectEntryFromRaw(metabase.RawObject(object)))
					}
					for _, key := range tc.expired {
						stream.ObjectKey = key
						metabasetest.CreateExpiredObject(ctx, t, db, stream, 0, now.Add(-2*time.Hour))
					}
					for _, batchSize := range []int{1, 2, 3} {
						opts := metabase.IterateObjectsWithStatus{
							ProjectID:             stream.ProjectID,
							BucketName:            stream.BucketName,
							BatchSize:             batchSize,
							Pending:               false,
							IncludeSystemMetadata: true,
						}
						metabasetest.IterateObjectsWithStatus{
							Opts:   opts,
							Result: expectedResult,
						}.Check(ctx, t, db)
						{
							opts := opts
							opts.Recursive = true
							metabasetest.IterateObjectsWithStatus{
								Opts:   opts,
								Result: expectedResult,
							}.Check(ctx, t, db)
						}
					}
				})
			}
		})

		t.Run("prefix longer than key", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectID, bucketName := uuid.UUID{1}, "bucky"
			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"aaaa/a",
				"aaaa/b",
				"aaaa/c",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             false,
					Prefix:                "aaaa/",
					Pending:               false,
					BatchSize:             2,
					IncludeSystemMetadata: true,
				},
				Result: withoutPrefix("aaaa/",
					objects["aaaa/a"],
					objects["aaaa/b"],
					objects["aaaa/c"],
				),
			}.Check(ctx, t, db)
		})

		t.Run("version greater than one", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectID, bucketName := uuid.UUID{2}, "bucky"

			id1 := metabasetest.RandObjectStream()
			id1.ProjectID = projectID
			id1.BucketName = bucketName
			id1.Version = metabase.Version(rand.Int31())

			id2 := metabasetest.RandObjectStream()
			id2.ProjectID = projectID
			id2.BucketName = bucketName
			id2.ObjectKey = id1.ObjectKey + "Z" // for deterministic ordering
			id2.Version = 1

			var objs []metabase.Object
			for _, id := range []metabase.ObjectStream{id1, id2} {
				obj, _ := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: id,
					},
					CommitObject: &metabase.CommitObject{
						ObjectStream: id,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Run(ctx, t, db, id, 1)
				objs = append(objs, obj)
			}

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					BatchSize:             3,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objs[0])),
					objectEntryFromRaw(metabase.RawObject(objs[1])),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestIterateObjectsSkipCursor(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID, bucketName := uuid.UUID{1}, "bucky"

		t.Run("no prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"08/test",
				"09/test",
				"10/test",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("08"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)
		})

		t.Run("prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"2017/05/08/test",
				"2017/05/09/test",
				"2017/05/10/test",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)
		})

		t.Run("batch-size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			afterDelimiter := metabase.ObjectKey(metabase.Delimiter + 1)

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"2017/05/08",
				"2017/05/08/a",
				"2017/05/08/b",
				"2017/05/08/c",
				"2017/05/08/d",
				"2017/05/08/e",
				"2017/05/08" + afterDelimiter,
				"2017/05/09/a",
				"2017/05/09/b",
				"2017/05/09/c",
				"2017/05/09/d",
				"2017/05/09/e",
				"2017/05/10/a",
				"2017/05/10/b",
				"2017/05/10/c",
				"2017/05/10/d",
				"2017/05/10/e",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					BatchSize:  3,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08"),
						Version: objects["2017/05/08"].Version,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
					withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					BatchSize:  3,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.IterateCursor{
						Key:     metabase.ObjectKey("2017/05/08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
					prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
					prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestIteratePendingObjectsWithObjectKey(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

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

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
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

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
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

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
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

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
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

		t.Run("same key different versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			obj2 := obj1
			obj2.StreamID = testrand.UUID()
			obj2.Version = 2

			pending := []metabase.ObjectStream{obj1, obj2}

			location := pending[0].Location()
			objects := make([]metabase.RawObject, 2)
			expected := make([]metabase.ObjectEntry, 2)

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

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
				}
				expected[i] = objectEntryFromRaw(objects[i])
			}

			sort.Slice(expected, func(i, j int) bool {
				return expected[i].StreamID.Less(expected[j].StreamID)
			})

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      1,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		// TODO(ver): add tests for delete markers and versioned/unversioned
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
			Status:       metabase.CommittedUnversioned,
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
			Status:     metabase.CommittedUnversioned,
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

func withoutPrefix1(prefix metabase.ObjectKey, entry metabase.ObjectEntry) metabase.ObjectEntry {
	entry.ObjectKey = entry.ObjectKey[len(prefix):]
	return entry
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
		TotalPlainSize:                m.TotalPlainSize,
		FixedSegmentSize:              m.FixedSegmentSize,
		Encryption:                    m.Encryption,
	}
}

func BenchmarkNonRecursiveListing(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		baseObj := metabasetest.RandObjectStream()

		for i := 0; i < 10; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("foo/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixA/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixB/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		for i := 0; i < 50; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("boo/foo" + strconv.Itoa(i) + "/object")
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		b.Run("listing no prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					BatchSize:  5,
					Pending:    false,
				}, func(ctx context.Context, oi metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for oi.Next(ctx, &entry) {
					}
					return nil
				})
				require.NoError(b, err)
			}
		})

		b.Run("listing with prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					Prefix:     "foo/",
					BatchSize:  5,
					Pending:    false,
				}, func(ctx context.Context, oi metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for oi.Next(ctx, &entry) {
					}
					return nil
				})
				require.NoError(b, err)
			}
		})

		b.Run("listing only prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					Prefix:     "boo/",
					BatchSize:  5,
					Pending:    false,
				}, func(ctx context.Context, oi metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for oi.Next(ctx, &entry) {
					}
					return nil
				})
				require.NoError(b, err)
			}
		})
	})
}
