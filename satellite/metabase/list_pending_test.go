// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestListPendingObjects(t *testing.T) {
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
					ProjectID:   obj.ProjectID,
					BucketName:  obj.BucketName,
					Pending:     true,
					AllVersions: true,
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
			objects := createPendingObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				if delimiterIndex := strings.Index(string(obj.ObjectKey), string(metabase.Delimiter)); delimiterIndex > -1 {
					expected[i] = metabase.ObjectEntry{
						IsPrefix:  true,
						ObjectKey: obj.ObjectKey[:delimiterIndex+1],
						Status:    metabase.Prefix,
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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
			objects := createPendingObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects[:limit] {
				expected[i] = objectEntryFromRaw(obj)
			}
			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Limit:                 limit,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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

			objectsBucketA := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-a")
			objectsBucketB := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "bucket-b")

			for i, obj := range objectsBucketA {
				expected[i] = objectEntryFromRaw(obj)
			}

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "bucket-a",
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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

			objectsProject1 := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{1}, "mybucket")
			objectsProject2 := createPendingObjects(ctx, t, db, numberOfObjectsPerBucket, uuid.UUID{2}, "mybucket")
			for i, obj := range objectsProject1 {
				expected[i] = objectEntryFromRaw(obj)
			}

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             uuid.UUID{1},
					BucketName:            "mybucket",
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
					IncludeETag:           true,
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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a"],
						prefixEntry("b/"),
						objects["c"],
						prefixEntry("c/"),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version + 1},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/"),
						objects["c"],
						prefixEntry("c/"),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "b", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/"),
						objects["c"],
						prefixEntry("c/"),
						objects["g"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c/",
						objects["c/"],
						prefixEntry("c//"),
						objects["c/1"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
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

func TestListPendingObjectsSkipCursor(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

		t.Run("no prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/")),
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
					}},
			}.Check(ctx, t, db)
		})

		t.Run("prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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
					Pending:               true,
					AllVersions:           true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/")),
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
					}},
			}.Check(ctx, t, db)
		})

		t.Run("batch-size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objects := createPendingObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry(metabase.ObjectKey("08/")),
						withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						withoutPrefix1("2017/05/", objects["2017/05/08"+metabase.DelimiterNext]),
						prefixEntry(metabase.ObjectKey("09/")),
						prefixEntry(metabase.ObjectKey("10/")),
					}},
			}.Check(ctx, t, db)
		})
	})
}

func TestListPendingObjectsVersions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {

		t.Run("2 objects, one with versions one without", func(t *testing.T) {
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

			objA0 := metabasetest.CreatePendingObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreatePendingObject(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreatePendingObject(ctx, t, db, b1, 0)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
					},
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
				},
			}.Check(ctx, t, db)
		})

		t.Run("3 objects, one with versions two without", func(t *testing.T) {
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

			streams := []metabase.ObjectStream{a0, b0, c0}
			sort.Slice(streams, func(i, j int) bool {
				return streams[i].Less(streams[j])
			})

			a0, b0, c0 = streams[0], streams[1], streams[2]

			b1 := b0
			b1.Version = 500

			objA0 := metabasetest.CreatePendingObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreatePendingObject(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreatePendingObject(ctx, t, db, b1, 0)
			objC0 := metabasetest.CreatePendingObject(ctx, t, db, c0, 0)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
						objectEntryFromRaw(metabase.RawObject(objC0)),
					},
				}}.Check(ctx, t, db)

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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						{
							ObjectKey: pendingObj.ObjectKey,
							Version:   pendingObj.Version,
							StreamID:  pendingObj.StreamID,
							CreatedAt: pendingObj.CreatedAt,
							Status:    metabase.Pending,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}}.Check(ctx, t, db)

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

			objA0 := metabasetest.CreatePendingObject(ctx, t, db, a0, 0)
			objA1 := metabasetest.CreatePendingObject(ctx, t, db, a1, 0)
			objB0 := metabasetest.CreatePendingObject(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreatePendingObject(ctx, t, db, b1, 0)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
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

		t.Run("list recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreatePendingObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			})

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a:1000"], objects["a:1001"],
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
						objects["c:1000"], objects["c:1001"],
						objects["c/:1000"], objects["c/:1001"],
						objects["c//:1000"], objects["c//:1001"],
						objects["c/1:1000"], objects["c/1:1001"],
						objects["g:1000"], objects["g:1001"],
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: 1002},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
						objects["c:1000"], objects["c:1001"],
						objects["c/:1000"], objects["c/:1001"],
						objects["c//:1000"], objects["c//:1001"],
						objects["c/1:1000"], objects["c/1:1001"],
						objects["g:1000"], objects["g:1001"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "b", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
						objects["c:1000"], objects["c:1001"],
						objects["c/:1000"], objects["c/:1001"],
						objects["c//:1000"], objects["c//:1001"],
						objects["c/1:1000"], objects["c/1:1001"],
						objects["g:1000"], objects["g:1001"],
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "a"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{},
			}.Check(ctx, t, db)
		})

		t.Run("list non-recursive objects with versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreatePendingObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
				"c/":  {1000, 1001},
				"c//": {1000, 1001},
				"c/1": {1000, 1001},
				"g":   {1000, 1001},
			})

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a:1000"], objects["a:1001"],
						prefixEntry("b/"),
						objects["c:1000"], objects["c:1001"],
						prefixEntry("c/"),
						objects["g:1000"], objects["g:1001"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: 1002},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/"),
						objects["c:1000"], objects["c:1001"],
						prefixEntry("c/"),
						objects["g:1000"], objects["g:1001"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "b", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/"),
						objects["c:1000"], objects["c:1001"],
						prefixEntry("c/"),
						objects["g:1000"], objects["g:1001"],
					}},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "a"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/1:1000"], objects["b/1:1001"],
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/2:1000"], objects["b/2:1001"],
						objects["b/3:1000"], objects["b/3:1001"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
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
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.ListObjectsCursor{Key: "c/"},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c/",
						objects["c/:1000"], objects["c/:1001"],
						prefixEntry("c//"),
						objects["c/1:1000"], objects["c/1:1001"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               true,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c//",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c//",
						objects["c//:1000"], objects["c//:1001"],
					)},
			}.Check(ctx, t, db)
		})
	})
}

