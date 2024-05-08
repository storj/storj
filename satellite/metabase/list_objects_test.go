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
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

type listObjectsScenario struct {
	Name        string
	Pending     bool
	AllVersions bool
}

var listObjectsScenarios = []listObjectsScenario{
	{Name: "", Pending: false, AllVersions: false},
	{Name: ",pending", Pending: true, AllVersions: false},
	{Name: ",all", Pending: false, AllVersions: true},
	{Name: ",pending,all", Pending: true, AllVersions: true},
}

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

			for _, scenario := range listObjectsScenarios {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Pending:     scenario.Pending,
						AllVersions: scenario.AllVersions,
					},
					Result: metabase.ListObjectsResult{},
				}.Check(ctx, t, db)
			}

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
						Status:    metabase.Prefix,
					}
				} else {
					expected[i] = objectEntryFromRaw(obj)
				}
			}

			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             uuid.UUID{1},
						BucketName:            "mybucket",
						Recursive:             false,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
						Limit:                 limit,
					},
					Result: metabase.ListObjectsResult{
						Objects: expected,
						More:    false,
					}}.Check(ctx, t, db)
			}
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

			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             uuid.UUID{1},
						BucketName:            "mybucket",
						Recursive:             true,
						Limit:                 limit,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: expected,
						More:    true,
					}}.Check(ctx, t, db)
			}

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

			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             uuid.UUID{1},
						BucketName:            "bucket-a",
						Recursive:             true,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: expected,
					}}.Check(ctx, t, db)
			}

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

			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             uuid.UUID{1},
						BucketName:            "mybucket",
						Recursive:             true,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: expected,
					}}.Check(ctx, t, db)
			}

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
			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             projectID,
						BucketName:            bucketName,
						Recursive:             true,
						Pending:               false,
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version - 1},
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
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "b/",
						Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
					},
					Result: metabase.ListObjectsResult{
						Objects: withoutPrefix("b/",
							objects["b/3"],
						),
					}}.Check(ctx, t, db)

				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             projectID,
						BucketName:            bucketName,
						Recursive:             true,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "b/",
						Cursor: metabase.ListObjectsCursor{Key: "c/"},
					},
					Result: metabase.ListObjectsResult{},
				}.Check(ctx, t, db)
			}
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
			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             projectID,
						BucketName:            bucketName,
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version - 1},
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
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "b/",
						Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
					},
					Result: metabase.ListObjectsResult{
						Objects: withoutPrefix("b/",
							objects["b/3"],
						)},
				}.Check(ctx, t, db)

				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             projectID,
						BucketName:            bucketName,
						Pending:               false,
						AllVersions:           allVersions,
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
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "c/",
						Cursor: metabase.ListObjectsCursor{Key: "c/", Version: 0},
					},
					Result: metabase.ListObjectsResult{
						Objects: withoutPrefix("c/",
							prefixEntry("c//"),
							objects["c/1"],
						)},
				}.Check(ctx, t, db)

				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:             projectID,
						BucketName:            bucketName,
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "c/",
						Cursor: metabase.ListObjectsCursor{Key: "c/", Version: metabase.MaxVersion},
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,

						Prefix: "c//",
					},
					Result: metabase.ListObjectsResult{
						Objects: withoutPrefix("c//",
							objects["c//"],
						)},
				}.Check(ctx, t, db)
			}
		})

	}, metabasetest.WithSpanner())
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

			for _, allVersions := range []bool{false, true} {
				metabasetest.ListObjects{
					Opts: metabase.ListObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Recursive:  false,
						Prefix:     "",
						Cursor: metabase.ListObjectsCursor{
							Key:     metabase.ObjectKey("08/"),
							Version: -100,
						},
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: []metabase.ObjectEntry{
							prefixEntry(metabase.ObjectKey("09/")),
							prefixEntry(metabase.ObjectKey("10/")),
						}},
				}.Check(ctx, t, db)
			}
		})

		t.Run("prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"2017/05/08/test",
				"2017/05/09/test",
				"2017/05/10/test",
			})
			for _, allVersions := range []bool{false, true} {
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
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: []metabase.ObjectEntry{
							prefixEntry(metabase.ObjectKey("09/")),
							prefixEntry(metabase.ObjectKey("10/")),
						}},
				}.Check(ctx, t, db)
			}
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

			for _, allVersions := range []bool{false, true} {
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
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: []metabase.ObjectEntry{
							prefixEntry(metabase.ObjectKey("08/")),
							withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: []metabase.ObjectEntry{
							withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
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
						Pending:               false,
						AllVersions:           allVersions,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					},
					Result: metabase.ListObjectsResult{
						Objects: []metabase.ObjectEntry{
							withoutPrefix1("2017/05/", objects["2017/05/08"+afterDelimiter]),
							prefixEntry(metabase.ObjectKey("09/")),
							prefixEntry(metabase.ObjectKey("10/")),
						}},
				}.Check(ctx, t, db)
			}
		})
	}, metabasetest.WithSpanner())
}

