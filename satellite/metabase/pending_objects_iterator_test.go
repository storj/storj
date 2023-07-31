// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestIteratePendingObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("invalid arguments", func(t *testing.T) {
			t.Run("ProjectID missing", func(t *testing.T) {
				metabasetest.IteratePendingObjects{
					Opts: metabase.IteratePendingObjects{
						ProjectID:  uuid.UUID{},
						BucketName: "sj://mybucket",
						Recursive:  true,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ProjectID missing",
				}.Check(ctx, t, db)
			})
			t.Run("BucketName missing", func(t *testing.T) {
				metabasetest.IteratePendingObjects{
					Opts: metabase.IteratePendingObjects{
						ProjectID:  uuid.UUID{1},
						BucketName: "",
						Recursive:  true,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "BucketName missing",
				}.Check(ctx, t, db)
			})
			t.Run("Limit is negative", func(t *testing.T) {
				metabasetest.IteratePendingObjects{
					Opts: metabase.IteratePendingObjects{
						ProjectID:  uuid.UUID{1},
						BucketName: "mybucket",
						BatchSize:  -1,
						Recursive:  true,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "BatchSize is negative",
				}.Check(ctx, t, db)
			})
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			objects := createObjects(ctx, t, db, 2, uuid.UUID{1}, "mybucket")
			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  uuid.UUID{1},
					BucketName: "myemptybucket",
					BatchSize:  10,
					Recursive:  true,
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			pending := metabasetest.RandObjectStream()
			pending.Version = metabase.NextVersion
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName
			committed.Version = metabase.NextVersion

			projectID := pending.ProjectID
			bucketName := pending.BucketName

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           pending,
					Encryption:             metabasetest.DefaultEncryption,
					UsePendingObjectsTable: true,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           committed,
					Encryption:             metabasetest.DefaultEncryption,
					UsePendingObjectsTable: true,
				},
				Version: 1,
			}.Check(ctx, t, db)
			committed.Version++

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					UsePendingObjectsTable:        true,
				},
			}.Check(ctx, t, db)

			// IteratePendingObjects should find only pending object
			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.PendingObjectEntry{{
					ObjectKey:  pending.ObjectKey,
					StreamID:   pending.StreamID,
					CreatedAt:  now,
					Encryption: metabasetest.DefaultEncryption,
				}},
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.PendingObjectEntry, numberOfObjects)
			expectedObjects := createPendingObjects(ctx, t, db, numberOfObjects, 0, uuid.UUID{1}, "mybucket")
			for i, object := range expectedObjects {
				expected[i] = pendingObjectEntryFromRaw(object)
			}
			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{PendingObjects: expectedObjects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.PendingObjectEntry, numberOfObjects)
			expectedObjects := createPendingObjects(ctx, t, db, numberOfObjects, 0, uuid.UUID{1}, "mybucket")
			for i, object := range expectedObjects {
				expected[i] = pendingObjectEntryFromRaw(object)
			}
			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					BatchSize:             limit,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)
			metabasetest.Verify{PendingObjects: expectedObjects}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket in project with 2 buckets", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5

			expected := make([]metabase.PendingObjectEntry, numberOfObjectsPerBucket)

			objectsBucketA := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, 0, uuid.UUID{1}, "bucket-a")
			objectsBucketB := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, 0, uuid.UUID{1}, "bucket-b")
			for i, obj := range objectsBucketA {
				expected[i] = pendingObjectEntryFromRaw(obj)
			}

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "bucket-a",
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				PendingObjects: append(objectsBucketA, objectsBucketB...),
			}.Check(ctx, t, db)
		})

		t.Run("objects in one bucket with same bucketName in another project", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjectsPerBucket := 5

			expected := make([]metabase.PendingObjectEntry, numberOfObjectsPerBucket)

			objectsProject1 := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, 0, uuid.UUID{1}, "mybucket")
			objectsProject2 := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, 0, uuid.UUID{2}, "mybucket")
			for i, obj := range objectsProject1 {
				expected[i] = pendingObjectEntryFromRaw(obj)
			}

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				PendingObjects: append(objectsProject1, objectsProject2...),
			}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.PendingObjectEntry{
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

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.PendingObjectsCursor{Key: "a", StreamID: uuid.Max()},
				},
				Result: []metabase.PendingObjectEntry{
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

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,

					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.PendingObjectsCursor{Key: "b"},
				},
				Result: []metabase.PendingObjectEntry{
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

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,

					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,

					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "a"},
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,

					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "b/2", StreamID: uuid.UUID{1}},
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  true,

					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)
		})

		t.Run("non-recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: []metabase.PendingObjectEntry{
					objects["a"],
					pendingPrefixEntry("b/"),
					objects["c"],
					pendingPrefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.PendingObjectsCursor{Key: "a", StreamID: uuid.Max()},
				},
				Result: []metabase.PendingObjectEntry{
					pendingPrefixEntry("b/"),
					objects["c"],
					pendingPrefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.PendingObjectsCursor{Key: "b"},
				},
				Result: []metabase.PendingObjectEntry{
					pendingPrefixEntry("b/"),
					objects["c"],
					pendingPrefixEntry("c/"),
					objects["g"],
				},
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "a"},
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/1"],
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "b/2", StreamID: uuid.UUID{1}},
				},
				Result: pendingWithoutPrefix("b/",
					objects["b/2"],
					objects["b/3"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.PendingObjectsCursor{Key: "c/"},
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.PendingObjectsCursor{Key: "c/", StreamID: uuid.UUID{1}},
				},
				Result: pendingWithoutPrefix("c/",
					objects["c/"],
					pendingPrefixEntry("c//"),
					objects["c/1"],
				),
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c//",
				},
				Result: pendingWithoutPrefix("c//",
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

			createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, queries[1:])

			var collector metabasetest.PendingObjectsCollector
			for _, cursor := range queries {
				for _, prefix := range queries {
					collector = collector[:0]
					err := db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.PendingObjectsCursor{
							Key:      cursor,
							StreamID: uuid.UUID{},
						},
						Prefix:                prefix,
						IncludeCustomMetadata: true,
					}, collector.Add)
					require.NoError(t, err)

					collector = collector[:0]
					err = db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Cursor: metabase.PendingObjectsCursor{
							Key:      cursor,
							StreamID: uuid.UUID{},
						},
						Prefix:                prefix,
						Recursive:             true,
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
			var collector metabasetest.PendingObjectsCollector
			err := db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
				ProjectID:  projectID,
				BucketName: bucketName,
				Cursor: metabase.PendingObjectsCursor{
					Key:      metabase.ObjectKey([]byte{}),
					StreamID: uuid.UUID{},
				},
				Prefix:                metabase.ObjectKey([]byte{1}),
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: true,
			}, collector.Add)
			require.NoError(t, err)
		})

		t.Run("include/exclude custom metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			obj1.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: obj1,
					Encryption:   metabasetest.DefaultEncryption,

					EncryptedMetadata:             []byte{3},
					EncryptedMetadataEncryptedKey: []byte{4},
					EncryptedMetadataNonce:        []byte{5},
					UsePendingObjectsTable:        true,
				},
				Version: 1,
			}.Check(ctx, t, db)

			// include custom metadata
			var collector metabasetest.PendingObjectsCollector
			err := db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: true,
			}, collector.Add)
			require.NoError(t, err)
			require.Len(t, collector, 1)

			for _, entry := range collector {
				require.Equal(t, entry.EncryptedMetadata, []byte{3})
				require.Equal(t, entry.EncryptedMetadataEncryptedKey, []byte{4})
				require.Equal(t, entry.EncryptedMetadataNonce, []byte{5})
			}

			// exclude custom metadata
			collector = collector[:0]
			err = db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				IncludeCustomMetadata: false,
				IncludeSystemMetadata: true,
			}, collector.Add)
			require.NoError(t, err)
			require.Len(t, collector, 1)

			for _, entry := range collector {
				require.Nil(t, entry.EncryptedMetadataNonce)
				require.Nil(t, entry.EncryptedMetadata)
				require.Nil(t, entry.EncryptedMetadataEncryptedKey)
			}
		})

		t.Run("exclude system metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			obj1.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: obj1,
					Encryption:   metabasetest.DefaultEncryption,

					EncryptedMetadata:             []byte{3},
					EncryptedMetadataEncryptedKey: []byte{4},
					EncryptedMetadataNonce:        []byte{5},
					UsePendingObjectsTable:        true,
				},
				Version: 1,
			}.Check(ctx, t, db)

			var collector metabasetest.PendingObjectsCollector
			err := db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
				ProjectID:             obj1.ProjectID,
				BucketName:            obj1.BucketName,
				Recursive:             true,
				IncludeCustomMetadata: true,
				IncludeSystemMetadata: false,
			}, collector.Add)

			require.NoError(t, err)
			require.Len(t, collector, 1)

			for _, entry := range collector {
				// fields that should always be set
				require.NotEmpty(t, entry.ObjectKey)
				require.NotEmpty(t, entry.StreamID)
				require.False(t, entry.Encryption.IsZero())

				require.True(t, entry.CreatedAt.IsZero())
				require.Nil(t, entry.ExpiresAt)

				require.NotNil(t, entry.EncryptedMetadataNonce)
				require.NotNil(t, entry.EncryptedMetadata)
				require.NotNil(t, entry.EncryptedMetadataEncryptedKey)
			}
		})

		t.Run("verify-cursor-continuation", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"
			createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"1",
				"a/a",
				"a/0",
			})
			var collector metabasetest.PendingObjectsCollector
			err := db.IteratePendingObjects(ctx, metabase.IteratePendingObjects{
				ProjectID:  projectID,
				BucketName: bucketName,
				Prefix:     metabase.ObjectKey("a/"),
				BatchSize:  1,
			}, collector.Add)
			require.NoError(t, err)
			require.Len(t, collector, 2)
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
				ProjectID:  testrand.UUID(),
				BucketName: "bucket",
			}
			for i, tc := range testCases {
				tc := tc
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					expectedResult := []metabase.PendingObjectEntry{}
					if len(tc.notExpired) == 0 {
						expectedResult = nil
					}
					for _, key := range tc.notExpired {
						stream.ObjectKey = key
						stream.StreamID = testrand.UUID()
						metabasetest.CreatePendingObjectNew(ctx, t, db, stream, 0)
						expectedResult = append(expectedResult, metabase.PendingObjectEntry{
							ObjectKey:  key,
							StreamID:   stream.StreamID,
							CreatedAt:  now,
							Encryption: metabasetest.DefaultEncryption,
						})
					}
					for _, key := range tc.expired {
						stream := stream
						stream.ObjectKey = key
						stream.StreamID = testrand.UUID()
						stream.Version = metabase.NextVersion

						expiresAt := now.Add(-2 * time.Hour)
						metabasetest.BeginObjectNextVersion{
							Opts: metabase.BeginObjectNextVersion{
								ObjectStream: stream,
								Encryption:   metabasetest.DefaultEncryption,

								ExpiresAt: &expiresAt,

								UsePendingObjectsTable: true,
							},
							Version: 1,
						}.Check(ctx, t, db)
					}
					for _, batchSize := range []int{1, 2, 3} {
						opts := metabase.IteratePendingObjects{
							ProjectID:             stream.ProjectID,
							BucketName:            stream.BucketName,
							BatchSize:             batchSize,
							IncludeSystemMetadata: true,
						}
						metabasetest.IteratePendingObjects{
							Opts:   opts,
							Result: expectedResult,
						}.Check(ctx, t, db)
						{
							opts := opts
							opts.Recursive = true
							metabasetest.IteratePendingObjects{
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
			objects := createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"aaaa/a",
				"aaaa/b",
				"aaaa/c",
			})

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             false,
					Prefix:                "aaaa/",
					BatchSize:             2,
					IncludeSystemMetadata: true,
				},
				Result: pendingWithoutPrefix("aaaa/",
					objects["aaaa/a"],
					objects["aaaa/b"],
					objects["aaaa/c"],
				),
			}.Check(ctx, t, db)
		})

		t.Run("two objects the same object key", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectID, bucketName := uuid.UUID{2}, "bucky"

			id1 := metabasetest.RandObjectStream()
			id1.ProjectID = projectID
			id1.BucketName = bucketName

			id2 := metabasetest.RandObjectStream()
			id2.ProjectID = projectID
			id2.BucketName = bucketName
			id2.ObjectKey = id1.ObjectKey

			if id2.StreamID.Less(id1.StreamID) {
				id1.StreamID, id2.StreamID = id2.StreamID, id1.StreamID
			}

			var objs []metabase.PendingObjectEntry
			for _, id := range []metabase.ObjectStream{id1, id2} {
				id.Version = metabase.NextVersion
				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream:           id,
						Encryption:             metabasetest.DefaultEncryption,
						UsePendingObjectsTable: true,
					},
					Version: 1,
				}.Check(ctx, t, db)

				objs = append(objs, metabase.PendingObjectEntry{
					ObjectKey:  id.ObjectKey,
					StreamID:   id.StreamID,
					CreatedAt:  time.Now(),
					Encryption: metabasetest.DefaultEncryption,
				})
			}

			metabasetest.IteratePendingObjects{
				Opts: metabase.IteratePendingObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					BatchSize:             1,
					IncludeSystemMetadata: true,
				},
				Result: objs,
			}.Check(ctx, t, db)
		})
	})
}

