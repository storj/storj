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

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil/spannerutil"
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

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:              committed,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
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
					IncludeETag:           true,
				},
				Result: []metabase.ObjectEntry{{
					ObjectKey:         committed.ObjectKey,
					Version:           committed.Version,
					StreamID:          committed.StreamID,
					CreatedAt:         now,
					Status:            metabase.CommittedUnversioned,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
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
					IncludeETag:           true,
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
					IncludeETag:           true,
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
					IncludeETag:           true,
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
					IncludeETag:           true,
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
					IncludeETag:           true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: append(objectsProject1, objectsProject2...),
			}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
					IncludeETag:           true,
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
					IncludeETag:           true,

					Cursor: metabase.IterateCursor{Key: "a", Version: objects["a"].Version - 1},
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
					IncludeETag:           true,

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
					IncludeETag:           true,

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
					IncludeETag:           true,

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
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: metabase.MaxVersion},
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
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)
		})

		t.Run("non-recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
					IncludeETag:           true,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
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
					IncludeETag:           true,

					Cursor: metabase.IterateCursor{Key: "a", Version: objects["a"].Version - 1},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
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
					IncludeETag:           true,

					Cursor: metabase.IterateCursor{Key: "b", Version: metabase.MaxVersion},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
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
					IncludeETag:           true,

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
					IncludeETag:           true,

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
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: metabase.MaxVersion},
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
					IncludeETag:           true,

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
					IncludeETag:           true,

					Prefix: "c/",
					Cursor: metabase.IterateCursor{Key: "c/", Version: metabase.MaxVersion},
				},
				Result: withoutPrefix("c/",
					objects["c/"],
					prefixEntry("c//"),
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
					IncludeETag:           true,

					Prefix: "c//",
				},
				Result: withoutPrefix("c//",
					objects["c//"],
				),
			}.Check(ctx, t, db)
		})

		t.Run("boundaries", func(t *testing.T) {
			if _, ok := db.ChooseAdapter(uuid.UUID{}).(*metabase.SpannerAdapter); ok {
				// TODO(spanner): find a fix for this
				t.Skip("test runs too slow for spanner")
			}
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := []metabase.RawObject{}
			baseObject := metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: bucketName,
					Version:    1,
				},
				Status: metabase.CommittedVersioned,
			}

			for a := 0; a <= 0xFF; a++ {
				if 3 < a && a < 252 {
					continue
				}
				baseObject.ObjectKey = metabase.ObjectKey([]byte{byte(a)})
				baseObject.StreamID = testrand.UUID()
				objects = append(objects, baseObject)
				for b := 0; b <= 0xFF; b++ {
					if 4 < b && b < 251 {
						continue
					}
					baseObject.ObjectKey = metabase.ObjectKey([]byte{byte(a), byte(b)})
					baseObject.StreamID = testrand.UUID()
					objects = append(objects, baseObject)
				}
			}

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			var collector metabasetest.IterateCollector
			for _, cursor := range objects {
				for _, prefix := range objects {
					collector = collector[:0]
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor.ObjectKey,
							Version: -1,
						},
						Prefix:                prefix.ObjectKey,
						Pending:               false,
						IncludeCustomMetadata: true,
					}, collector.Add)
					require.NoError(t, err)

					collector = collector[:0]
					err = db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor.ObjectKey,
							Version: -1,
						},
						Prefix:                prefix.ObjectKey,
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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
			userData := metabasetest.RandEncryptedUserData()

			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
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
				IncludeETag:           true,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Equal(t, userData, entry.EncryptedUserData)
			}
		})

		t.Run("exclude custom metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
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
				IncludeETag:           false,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Nil(t, entry.EncryptedMetadataNonce)
				require.Nil(t, entry.EncryptedMetadata)
				require.Nil(t, entry.EncryptedMetadataEncryptedKey)
				require.Nil(t, entry.EncryptedETag)
			}
		})

		t.Run("include etag", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
				},
			}.Run(ctx, t, db, obj1, 4)

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				Pending:               false,
				IncludeCustomMetadata: false,
				IncludeSystemMetadata: false,
				IncludeETag:           true,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.NotNil(t, entry.EncryptedMetadataNonce)
				require.Nil(t, entry.EncryptedMetadata)
				require.NotNil(t, entry.EncryptedMetadataEncryptedKey)
				require.NotNil(t, entry.EncryptedETag)
			}
		})

		t.Run("include etag or metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			data1 := metabasetest.RandEncryptedUserData()
			data1.EncryptedETag = nil

			obj1 := metabasetest.RandObjectStream()
			obj1.ObjectKey = "1"
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         data1,
				},
			}.Run(ctx, t, db, obj1, 0)

			data2 := metabasetest.RandEncryptedUserData()

			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID, obj2.BucketName = obj1.ProjectID, obj1.BucketName
			obj2.ObjectKey = "2"
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj2,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         data2,
				},
			}.Run(ctx, t, db, obj2, 0)

			var collector metabasetest.IterateCollector
			err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
				ProjectID:  obj1.ProjectID,
				BucketName: obj1.BucketName,
				Recursive:  true,
				Pending:    false,

				IncludeETagOrCustomMetadata: true,
			}, collector.Add)
			require.NoError(t, err)

			require.NotNil(t, collector[0].EncryptedMetadataEncryptedKey)
			require.NotNil(t, collector[0].EncryptedMetadataNonce)
			require.NotNil(t, collector[1].EncryptedMetadataEncryptedKey)
			require.NotNil(t, collector[1].EncryptedMetadataNonce)

			require.NotNil(t, collector[0].EncryptedMetadata)
			require.Nil(t, collector[0].EncryptedETag)
			require.Nil(t, collector[1].EncryptedMetadata)
			require.NotNil(t, collector[1].EncryptedETag)
		})

		t.Run("exclude system metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
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
				IncludeETag:           true,
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
				require.NotNil(t, entry.EncryptedETag)
			}
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
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

			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
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

			projectID, bucketName := uuid.UUID{2}, metabase.BucketName("bucky")

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

		t.Run("2 objects, one with multiple versions and one without versioning", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("3 objects, one with versions one without and one pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			c0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000
			c0.ProjectID = a0.ProjectID
			c0.BucketName = a0.BucketName
			c0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)
			objC0 := metabasetest.CreatePendingObject(ctx, t, db, c0, 0)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
					metabase.RawObject(objC0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects one with versions and one pending, list pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001

			pendingObj := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  b0.ProjectID,
						BucketName: b0.BucketName,
						ObjectKey:  b0.ObjectKey,
						Version:    b0.Version,
						StreamID:   b0.StreamID,
					},
					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					{
						ObjectKey: pendingObj.ObjectKey,
						Version:   pendingObj.Version,
						StreamID:  pendingObj.StreamID,
						CreatedAt: pendingObj.CreatedAt,
						Status:    metabase.Pending,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(pendingObj),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with 2 versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001
			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with two versions and one with delete_marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001
			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			deletionResult := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA0.ProjectID,
								BucketName: objA0.BucketName,
								ObjectKey:  objA0.ObjectKey,
								Version:    1002,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(deletionResult.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResult.Markers[0]),
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with two versions and multiple delete_markers", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)

			deletionResultA0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA0.ProjectID,
								BucketName: objA0.BucketName,
								ObjectKey:  objA0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			deletionResultB0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objB0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objB0.ProjectID,
								BucketName: objB0.BucketName,
								ObjectKey:  objB0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			a1 := a0
			a1.Version = 1002
			b1 := b0
			b1.Version = 1002

			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)

			deletionResultA1 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA1.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA1.ProjectID,
								BucketName: objA1.BucketName,
								ObjectKey:  objA1.ObjectKey,
								Version:    1003,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			deletionResultB1 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objB1.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objB1.ProjectID,
								BucketName: objB1.BucketName,
								ObjectKey:  objB1.ObjectKey,
								Version:    1003,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(deletionResultA1.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(deletionResultA0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(deletionResultB1.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(deletionResultB0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objB0)),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResultA1.Markers[0]),
					metabase.RawObject(deletionResultB1.Markers[0]),
					metabase.RawObject(objA1),
					metabase.RawObject(objB1),
					metabase.RawObject(deletionResultA0.Markers[0]),
					metabase.RawObject(deletionResultB0.Markers[0]),
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("3 objects, 1 unversioned, 2 with multiple versions, 1 with and 1 without delete_marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000
			c0 := metabasetest.RandObjectStream()
			c0.ProjectID = a0.ProjectID
			c0.BucketName = a0.BucketName
			c0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				a0.ObjectKey, b0.ObjectKey = b0.ObjectKey, a0.ObjectKey
			}
			if a0.ObjectKey > c0.ObjectKey {
				a0.ObjectKey, c0.ObjectKey = c0.ObjectKey, a0.ObjectKey
			}
			if b0.ObjectKey > c0.ObjectKey {
				b0.ObjectKey, c0.ObjectKey = c0.ObjectKey, b0.ObjectKey
			}

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objC0 := metabasetest.CreateObjectVersioned(ctx, t, db, c0, 0)

			deletionResultC0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objC0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objC0.ProjectID,
								BucketName: objC0.BucketName,
								ObjectKey:  objC0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			b1 := b0
			b1.Version = 1001
			c1 := c0
			c1.Version = 1002

			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)
			objC1 := metabasetest.CreateObjectVersioned(ctx, t, db, c1, 0)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objC1)),
					objectEntryFromRaw(metabase.RawObject(deletionResultC0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objC0)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objC1),
					metabase.RawObject(objB1),
					metabase.RawObject(deletionResultC0.Markers[0]),
					metabase.RawObject(objC0),
					metabase.RawObject(objB0),
					metabase.RawObject(objA0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("list recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreateVersionedObjectsWithKeysAll(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			}, true)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: concat(
					objects["a"],
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
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

					Cursor: metabase.IterateCursor{Key: "a", Version: last(objects["a"]).Version - 1},
				},
				Result: concat(
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
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

					Cursor: metabase.IterateCursor{Key: "b", Version: metabase.MaxVersion},
				},
				Result: concat(
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
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
				},
				Result: withoutPrefix("b/",
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
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
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
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
					Cursor: metabase.IterateCursor{Key: "b/2", Version: metabase.MaxVersion},
				},
				Result: withoutPrefix("b/",
					concat(
						objects["b/2"],
						objects["b/3"],
					)...,
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

		t.Run("list non-recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreateVersionedObjectsWithKeysAll(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			}, true)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: concat(
					objects["a"],
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "a", Version: last(objects["a"]).Version - 1},
				},
				Result: concat(
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "b", Version: metabase.MaxVersion},
				},
				Result: concat(
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
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
				},
				Result: withoutPrefix("b/",
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
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
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
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
					Cursor: metabase.IterateCursor{Key: "b/2", Version: metabase.MaxVersion},
				},
				Result: withoutPrefix("b/",
					concat(
						objects["b/2"],
						objects["b/3"],
					)...,
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
					Cursor: metabase.IterateCursor{Key: "c/", Version: metabase.MaxVersion},
				},
				Result: withoutPrefix("c/",
					concat(
						objects["c/"],
						[]metabase.ObjectEntry{prefixEntry("c//")},
						objects["c/1"],
					)...,
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
					objects["c//"]...,
				),
			}.Check(ctx, t, db)
		})

		t.Run("batch iterate committed versioned, unversioned, and delete markers with pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			var expected []metabase.ObjectEntry
			var objLocation metabase.ObjectLocation

			// create 1 pending object first
			pendingStream1 := metabasetest.RandObjectStream()
			objLocation = pendingStream1.Location()
			pendingStream1.Version = 100

			pendingObject1 := metabasetest.CreatePendingObject(ctx, t, db, pendingStream1, 0)

			expected = append(expected, objectEntryFromRaw(metabase.RawObject(pendingObject1)))

			for i := 0; i < 10; i++ {
				unversionedStream := metabasetest.RandObjectStream()
				unversionedStream.ProjectID = objLocation.ProjectID
				unversionedStream.BucketName = objLocation.BucketName
				unversionedStream.ObjectKey = objLocation.ObjectKey
				unversionedStream.Version = metabase.Version(200 + i)
				if i == 0 {
					metabasetest.CreateObject(ctx, t, db, unversionedStream, 0)
				} else {
					metabasetest.CreateObjectVersioned(ctx, t, db, unversionedStream, 0)
				}
			}

			// create a second pending object
			pendingStream2 := metabasetest.RandObjectStream()
			pendingStream2.ProjectID = objLocation.ProjectID
			pendingStream2.BucketName = objLocation.BucketName
			pendingStream2.ObjectKey = objLocation.ObjectKey
			pendingStream2.Version = 300

			pendingObject2 := metabasetest.CreatePendingObject(ctx, t, db, pendingStream2, 0)

			expected = append(expected, objectEntryFromRaw(metabase.RawObject(pendingObject2)))

			sort.Slice(expected, func(i, k int) bool {
				return expected[i].Less(expected[k])
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             objLocation.ProjectID,
					BucketName:            objLocation.BucketName,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					BatchSize:             3,
					Recursive:             true,
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("final prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"\xff\x00",
				"\xffA",
				"\xff\xff",
			})

			metabasetest.IterateObjectsWithStatus{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					Prefix:                "\xff",
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: withoutPrefix("\xff",
					objects["\xff\x00"],
					objects["\xffA"],
					objects["\xff\xff"],
				),
			}.Check(ctx, t, db)
		})
	})
}