const benchmarkBatchSize = 100

func BenchmarkNonRecursiveObjectsListingOld(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		obj, objects := generateBenchmarkData()
		require.NoError(b, db.TestingBatchInsertObjects(ctx, objects))

		b.Run("listing no prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result, err := db.ListObjectsWithIterator(ctx, metabase.ListObjects{
					ProjectID:   obj.ProjectID,
					BucketName:  obj.BucketName,
					Pending:     false,
					AllVersions: false,
					Limit:       benchmarkBatchSize,
				})
				require.NoError(b, err)
				for result.More {
					result, err = db.ListObjectsWithIterator(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Cursor:      metabase.ListObjectsCursor{Key: result.Objects[len(result.Objects)-1].ObjectKey},
						Pending:     false,
						AllVersions: false,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)
				}
			}
		})

		b.Run("listing with prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result, err := db.ListObjectsWithIterator(ctx, metabase.ListObjects{
					ProjectID:   obj.ProjectID,
					BucketName:  obj.BucketName,
					Prefix:      "foo/",
					Pending:     false,
					AllVersions: false,
					Limit:       benchmarkBatchSize,
				})
				require.NoError(b, err)
				for result.More {
					cursorKey := "foo/" + result.Objects[len(result.Objects)-1].ObjectKey
					result, err = db.ListObjectsWithIterator(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Prefix:      "foo/",
						Cursor:      metabase.ListObjectsCursor{Key: cursorKey},
						Pending:     false,
						AllVersions: false,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)
				}
			}
		})

		b.Run("listing only prefix", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result, err := db.ListObjectsWithIterator(ctx, metabase.ListObjects{
					ProjectID:   obj.ProjectID,
					BucketName:  obj.BucketName,
					Prefix:      "boo/",
					Pending:     false,
					AllVersions: false,
					Limit:       benchmarkBatchSize,
				})
				require.NoError(b, err)
				for result.More {
					cursorKey := "boo/" + result.Objects[len(result.Objects)-1].ObjectKey
					result, err = db.ListObjectsWithIterator(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Prefix:      "boo/",
						Cursor:      metabase.ListObjectsCursor{Key: cursorKey},
						Pending:     false,
						AllVersions: false,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)

				}
			}
		})
	})
}

func BenchmarkNonRecursiveObjectsListing(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		obj, objects := generateBenchmarkData()
		require.NoError(b, db.TestingBatchInsertObjects(ctx, objects))

		for _, scenario := range listObjectsScenarios {
			b.Run("no prefix"+scenario.Name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					result, err := db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Pending:     scenario.Pending,
						AllVersions: scenario.AllVersions,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)
					for result.More {
						result, err = db.ListObjects(ctx, metabase.ListObjects{
							ProjectID:   obj.ProjectID,
							BucketName:  obj.BucketName,
							Cursor:      metabase.ListObjectsCursor{Key: result.Objects[len(result.Objects)-1].ObjectKey},
							Pending:     scenario.Pending,
							AllVersions: scenario.AllVersions,
							Limit:       benchmarkBatchSize,
						})
						require.NoError(b, err)
					}
				}
			})

			b.Run("with prefix"+scenario.Name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					result, err := db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Prefix:      "foo/",
						Pending:     scenario.Pending,
						AllVersions: scenario.AllVersions,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)
					for result.More {
						cursorKey := "foo/" + result.Objects[len(result.Objects)-1].ObjectKey
						result, err = db.ListObjects(ctx, metabase.ListObjects{
							ProjectID:   obj.ProjectID,
							BucketName:  obj.BucketName,
							Prefix:      "foo/",
							Cursor:      metabase.ListObjectsCursor{Key: cursorKey},
							Pending:     scenario.Pending,
							AllVersions: scenario.AllVersions,
							Limit:       benchmarkBatchSize,
						})
						require.NoError(b, err)
					}
				}
			})

			b.Run("only prefix"+scenario.Name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					result, err := db.ListObjects(ctx, metabase.ListObjects{
						ProjectID:   obj.ProjectID,
						BucketName:  obj.BucketName,
						Prefix:      "boo/",
						Pending:     scenario.Pending,
						AllVersions: scenario.AllVersions,
						Limit:       benchmarkBatchSize,
					})
					require.NoError(b, err)
					for result.More {
						cursorKey := "boo/" + result.Objects[len(result.Objects)-1].ObjectKey
						result, err = db.ListObjects(ctx, metabase.ListObjects{
							ProjectID:   obj.ProjectID,
							BucketName:  obj.BucketName,
							Prefix:      "boo/",
							Cursor:      metabase.ListObjectsCursor{Key: cursorKey},
							Pending:     scenario.Pending,
							AllVersions: scenario.AllVersions,
							Limit:       benchmarkBatchSize,
						})
						require.NoError(b, err)

					}
				}
			})
		}
	})
}