func TestListPendingObjects_Limit(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		bucketName := metabase.BucketName(testrand.BucketName())

		numberOfObjects := 0

		const minVersion = -10000
		const maxVersion = 10000

		prefixes := []string{"", "aprefix/"}
		for _, prefix := range prefixes {
			for i := range 10 {
				metabasetest.CreatePendingObject(ctx, t, db, metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: bucketName,
					ObjectKey:  metabase.ObjectKey(prefix + "object" + strconv.Itoa(i)),
					Version:    metabase.Version(testrand.Int63n(maxVersion-minVersion+1) + minVersion),
					StreamID:   testrand.UUID(),
				}, 0)
				numberOfObjects++
			}
		}

		testCases := []struct {
			Prefix          metabase.ObjectKey
			Cursor          metabase.ObjectKey
			Recursive       bool
			ExpectedResults int
		}{
			0: {"", "", false, 11}, // 10 objects + prefix
			1: {"aprefix/", "", false, 10},
			2: {"", "", true, numberOfObjects},
			3: {"aprefix/", "", true, 10},
			4: {"", "object1", false, 8},
			5: {"", "object1", true, 8},
			6: {"aprefix/", "object1", true, 8},
		}
		listLimits := []int{1, 2, 3, 7, numberOfObjects - 1, numberOfObjects, numberOfObjects + 1}

		for i, test := range testCases {
			prefixLabel := test.Prefix
			if prefixLabel == "" {
				prefixLabel = "empty"
			}
			t.Run(fmt.Sprintf("#%d prefix %s", i, prefixLabel), func(t *testing.T) {
				for _, listLimit := range listLimits {
					t.Run(fmt.Sprintf("limit %d cursor %s rec %t", listLimit, test.Cursor, test.Recursive), func(t *testing.T) {

						objects, err := db.ListObjects(ctx, metabase.ListObjects{
							ProjectID:   projectID,
							BucketName:  bucketName,
							Pending:     true,
							AllVersions: true,

							Recursive: test.Recursive,
							Cursor: metabase.ListObjectsCursor{
								Key:     test.Prefix + test.Cursor,
								Version: metabase.MaxVersion,
							},
							Prefix: test.Prefix,

							Limit: listLimit,
						})
						require.NoError(t, err)

						if listLimit < test.ExpectedResults {
							require.Equal(t, listLimit, len(objects.Objects))
							require.Equal(t, true, objects.More)
						} else {
							require.Equal(t, test.ExpectedResults, len(objects.Objects))
							require.Equal(t, false, objects.More)
						}
					})
				}
			})
		}
	})
}

