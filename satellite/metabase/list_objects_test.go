// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestListObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("ProjectID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts:     metabase.ListObjects{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketName missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID: obj.ProjectID,
					Limit:     -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					Limit:      -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					Pending:    false,
				},
				Result: metabase.ListObjectsResult{},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.ObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				if delimiterIndex := strings.Index(string(obj.ObjectKey), string(metabase.Delimiter)); delimiterIndex > -1 {
					expected[i] = metabase.ObjectEntry{
						IsPrefix:  true,
						ObjectKey: obj.ObjectKey[:delimiterIndex+1],
						Status:    3,
					}
				} else {
					expected[i] = objectEntryFromRaw(obj)
				}
			}
			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             false,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					Limit:                 limit,
				},
				Result: metabase.ListObjectsResult{
					Objects: expected,
					More:    false,
				}}.Check(ctx, t, db)
			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.ObjectEntry, limit)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects[:limit] {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Limit:                 limit,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: expected,
					More:    true,
				}}.Check(ctx, t, db)
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "bucket-a",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: expected,
				}}.Check(ctx, t, db)

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: expected,
				}}.Check(ctx, t, db)

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
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
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version + 1},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
						objects["c"],
						objects["c/"],
						objects["c//"],
						objects["c/1"],
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "b", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
						objects["c"],
						objects["c/"],
						objects["c//"],
						objects["c/1"],
						objects["g"],
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "a"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/2"],
						objects["b/3"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{},
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a"],
						prefixEntry("b/", metabase.CommittedUnversioned),
						objects["c"],
						prefixEntry("c/", metabase.CommittedUnversioned),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version + 1},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/", metabase.CommittedUnversioned),
						objects["c"],
						prefixEntry("c/", metabase.CommittedUnversioned),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "b", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/", metabase.CommittedUnversioned),
						objects["c"],
						prefixEntry("c/", metabase.CommittedUnversioned),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "a"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/2"],
						objects["b/3"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c/",
						objects["c/"],
						prefixEntry("c//", metabase.CommittedUnversioned),
						objects["c/1"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c//",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c//",
						objects["c//"],
					)},
			}.Check(ctx, t, db)
		})

	})
}

func TestListObjectsSkipCursor(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID, bucketName := uuid.UUID{1}, "bucky"

		t.Run("no prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"08/test",
				"09/test",
				"10/test",
			})

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("08"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     "",
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)
		})

		t.Run("prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"2017/05/08/test",
				"2017/05/09/test",
				"2017/05/10/test",
			})

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08"),
						Version: objects["2017/05/08"].Version,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/"), metabase.CommittedUnversioned),
						withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					//BatchSize:  3,
					Prefix: metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08/"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Recursive:  false,
					Prefix:     metabase.ObjectKey("2017/05/"),
					Cursor: metabase.ListObjectsCursor{
						Key:     metabase.ObjectKey("2017/05/08/a/x"),
						Version: 1,
					},
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
						prefixEntry(metabase.ObjectKey("09/"), metabase.CommittedUnversioned),
						prefixEntry(metabase.ObjectKey("10/"), metabase.CommittedUnversioned),
					}},
			}.Check(ctx, t, db)
		})
	})
}

func BenchmarkNonRecursiveObjectsListing(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		baseObj := metabasetest.RandObjectStream()

		batchsize := 5
		for i := 0; i < 500; i++ {
			metabasetest.CreateObject(ctx, b, db, metabasetest.RandObjectStream(), 0)
		}

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
				result, err := db.ListObjects(ctx, metabase.ListObjects{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					Pending:    false,
					Limit:      batchsize,
				})
				require.NoError(b, err)
				for result.More {
					result, err = db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:  baseObj.ProjectID,
						BucketName: baseObj.BucketName,
						Cursor:     metabase.ListObjectsCursor{Key: result.Objects[len(result.Objects)-1].ObjectKey},
						Pending:    false,
						Limit:      batchsize,
					})
					require.NoError(b, err)
				}
			}
		})

		b.Run("listing with prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result, err := db.ListObjects(ctx, metabase.ListObjects{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					Prefix:     "foo/",
					Pending:    false,
					Limit:      batchsize,
				})
				require.NoError(b, err)
				for result.More {
					cursorKey := "foo/" + result.Objects[len(result.Objects)-1].ObjectKey
					result, err = db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:  baseObj.ProjectID,
						BucketName: baseObj.BucketName,
						Prefix:     "foo/",
						Cursor:     metabase.ListObjectsCursor{Key: cursorKey},
						Pending:    false,
						Limit:      batchsize,
					})
					require.NoError(b, err)
				}
			}
		})

		b.Run("listing only prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result, err := db.ListObjects(ctx, metabase.ListObjects{
					ProjectID:  baseObj.ProjectID,
					BucketName: baseObj.BucketName,
					Prefix:     "boo/",
					Pending:    false,
					Limit:      batchsize,
				})
				require.NoError(b, err)
				for result.More {
					cursorKey := "boo/" + result.Objects[len(result.Objects)-1].ObjectKey
					result, err = db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:  baseObj.ProjectID,
						BucketName: baseObj.BucketName,
						Prefix:     "boo/",
						Cursor:     metabase.ListObjectsCursor{Key: cursorKey},
						Pending:    false,
						Limit:      batchsize,
					})
					require.NoError(b, err)

				}
			}
		})
	})
}

func TestListObjectsVersioned(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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
			b1 := a0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
					},
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
			b1 := a0
			b1.Version = 500

			objA0 := metabasetest.CreateObjectVersioned(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreateObjectVersioned(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersioned(ctx, t, db, b1, 0)

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objB0)),
					},
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

		// TODO(ver): more exhaustive tests (committed/deletemarker, unversioned/versioned)
		// TODO(ver): test with non-recursive listing
	})
}