func generateBenchmarkData() (obj metabase.ObjectStream, objects []metabase.RawObject) {
	obj = metabase.ObjectStream{
		ProjectID:  uuid.UUID{1, 1, 1, 1},
		BucketName: "bucket",
	}

	for i := 0; i < 500; i++ {
		objects = append(objects, metabase.RawObject{
			ObjectStream: metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  metabase.ObjectKey(strconv.Itoa(i)),
				Version:    100,
				StreamID:   uuid.UUID{1},
			},
			CreatedAt: time.Now(),
			Status:    metabase.CommittedVersioned,
		})
	}

	for i := 0; i < 10; i++ {
		objects = append(objects,
			metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  metabase.ObjectKey("foo/" + strconv.Itoa(i)),
					Version:    100,
					StreamID:   uuid.UUID{1},
				},
				CreatedAt: time.Now(),
				Status:    metabase.CommittedVersioned,
			},
			metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  metabase.ObjectKey("foo/prefixA/" + strconv.Itoa(i)),
					Version:    100,
					StreamID:   uuid.UUID{1},
				},
				CreatedAt: time.Now(),
				Status:    metabase.CommittedVersioned,
			},
			metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  metabase.ObjectKey("foo/prefixB/" + strconv.Itoa(i)),
					Version:    100,
					StreamID:   uuid.UUID{1},
				},
				CreatedAt: time.Now(),
				Status:    metabase.CommittedVersioned,
			},
		)
	}

	for i := 0; i < 50; i++ {
		objects = append(objects, metabase.RawObject{
			ObjectStream: metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  metabase.ObjectKey("boo/foo" + strconv.Itoa(i) + "/object"),
				Version:    100,
				StreamID:   uuid.UUID{1},
			},
			CreatedAt: time.Now(),
			Status:    metabase.CommittedVersioned,
		})
	}

	return obj, objects
}