func createPendingObjects(ctx *testcontext.Context, t *testing.T, db *metabase.DB, numberOfObjects int, numberOfSegments int, projectID uuid.UUID, bucketName string) []metabase.RawPendingObject {
	objects := make([]metabase.RawPendingObject, numberOfObjects)
	for i := 0; i < numberOfObjects; i++ {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.Version = metabase.NextVersion
		now := time.Now()

		metabasetest.BeginObjectNextVersion{
			Opts: metabase.BeginObjectNextVersion{
				ObjectStream:           obj,
				Encryption:             metabasetest.DefaultEncryption,
				UsePendingObjectsTable: true,
			},
			Version: 1,
		}.Check(ctx, t, db)
		obj.Version++

		for i := 0; i < numberOfSegments; i++ {
			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
					RootPieceID:  storj.PieceID{byte(i) + 1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					UsePendingObjectsTable: true,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
					RootPieceID:  storj.PieceID{1},
					Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},
					EncryptedETag:     []byte{5},

					EncryptedSize:          1024,
					PlainSize:              512,
					PlainOffset:            0,
					Redundancy:             metabasetest.DefaultRedundancy,
					UsePendingObjectsTable: true,
				},
			}.Check(ctx, t, db)
		}

		zombieDeletionDeadline := time.Now().Add(24 * time.Hour)
		objects[i] = metabase.RawPendingObject{
			PendingObjectStream:    metabasetest.ObjectStreamToPending(obj),
			CreatedAt:              now,
			Encryption:             metabasetest.DefaultEncryption,
			ZombieDeletionDeadline: &zombieDeletionDeadline,
		}
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return objects[i].ObjectKey < objects[j].ObjectKey
	})
	return objects
}