// TODO this test was copied (and renamed) from v1.95.1 (TestIterateObjectsWithStatus)
// Should be removed when metabase.ListingObjects performance issues will be fixed.
func TestIterateObjectsWithStatusAscending(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("invalid arguments", func(t *testing.T) {
			t.Run("ProjectID missing", func(t *testing.T) {
				metabasetest.IterateObjectsWithStatusAscending{
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
				metabasetest.IterateObjectsWithStatusAscending{
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
				metabasetest.IterateObjectsWithStatusAscending{
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
			metabasetest.IterateObjectsWithStatusAscending{
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

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:              committed,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
				},
				Result: []metabase.ObjectEntry{{
					ObjectKey:         committed.ObjectKey,
					Version:           committed.Version,
					StreamID:          committed.StreamID,
					CreatedAt:         now,
					Status:            metabase.CommittedUnversioned,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				}},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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
			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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
			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             uuid.UUID{1},
					BucketName:            "bucket-a",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: append(objectsProject1, objectsProject2...),
			}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "a"},
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: -3},
				},
				Result: withoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)
		})

		t.Run("non-recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
				},
				Result: []metabase.ObjectEntry{
					objects["a"],
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Cursor: metabase.IterateCursor{Key: "a", Version: objects["a"].Version + 1},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: []metabase.ObjectEntry{
					prefixEntry("b/"),
					objects["c"],
					prefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "a"},
				},
				Result: withoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "b/2", Version: -3},
				},
				Result: withoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "b/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "c/",
					Cursor: metabase.IterateCursor{Key: "c/"},
				},
				Result: withoutPrefix("c/",
					objects["c/"],
					prefixEntry("c//"),
					objects["c/1"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,

					Prefix: "c//",
				},
				Result: withoutPrefix("c//",
					objects["c//"],
				),
			}.Check(ctx, t, db)
		})

		t.Run("boundaries", func(t *testing.T) {
			if _, ok := db.ChooseAdapter(uuid.UUID{}).(*metabase.SpannerAdapter); ok {
				// TODO(spanner): find a fix for this
				t.Skip("test runs too slow for spanner")
			}
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := []metabase.RawObject{}
			baseObject := metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: bucketName,
					Version:    1,
				},
				Status: metabase.CommittedVersioned,
			}

			for a := 0; a <= 0xFF; a++ {
				if 3 < a && a < 252 {
					continue
				}
				baseObject.ObjectKey = metabase.ObjectKey([]byte{byte(a)})
				baseObject.StreamID = testrand.UUID()
				objects = append(objects, baseObject)
				for b := 0; b <= 0xFF; b++ {
					if 4 < b && b < 251 {
						continue
					}
					baseObject.ObjectKey = metabase.ObjectKey([]byte{byte(a), byte(b)})
					baseObject.StreamID = testrand.UUID()
					objects = append(objects, baseObject)
				}
			}

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			var collector metabasetest.IterateCollector
			for _, cursor := range objects {
				for _, prefix := range objects {
					collector = collector[:0]
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor.ObjectKey,
							Version: -1,
						},
						Prefix:                prefix.ObjectKey,
						Pending:               false,
						IncludeCustomMetadata: true,
						IncludeETag:           true,
					}, collector.Add)
					require.NoError(t, err)

					collector = collector[:0]
					err = db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.IterateCursor{
							Key:     cursor.ObjectKey,
							Version: -1,
						},
						Prefix:                prefix.ObjectKey,
						Recursive:             true,
						Pending:               false,
						IncludeCustomMetadata: true,
						IncludeETag:           true,
					}, collector.Add)
					require.NoError(t, err)
				}
			}
		})

		t.Run("verify-iterator-boundary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
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
				IncludeETag:           true,
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
				IncludeETag:           true,
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("include metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			userData := metabasetest.RandEncryptedUserData()

			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
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
				IncludeETag:           true,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Equal(t, userData, entry.EncryptedUserData)
			}
		})

		t.Run("exclude custom metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
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
				IncludeETag:           false,
			}, collector.Add)

			require.NoError(t, err)

			for _, entry := range collector {
				require.Nil(t, entry.EncryptedMetadataNonce)
				require.Nil(t, entry.EncryptedMetadata)
				require.Nil(t, entry.EncryptedMetadataEncryptedKey)
				require.Nil(t, entry.EncryptedETag)
			}
		})

		t.Run("exclude system metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj1,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
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
				IncludeETag:           true,
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
				require.NotNil(t, entry.EncryptedETag)
			}
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
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
						metabasetest.IterateObjectsWithStatusAscending{
							Opts:   opts,
							Result: expectedResult,
						}.Check(ctx, t, db)
						{
							opts := opts
							opts.Recursive = true
							metabasetest.IterateObjectsWithStatusAscending{
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

			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")
			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"aaaa/a",
				"aaaa/b",
				"aaaa/c",
			})

			metabasetest.IterateObjectsWithStatusAscending{
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

			projectID, bucketName := uuid.UUID{2}, metabase.BucketName("bucky")

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

			metabasetest.IterateObjectsWithStatusAscending{
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

		t.Run("2 objects, one with multiple versions and one without versioning", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("3 objects, one with versions one without and one pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			c0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000
			c0.ProjectID = a0.ProjectID
			c0.BucketName = a0.BucketName
			c0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)
			objC0 := metabasetest.CreatePendingObject(ctx, t, db, c0, 0)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
					metabase.RawObject(objC0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects one with versions and one pending, list pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001

			pendingObj := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  b0.ProjectID,
						BucketName: b0.BucketName,
						ObjectKey:  b0.ObjectKey,
						Version:    b0.Version,
						StreamID:   b0.StreamID,
					},
					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					{
						ObjectKey: pendingObj.ObjectKey,
						Version:   pendingObj.Version,
						StreamID:  pendingObj.StreamID,
						CreatedAt: pendingObj.CreatedAt,
						Status:    metabase.Pending,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(pendingObj),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with 2 versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001
			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with two versions and one with delete_marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			a1 := a0
			a1.Version = 1001
			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			deletionResult := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA0.ProjectID,
								BucketName: objA0.BucketName,
								ObjectKey:  objA0.ObjectKey,
								Version:    1002,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(deletionResult.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResult.Markers[0]),
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("2 objects, each with two versions and multiple delete_markers", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			a0.Version = 1000
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				b0.ObjectKey, a0.ObjectKey = a0.ObjectKey, b0.ObjectKey
			}

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)

			deletionResultA0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA0.ProjectID,
								BucketName: objA0.BucketName,
								ObjectKey:  objA0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			deletionResultB0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objB0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objB0.ProjectID,
								BucketName: objB0.BucketName,
								ObjectKey:  objB0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			a1 := a0
			a1.Version = 1002
			b1 := b0
			b1.Version = 1002

			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)

			deletionResultA1 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objA1.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objA1.ProjectID,
								BucketName: objA1.BucketName,
								ObjectKey:  objA1.ObjectKey,
								Version:    1003,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			deletionResultB1 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objB1.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objB1.ProjectID,
								BucketName: objB1.BucketName,
								ObjectKey:  objB1.ObjectKey,
								Version:    1003,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(deletionResultA0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objA1)),
					objectEntryFromRaw(metabase.RawObject(deletionResultA1.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(deletionResultB0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(deletionResultB1.Markers[0])),
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResultA1.Markers[0]),
					metabase.RawObject(deletionResultB1.Markers[0]),
					metabase.RawObject(objA1),
					metabase.RawObject(objB1),
					metabase.RawObject(deletionResultA0.Markers[0]),
					metabase.RawObject(deletionResultB0.Markers[0]),
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("3 objects, 1 unversioned, 2 with multiple versions, 1 with and 1 without delete_marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			a0 := metabasetest.RandObjectStream()
			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = a0.ProjectID
			b0.BucketName = a0.BucketName
			b0.Version = 1000
			c0 := metabasetest.RandObjectStream()
			c0.ProjectID = a0.ProjectID
			c0.BucketName = a0.BucketName
			c0.Version = 1000

			if a0.ObjectKey > b0.ObjectKey {
				a0.ObjectKey, b0.ObjectKey = b0.ObjectKey, a0.ObjectKey
			}
			if a0.ObjectKey > c0.ObjectKey {
				a0.ObjectKey, c0.ObjectKey = c0.ObjectKey, a0.ObjectKey
			}
			if b0.ObjectKey > c0.ObjectKey {
				b0.ObjectKey, c0.ObjectKey = c0.ObjectKey, b0.ObjectKey
			}

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objC0 := metabasetest.CreateObjectVersioned(ctx, t, db, c0, 0)

			deletionResultC0 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objC0.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  objC0.ProjectID,
								BucketName: objC0.BucketName,
								ObjectKey:  objC0.ObjectKey,
								Version:    1001,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			b1 := b0
			b1.Version = 1001
			c1 := c0
			c1.Version = 1002

			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)
			objC1 := metabasetest.CreateObjectVersioned(ctx, t, db, c1, 0)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					objectEntryFromRaw(metabase.RawObject(objA0)),
					objectEntryFromRaw(metabase.RawObject(objB0)),
					objectEntryFromRaw(metabase.RawObject(objB1)),
					objectEntryFromRaw(metabase.RawObject(objC0)),
					objectEntryFromRaw(metabase.RawObject(deletionResultC0.Markers[0])),
					objectEntryFromRaw(metabase.RawObject(objC1)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objC1),
					metabase.RawObject(objB1),
					metabase.RawObject(deletionResultC0.Markers[0]),
					metabase.RawObject(objC0),
					metabase.RawObject(objB0),
					metabase.RawObject(objA0),
				},
			}.Check(ctx, t, db)
		})

		t.Run("list recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreateVersionedObjectsWithKeysAll(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			}, false)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: concat(
					objects["a"],
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "a", Version: last(objects["a"]).Version + 1},
				},
				Result: concat(
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: concat(
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
					objects["c"],
					objects["c/"],
					objects["c//"],
					objects["c/1"],
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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

		t.Run("list non-recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreateVersionedObjectsWithKeysAll(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			}, false)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: concat(
					objects["a"],
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "a", Version: last(objects["a"]).Version + 1},
				},
				Result: concat(
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.IterateCursor{Key: "b", Version: 0},
				},
				Result: concat(
					[]metabase.ObjectEntry{prefixEntry("b/")},
					objects["c"],
					[]metabase.ObjectEntry{prefixEntry("c/")},
					objects["g"],
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: withoutPrefix("b/",
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["b/2"],
						objects["b/3"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
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

			metabasetest.IterateObjectsWithStatusAscending{
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
					concat(
						objects["c/"],
						[]metabase.ObjectEntry{prefixEntry("c//")},
						objects["c/1"],
					)...,
				),
			}.Check(ctx, t, db)

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c//",
				},
				Result: withoutPrefix("c//",
					objects["c//"]...,
				),
			}.Check(ctx, t, db)
		})

		t.Run("batch iterate committed versioned, unversioned, and delete markers with pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			var expected []metabase.ObjectEntry
			var objLocation metabase.ObjectLocation

			// create 1 pending object first
			pendingStream1 := metabasetest.RandObjectStream()
			objLocation = pendingStream1.Location()
			pendingStream1.Version = 100

			pendingObject1 := metabasetest.CreatePendingObject(ctx, t, db, pendingStream1, 0)

			expected = append(expected, objectEntryFromRaw(metabase.RawObject(pendingObject1)))

			for i := 0; i < 10; i++ {
				unversionedStream := metabasetest.RandObjectStream()
				unversionedStream.ProjectID = objLocation.ProjectID
				unversionedStream.BucketName = objLocation.BucketName
				unversionedStream.ObjectKey = objLocation.ObjectKey
				unversionedStream.Version = metabase.Version(200 + i)
				if i == 0 {
					metabasetest.CreateObject(ctx, t, db, unversionedStream, 0)
				} else {
					metabasetest.CreateObjectVersioned(ctx, t, db, unversionedStream, 0)
				}
			}

			// create a second pending object
			pendingStream2 := metabasetest.RandObjectStream()
			pendingStream2.ProjectID = objLocation.ProjectID
			pendingStream2.BucketName = objLocation.BucketName
			pendingStream2.ObjectKey = objLocation.ObjectKey
			pendingStream2.Version = 300

			pendingObject2 := metabasetest.CreatePendingObject(ctx, t, db, pendingStream2, 0)

			expected = append(expected, objectEntryFromRaw(metabase.RawObject(pendingObject2)))

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             objLocation.ProjectID,
					BucketName:            objLocation.BucketName,
					Pending:               true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					BatchSize:             3,
					Recursive:             true,
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("final prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"\xff\x00",
				"\xffA",
				"\xff\xff",
			})

			metabasetest.IterateObjectsWithStatusAscending{
				Opts: metabase.IterateObjectsWithStatus{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					Prefix:                "\xff",
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: withoutPrefix("\xff",
					objects["\xff\x00"],
					objects["\xffA"],
					objects["\xff\xff"],
				),
			}.Check(ctx, t, db)
		})
	})
}