func TestListObjectsVersioned(t *testing.T) {
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

			objA0 := metabasetest.CreateObject(ctx, t, db, a0, 0)
			objB0 := metabasetest.CreateObjectVersioned(ctx, t, db, b0, 0)
			objB1 := metabasetest.CreateObjectVersionedOutOfOrder(ctx, t, db, b1, 0, 1001)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
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
			metabasetest.CreatePendingObject(ctx, t, db, c0, 0)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

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
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
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

			pendingObject := metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  c0.ProjectID,
					BucketName: c0.BucketName,
					ObjectKey:  c0.ObjectKey,
					Version:    1000,
					StreamID:   c0.StreamID,
				},
				CreatedAt: now,
				Status:    metabase.Pending,

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
			}

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
						objectEntryFromRaw(pendingObject),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(pendingObject),
					},
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objB0),
					metabase.RawObject(objB1),
					pendingObject,
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

			metabasetest.BeginObjectExactVersion{
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
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

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
							ObjectKey: b0.ObjectKey,
							Version:   1000,
							StreamID:  b0.StreamID,
							CreatedAt: now,
							Status:    metabase.Pending,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               true,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						{
							ObjectKey: b0.ObjectKey,
							Version:   1000,
							StreamID:  b0.StreamID,
							CreatedAt: now,
							Status:    metabase.Pending,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objA0),
					metabase.RawObject(objA1),
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  b0.ProjectID,
							BucketName: b0.BucketName,
							ObjectKey:  b0.ObjectKey,
							Version:    1000,
							StreamID:   b0.StreamID,
						},
						CreatedAt: now,
						Status:    metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objA0)),
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objB1)),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(deletionResult.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(deletionResultA1.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(deletionResultA0.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(deletionResultB1.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(deletionResultB0.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objB0)),
					},
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

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objC1)),
					},
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             a0.ProjectID,
					BucketName:            a0.BucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRaw(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
						objectEntryFromRaw(metabase.RawObject(objC1)),
						objectEntryFromRaw(metabase.RawObject(deletionResultC0.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objC0)),
					},
				}}.Check(ctx, t, db)

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
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := metabasetest.CreateVersionedObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
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
					Pending:               false,
					AllVersions:           false,
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
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version},
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
					AllVersions:           false,
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
					AllVersions:           false,
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
					AllVersions:           false,
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
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/3"],
					),
				}}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Recursive:             true,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: metabase.MaxVersion},
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
					AllVersions:           false,
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
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := metabasetest.CreateVersionedObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
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
					Pending:               false,
					AllVersions:           false,
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
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{Key: "a", Version: objects["a"].Version},
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
					Pending:               false,
					AllVersions:           false,
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
					Pending:               false,
					AllVersions:           false,
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
					AllVersions:           false,
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
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: -3},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("b/",
						objects["b/3"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "b/",
					Cursor: metabase.ListObjectsCursor{Key: "b/2", Version: metabase.MaxVersion},
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
					AllVersions:           false,
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
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.ListObjectsCursor{Key: "c/", Version: 0},
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("c/",
						prefixEntry("c//"),
						objects["c/1"],
					)},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Prefix: "c/",
					Cursor: metabase.ListObjectsCursor{Key: "c/", Version: metabase.MaxVersion},
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
					Pending:               false,
					AllVersions:           false,
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

		t.Run("ignore non-latest objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := metabasetest.CreateVersionedObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"a":   {1000, 1001},
				"b/1": {1000, 1001},
				"b/2": {1000, 1001},
				"b/3": {1000, 1001},
				"c":   {1000, 1001},
			})

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "a",
						Version: 1002,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a"],
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             true,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "a",
						Version: 1001,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["b/1"],
						objects["b/2"],
						objects["b/3"],
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "a",
						Version: 1002,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["a"],
						prefixEntry("b/"),
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "a",
						Version: 1001,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						prefixEntry("b/"),
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "b/3",
						Version: 1002,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						// prefixEntry("b/"), // TODO: not sure whether this is the right behaviour
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:             projectID,
					BucketName:            bucketName,
					Pending:               false,
					AllVersions:           false,
					Recursive:             false,
					IncludeCustomMetadata: true,
					IncludeSystemMetadata: true,

					Cursor: metabase.ListObjectsCursor{
						Key:     "b/3",
						Version: 1001,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objects["c"],
					},
				},
			}.Check(ctx, t, db)

		})
	}, metabasetest.WithSpanner())
}

func TestListObjects_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("this is slow")
	}

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		prefixes := []string{"a", "b", "c", "d", "e", "f"}
		const objectsPerPrefix = 10
		const versionsPerObject = 10

		objects := make([]metabase.RawObject, 0, len(prefixes)*objectsPerPrefix*versionsPerObject)

		for i, prefix := range prefixes {
			isVersioned := i >= 3

			for k := 0; k < objectsPerPrefix; k++ {
				objectkey := metabase.ObjectKey(prefix + "/" + strconv.Itoa(k))
				if isVersioned {
					for v := 0; v < versionsPerObject; v++ {
						objects = append(objects, metabase.RawObject{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj.ProjectID,
								BucketName: obj.BucketName,
								ObjectKey:  objectkey,
								Version:    100 + metabase.Version(v),
								StreamID:   testrand.UUID(),
							},
							CreatedAt: time.Now(),
							Status:    metabase.CommittedVersioned,
						})
					}
				} else {
					objects = append(objects, metabase.RawObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  objectkey,
							Version:    100,
							StreamID:   testrand.UUID(),
						},
						CreatedAt: time.Now(),
						Status:    metabase.CommittedUnversioned,
					})
				}
			}
		}

		err := db.TestingBatchInsertObjects(ctx, objects)
		require.NoError(t, err)

		t.Log("recursive=false all-versions=false")
		items, err := db.ListObjects(ctx, metabase.ListObjects{
			ProjectID:   obj.ProjectID,
			BucketName:  obj.BucketName,
			Recursive:   false,
			Pending:     false,
			AllVersions: false,
		})
		require.NoError(t, err)
		require.Len(t, items.Objects, 6)

		t.Log("recursive=false all-versions=true")
		_, err = db.ListObjects(ctx, metabase.ListObjects{
			ProjectID:   obj.ProjectID,
			BucketName:  obj.BucketName,
			Recursive:   false,
			Pending:     true,
			AllVersions: true,
			Limit:       4,
		})
		require.NoError(t, err)

		t.Log("recursive=true")
		_, err = db.ListObjects(ctx, metabase.ListObjects{
			ProjectID:   obj.ProjectID,
			BucketName:  obj.BucketName,
			Recursive:   true,
			Pending:     false,
			AllVersions: false,
			Limit:       4,
		})
		require.NoError(t, err)
	}, metabasetest.WithSpanner())
}