func pendingObjectEntryFromRaw(obj metabase.RawPendingObject) metabase.PendingObjectEntry {
	return metabase.PendingObjectEntry{
		ObjectKey:                     obj.ObjectKey,
		StreamID:                      obj.StreamID,
		ExpiresAt:                     obj.ExpiresAt,
		CreatedAt:                     obj.CreatedAt,
		Encryption:                    obj.Encryption,
		EncryptedMetadataNonce:        obj.EncryptedMetadataNonce,
		EncryptedMetadata:             obj.EncryptedMetadataNonce,
		EncryptedMetadataEncryptedKey: obj.EncryptedMetadataEncryptedKey,
	}
}

func createPendingObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.PendingObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.PendingObjectEntry, len(keys))
	for _, key := range keys {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key
		now := time.Now()

		metabasetest.CreatePendingObjectNew(ctx, t, db, obj, 0)

		objects[key] = metabase.PendingObjectEntry{
			ObjectKey:  obj.ObjectKey,
			StreamID:   obj.StreamID,
			CreatedAt:  now,
			Encryption: metabasetest.DefaultEncryption,
		}
	}

	return objects
}

func pendingWithoutPrefix(prefix metabase.ObjectKey, entries ...metabase.PendingObjectEntry) []metabase.PendingObjectEntry {
	xs := make([]metabase.PendingObjectEntry, len(entries))
	for i, e := range entries {
		xs[i] = e
		xs[i].ObjectKey = entries[i].ObjectKey[len(prefix):]
	}
	return xs
}

func pendingPrefixEntry(key metabase.ObjectKey) metabase.PendingObjectEntry {
	return metabase.PendingObjectEntry{
		IsPrefix:  true,
		ObjectKey: key,
	}
}