func TestIterateObjectsSkipCursor(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
					prefixEntry(metabase.ObjectKey("08/")),
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
					prefixEntry(metabase.ObjectKey("08/")),
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
				},
			}.Check(ctx, t, db)
		})

		t.Run("batch-size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"2017/05/08",
				"2017/05/08/a",
				"2017/05/08/b",
				"2017/05/08/c",
				"2017/05/08/d",
				"2017/05/08/e",
				"2017/05/08" + metabase.DelimiterNext,
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
					prefixEntry(metabase.ObjectKey("08/")),
					withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
						Version: metabase.MaxVersion,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
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
						Version: metabase.MaxVersion,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.ObjectEntry{
					withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
					prefixEntry(metabase.ObjectKey("09/")),
					prefixEntry(metabase.ObjectKey("10/")),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestIterateObjectsWithStatus_Delimiter(t *testing.T) {
	for _, tt := range []struct {
		name      string
		ascending bool
	}{
		{"Descending", false}, {"Ascending", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testListObjectsDelimiter(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, testCase listObjectsDelimiterTestCase) ([]metabase.ObjectEntry, error) {
				var result metabasetest.IterateCollector

				iter := db.IterateObjectsAllVersionsWithStatus
				if tt.ascending {
					iter = db.IterateObjectsAllVersionsWithStatusAscending
				}
				err := iter(ctx, metabase.IterateObjectsWithStatus{
					ProjectID:             testCase.projectID,
					BucketName:            testCase.bucketName,
					Prefix:                testCase.prefix,
					Delimiter:             testCase.delimiter,
					IncludeSystemMetadata: true,
				}, result.Add)

				return []metabase.ObjectEntry(result), err
			})
		})
	}
}