func TestListObjectsPendingDuplicates(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()

		const amount = 23
		const minVersion = -10000
		const maxVersion = 10000

		type TestCase struct {
			Name       string
			Bucket     string
			Prefixes   []string
			UploadFunc func(bucket string, prefixe string) map[metabase.ObjectStream]struct{}
		}

		testCases := []TestCase{
			{
				Name:     "single location many pending objects",
				Bucket:   "test1",
				Prefixes: []string{"", "aprefix/"},
				UploadFunc: func(bucket string, prefix string) map[metabase.ObjectStream]struct{} {
					// upload objects to the same location to have many pending objects
					// with different versions
					expectedKeys := make(map[metabase.ObjectStream]struct{})
					for range amount {
						object := metabasetest.CreatePendingObject(ctx, t, db, metabase.ObjectStream{
							ProjectID:  projectID,
							BucketName: metabase.BucketName(bucket),
							ObjectKey:  metabase.ObjectKey(prefix + "object"),
							Version:    metabase.Version(testrand.Int63n(maxVersion-minVersion+1) + minVersion),
							StreamID:   testrand.UUID(),
						}, 0)
						expectedKeys[object.ObjectStream] = struct{}{}
					}
					return expectedKeys
				},
			},
			{
				Name:     "many locations many pending objects",
				Bucket:   "test2",
				Prefixes: []string{"", "aprefix/"},
				UploadFunc: func(bucket string, prefix string) map[metabase.ObjectStream]struct{} {
					// upload to the same location many times to have internally different versions
					expectedKeys := make(map[metabase.ObjectStream]struct{})
					for i := range amount {
						version := 1
						if i%2 == 0 {
							version = 2
						} else if i%3 == 0 {
							version = 3
						}

						for v := 0; v < version; v++ {
							object := metabasetest.CreatePendingObject(ctx, t, db, metabase.ObjectStream{
								ProjectID:  projectID,
								BucketName: metabase.BucketName(bucket),
								ObjectKey:  metabase.ObjectKey(prefix + fmt.Sprintf("object-%d", i)),
								Version:    metabase.Version(testrand.Int63n(maxVersion-minVersion+1) + minVersion),
								StreamID:   testrand.UUID(),
							}, 0)
							expectedKeys[object.ObjectStream] = struct{}{}
						}
					}
					return expectedKeys
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.Name, func(t *testing.T) {
				for _, prefix := range testCase.Prefixes {
					expectedKeys := testCase.UploadFunc(testCase.Bucket, prefix)

					prefixLabel := prefix
					if prefixLabel == "" {
						prefixLabel = "empty"
					}

					for _, listLimit := range []int{
						0, 1, 2, 3, 7, amount - 1, amount, amount + 1,
					} {
						t.Run(fmt.Sprintf("prefix %s limit %d", prefixLabel, listLimit), func(t *testing.T) {
							keys := make(map[metabase.ObjectStream]struct{})

							more := true
							cursor := metabase.ListObjectsCursor{}
							for more {
								result, err := db.ListObjects(ctx, metabase.ListObjects{
									ProjectID:   projectID,
									BucketName:  metabase.BucketName(testCase.Bucket),
									Prefix:      metabase.ObjectKey(prefix),
									Limit:       listLimit,
									Pending:     true,
									AllVersions: true,
									Cursor:      cursor,
								})
								require.NoError(t, err)

								for _, object := range result.Objects {
									keys[metabase.ObjectStream{
										ProjectID:  projectID,
										BucketName: metabase.BucketName(testCase.Bucket),
										ObjectKey:  metabase.ObjectKey(prefix) + object.ObjectKey,
										Version:    object.Version,
										StreamID:   object.StreamID,
									}] = struct{}{}

									cursor = metabase.ListObjectsCursor{
										Key:     metabase.ObjectKey(prefix) + object.ObjectKey,
										Version: object.Version,
									}
								}
								more = result.More
							}

							require.Equal(t, expectedKeys, keys)
						})
					}
				}
			})
		}
	})
}