func createObjects(ctx *testcontext.Context, t *testing.T, db *metabase.DB, numberOfObjects int, projectID uuid.UUID, bucketName metabase.BucketName) []metabase.RawObject {
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

func createObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName metabase.BucketName, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.ObjectEntry, len(keys))
	for _, key := range keys {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key
		now := time.Now()

		metabasetest.CreateObject(ctx, t, db, obj, 0)

		objects[key] = metabase.ObjectEntry{
			IsLatest:   true,
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

func prefixEntry(key metabase.ObjectKey) metabase.ObjectEntry {
	return metabase.ObjectEntry{
		IsPrefix:  true,
		ObjectKey: key,
		Status:    metabase.Prefix,
	}
}

func objectEntryFromRaw(m metabase.RawObject) metabase.ObjectEntry {
	return metabase.ObjectEntry{
		IsLatest:           false,
		IsPrefix:           false,
		ObjectKey:          m.ObjectKey,
		Version:            m.Version,
		StreamID:           m.StreamID,
		CreatedAt:          m.CreatedAt,
		ExpiresAt:          m.ExpiresAt,
		Status:             m.Status,
		SegmentCount:       m.SegmentCount,
		EncryptedUserData:  m.EncryptedUserData,
		TotalEncryptedSize: m.TotalEncryptedSize,
		TotalPlainSize:     m.TotalPlainSize,
		FixedSegmentSize:   m.FixedSegmentSize,
		Encryption:         m.Encryption,
	}
}

func objectEntryFromRawLatest(m metabase.RawObject) metabase.ObjectEntry {
	obj := objectEntryFromRaw(m)
	obj.IsLatest = true
	return obj
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

func TestTupleGreaterThanSQLEvaluate(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		adapter := db.ChooseAdapter(uuid.UUID{})
		evaluateSQL := func(expr string) (response bool) {
			switch ad := adapter.(type) {
			case *metabase.PostgresAdapter:
				rawDB := ad.UnderlyingDB()
				row := rawDB.QueryRowContext(ctx, "SELECT "+expr)
				require.NoError(t, row.Err())
				require.NoError(t, row.Scan(&response))
			case *metabase.CockroachAdapter:
				rawDB := ad.UnderlyingDB()
				row := rawDB.QueryRowContext(ctx, "SELECT "+expr)
				require.NoError(t, row.Err())
				require.NoError(t, row.Scan(&response))
			case *metabase.SpannerAdapter:
				rawDB := ad.UnderlyingDB()
				result := rawDB.Single().Query(ctx, spanner.Statement{SQL: "SELECT " + expr})
				row, err := result.Next()
				require.NoError(t, err)
				require.NoError(t, row.Columns(&response))
			default:
				t.Skipf("unknown adapter type %T", adapter)
			}
			return response
		}

		expectGreater := func(a, b []string) {
			expr1, err := spannerutil.TupleGreaterThanSQL(a, b, false)
			require.NoError(t, err)
			assert.True(t, evaluateSQL(expr1), expr1)
			expr2, err := spannerutil.TupleGreaterThanSQL(b, a, false)
			require.NoError(t, err)
			assert.False(t, evaluateSQL(expr2), expr2)
			expr3, err := spannerutil.TupleGreaterThanSQL(a, b, true)
			require.NoError(t, err)
			assert.True(t, evaluateSQL(expr3), expr3)
			expr4, err := spannerutil.TupleGreaterThanSQL(b, a, true)
			require.NoError(t, err)
			assert.False(t, evaluateSQL(expr4), expr4)
		}
		expectEqual := func(a, b []string) {
			expr1, err := spannerutil.TupleGreaterThanSQL(a, b, true)
			require.NoError(t, err)
			assert.True(t, evaluateSQL(expr1), expr1)
			expr2, err := spannerutil.TupleGreaterThanSQL(b, a, true)
			require.NoError(t, err)
			assert.True(t, evaluateSQL(expr2), expr2)
			expr3, err := spannerutil.TupleGreaterThanSQL(a, b, false)
			require.NoError(t, err)
			assert.False(t, evaluateSQL(expr3), expr3)
			expr4, err := spannerutil.TupleGreaterThanSQL(b, a, false)
			require.NoError(t, err)
			assert.False(t, evaluateSQL(expr4), expr4)
		}

		expectGreater([]string{"0", "0", "1"}, []string{"0", "0", "0"})
		expectGreater([]string{"0", "1", "0"}, []string{"0", "0", "0"})
		expectGreater([]string{"1", "0", "0"}, []string{"0", "0", "0"})
		expectGreater([]string{"1", "0", "0"}, []string{"0", "1", "1"})
		expectGreater([]string{"1", "0", "1"}, []string{"1", "0", "0"})
		expectGreater([]string{"1", "1", "1"}, []string{"1", "1", "0"})
		expectGreater([]string{"1"}, []string{"0"})
		expectEqual([]string{"0", "1", "0"}, []string{"0", "1", "0"})
		expectEqual([]string{"0"}, []string{"0"})

	})
}

func concat[E any](slices ...[]E) (concatenated []E) {
	for _, s := range slices {
		concatenated = append(concatenated, s...)
	}
	return concatenated
}

func last[E any](someSlice []E) E {
	return someSlice[len(someSlice)-1]
}