func TestListPendingObjectsWithNegativeVersions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		bucketName := metabase.BucketName(testrand.BucketName())

		t.Run("list pending objects with negative versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// Create pending objects with negative versions
			objects := make([]metabase.RawObject, 0, 5)
			for range 5 {
				obj := metabasetest.RandObjectStream()
				obj.ProjectID = projectID
				obj.BucketName = bucketName
				obj.Version = metabase.Version(-1 * testrand.Int63n(math.MaxInt64))

				pendingObject := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				objects = append(objects, metabase.RawObject{
					ObjectStream:           pendingObject.ObjectStream,
					CreatedAt:              pendingObject.CreatedAt,
					Status:                 metabase.Pending,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: pendingObject.ZombieDeletionDeadline,
				})
			}

			// Sort objects by key and version (ascending)
			sort.SliceStable(objects, func(i, j int) bool {
				if objects[i].ObjectKey != objects[j].ObjectKey {
					return objects[i].ObjectKey < objects[j].ObjectKey
				}
				return objects[i].Version < objects[j].Version
			})

			// List all pending objects
			result, err := db.ListObjects(ctx, metabase.ListObjects{
				ProjectID:   projectID,
				BucketName:  bucketName,
				Pending:     true,
				AllVersions: true,
				Recursive:   true,
				Limit:       10,
			})
			require.NoError(t, err)
			require.Equal(t, len(objects), len(result.Objects))
			require.False(t, result.More)

			// Verify all objects are returned in correct order
			for i, expected := range objects {
				require.Equal(t, expected.ObjectKey, result.Objects[i].ObjectKey)
				require.Equal(t, expected.Version, result.Objects[i].Version)
				require.Equal(t, expected.StreamID, result.Objects[i].StreamID)
			}

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("list pending objects with negative versions using cursor", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// Create pending objects with negative versions
			objects := make([]metabase.RawObject, 0, 5)
			for range 5 {
				obj := metabasetest.RandObjectStream()
				obj.ProjectID = projectID
				obj.BucketName = bucketName
				obj.Version = metabase.Version(-1 * testrand.Int63n(math.MaxInt64))

				pendingObject := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				objects = append(objects, metabase.RawObject{
					ObjectStream:           pendingObject.ObjectStream,
					CreatedAt:              pendingObject.CreatedAt,
					Status:                 metabase.Pending,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: pendingObject.ZombieDeletionDeadline,
				})
			}

			// Sort objects by key and version (ascending)
			sort.SliceStable(objects, func(i, j int) bool {
				if objects[i].ObjectKey != objects[j].ObjectKey {
					return objects[i].ObjectKey < objects[j].ObjectKey
				}
				return objects[i].Version < objects[j].Version
			})

			// List with pagination using limit
			var allListedObjects []metabase.ObjectEntry
			cursor := metabase.ListObjectsCursor{}

			for {
				result, err := db.ListObjects(ctx, metabase.ListObjects{
					ProjectID:   projectID,
					BucketName:  bucketName,
					Pending:     true,
					AllVersions: true,
					Recursive:   true,
					Limit:       2,
					Cursor:      cursor,
				})
				require.NoError(t, err)

				allListedObjects = append(allListedObjects, result.Objects...)

				if !result.More {
					break
				}

				// Set cursor to last returned object
				lastObj := result.Objects[len(result.Objects)-1]
				cursor = metabase.ListObjectsCursor{
					Key:     lastObj.ObjectKey,
					Version: lastObj.Version,
				}
			}

			// Verify all objects were listed
			require.Equal(t, len(objects), len(allListedObjects))

			for i, expected := range objects {
				require.Equal(t, expected.ObjectKey, allListedObjects[i].ObjectKey)
				require.Equal(t, expected.Version, allListedObjects[i].Version)
				require.Equal(t, expected.StreamID, allListedObjects[i].StreamID)
			}

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})
	})
}

func createPendingObjects(ctx *testcontext.Context, t *testing.T, db *metabase.DB, numberOfObjects int, projectID uuid.UUID, bucketName metabase.BucketName) []metabase.RawObject {
	objects := make([]metabase.RawObject, numberOfObjects)
	for i := 0; i < numberOfObjects; i++ {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		userData := metabasetest.RandEncryptedUserData()
		now := time.Now()

		object := metabasetest.BeginObjectExactVersion{
			Opts: metabase.BeginObjectExactVersion{
				ObjectStream:      obj,
				Encryption:        metabasetest.DefaultEncryption,
				EncryptedUserData: userData,
			},
		}.Check(ctx, t, db)

		objects[i] = metabase.RawObject{
			ObjectStream:      obj,
			CreatedAt:         now,
			Status:            metabase.Pending,
			Encryption:        metabasetest.DefaultEncryption,
			EncryptedUserData: userData,

			ZombieDeletionDeadline: object.ZombieDeletionDeadline,
		}
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return objects[i].ObjectKey < objects[j].ObjectKey
	})
	return objects
}

func createPendingObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName metabase.BucketName, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.ObjectEntry, len(keys))
	for _, key := range keys {
		obj := metabasetest.RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key
		now := time.Now()

		metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

		objects[key] = metabase.ObjectEntry{
			ObjectKey:  obj.ObjectKey,
			Version:    obj.Version,
			StreamID:   obj.StreamID,
			CreatedAt:  now,
			Status:     metabase.Pending,
			Encryption: metabasetest.DefaultEncryption,
		}
	}

	return objects
}

func TestIteratePendingObjectsWithObjectKey(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

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
			object := metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

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

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
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

				objects[i] = metabase.RawObject(metabasetest.CreatePendingObject(ctx, t, db, obj, 0))
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
			object := metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: pending.Location(),
				},
				Result: []metabase.ObjectEntry{objectEntryFromRaw(metabase.RawObject(object))},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
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

				objects[i] = metabase.RawObject(metabasetest.CreatePendingObject(ctx, t, db, obj, 0))
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

				objects[i] = metabase.RawObject(metabasetest.CreatePendingObject(ctx, t, db, obj, 0))
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

		t.Run("committed versioned, unversioned, and delete markers with pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.RandObjectStream()
			location := pending.Location()
			pending.Version = 1000
			pendingObject := metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			a0 := metabasetest.RandObjectStream()
			a0.ProjectID = location.ProjectID
			a0.BucketName = location.BucketName
			a0.ObjectKey = location.ObjectKey
			a0.Version = 2000
			metabasetest.CreateObject(ctx, t, db, a0, 0)

			deletedSuspended, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: location,
				Suspended:      true,
			})
			require.NoError(t, err)

			b0 := metabasetest.RandObjectStream()
			b0.ProjectID = location.ProjectID
			b0.BucketName = location.BucketName
			b0.ObjectKey = location.ObjectKey
			b0.Version = 3000

			obj2 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)

			deletedVersioned, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: location,
				Versioned:      true,
			})
			require.NoError(t, err)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: location,
					BatchSize:      10,
				},
				Result: []metabase.ObjectEntry{objectEntryFromRaw(metabase.RawObject(pendingObject))},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				metabase.RawObject(pendingObject),
				metabase.RawObject(deletedSuspended.Markers[0]),
				metabase.RawObject(obj2),
				metabase.RawObject(deletedVersioned.Markers[0]),
			}}.Check(ctx, t, db)
		})

		t.Run("batch iterate committed versioned, unversioned, and delete markers with pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			var objects []metabase.RawObject
			var expected []metabase.ObjectEntry
			var objLocation metabase.ObjectLocation

			// create 1 pending object first
			pendingStream1 := metabasetest.RandObjectStream()
			objLocation = pendingStream1.Location()
			pendingStream1.Version = 100

			pendingObject1 := metabase.RawObject(metabasetest.CreatePendingObject(ctx, t, db, pendingStream1, 0))
			objects = append(objects, pendingObject1)
			expected = append(expected, objectEntryFromRaw(pendingObject1))

			// create one unversioned committed object and 9 versioned committed objects
			for i := 0; i < 10; i++ {
				unversionedStream := metabasetest.RandObjectStream()

				unversionedStream.ProjectID = objLocation.ProjectID
				unversionedStream.BucketName = objLocation.BucketName
				unversionedStream.ObjectKey = objLocation.ObjectKey
				unversionedStream.Version = metabase.Version(200 + i)
				var comittedtObject metabase.RawObject
				if i%10 == 0 {
					comittedtObject = metabase.RawObject(metabasetest.CreateObject(ctx, t, db, unversionedStream, 0))
				} else {
					comittedtObject = metabase.RawObject(metabasetest.CreateObjectVersioned(ctx, t, db, unversionedStream, 0))
				}
				objects = append(objects, comittedtObject)
			}

			// create a second pending object
			pendingStream2 := metabasetest.RandObjectStream()
			pendingStream2.ProjectID = objLocation.ProjectID
			pendingStream2.BucketName = objLocation.BucketName
			pendingStream2.ObjectKey = objLocation.ObjectKey
			pendingStream2.Version = 300

			pendingObject2 := metabase.RawObject(metabasetest.CreatePendingObject(ctx, t, db, pendingStream2, 0))
			objects = append(objects, pendingObject2)
			expected = append(expected, objectEntryFromRaw(pendingObject2))

			sort.Slice(expected, func(i, j int) bool {
				return expected[i].StreamID.Less(expected[j].StreamID)
			})

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: objLocation,
					BatchSize:      3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: objLocation,
					BatchSize:      1,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: objLocation,
					BatchSize:      1,
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
