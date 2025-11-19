// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
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
					expected[i] = objectEntryFromRawLatest(obj)
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
				expected[i] = objectEntryFromRawLatest(obj)
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
				expected[i] = objectEntryFromRawLatest(obj)
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
				expected[i] = objectEntryFromRawLatest(obj)
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

		t.Run("final prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"\xff\x00",
				"\xffA",
				"\xff\xff",
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
					Prefix:                "\xff",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("\xff",
						objects["\xff\x00"],
						objects["\xffA"],
						objects["\xff\xff"],
					),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestListObjectsSkipCursor(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
						Pending:               false,
						AllVersions:           allVersions,
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
						Pending:               false,
						AllVersions:           allVersions,
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
			}
		})
	})
}

const benchmarkBatchSize = 100

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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
			objC0 := metabasetest.CreatePendingObject(ctx, t, db, c0, 0)

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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
					},
				}}.Check(ctx, t, db)

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
						objectEntryFromRaw(metabase.RawObject(objC0)),
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
						objectEntryFromRawLatest(metabase.RawObject(objA1)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(deletionResult.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
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
						objectEntryFromRawLatest(metabase.RawObject(deletionResultA1.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA1)),
						objectEntryFromRaw(metabase.RawObject(deletionResultA0.Markers[0])),
						objectEntryFromRaw(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(deletionResultB1.Markers[0])),
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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
						objectEntryFromRawLatest(metabase.RawObject(objC1)),
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
						objectEntryFromRawLatest(metabase.RawObject(objA0)),
						objectEntryFromRawLatest(metabase.RawObject(objB1)),
						objectEntryFromRaw(metabase.RawObject(objB0)),
						objectEntryFromRawLatest(metabase.RawObject(objC1)),
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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

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

		t.Run("final prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, metabase.BucketName("bucky")

			objects := metabasetest.CreateVersionedObjectsWithKeys(ctx, t, db, projectID, bucketName, map[metabase.ObjectKey][]metabase.Version{
				"\xff\x00": {1000},
				"\xffA":    {1000},
				"\xff\xff": {1000},
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
					Prefix:                "\xff",
				},
				Result: metabase.ListObjectsResult{
					Objects: withoutPrefix("\xff",
						objects["\xff\x00"],
						objects["\xffA"],
						objects["\xff\xff"],
					),
				},
			}.Check(ctx, t, db)
		})
	})
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
	})
}

func TestListObjects_Requery(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			obj := metabasetest.RandObjectStream()

			objects := []metabase.RawObject{}
			for i := 0; i < 1; i++ {
				for j := 0; j < 3; j++ {
					for k := 0; k < 3; k++ {
						objects = append(objects, metabase.RawObject{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj.ProjectID,
								BucketName: metabase.BucketName("bucket"),
								ObjectKey:  metabase.ObjectKey(fmt.Sprintf("%d/%d/%d/object-%d", i, j, k, i+j+k)),
								Version:    1,
								StreamID:   uuid.UUID{1, byte(i), byte(k), byte(j)},
							},
							Status: metabase.CommittedUnversioned,
						})
					}
				}
			}

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			result, err := db.ListObjects(ctx, metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Recursive:  true,
				Cursor: metabase.ListObjectsCursor{
					Key:     "0/0/0/object-0",
					Version: 1,
				},
				Params: metabase.ListObjectsParams{
					MinBatchSize: 1,
				},
				Limit: len(objects) - 2,
			})
			require.NoError(t, err)
			assert.Len(t, result.Objects, len(objects)-2)
			require.True(t, result.More)
		})

		t.Run("versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			obj := metabasetest.RandObjectStream()

			objects := []metabase.RawObject{}
			for i := 0; i < 11; i++ {
				objects = append(objects, metabase.RawObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: metabase.BucketName("bucket"),
						ObjectKey:  metabase.ObjectKey("0000"),
						Version:    1 + metabase.Version(i),
						StreamID:   uuid.UUID{1},
					},
					Status: metabase.CommittedVersioned,
				})
			}
			for i := 0; i < 105; i++ {
				objects = append(objects, metabase.RawObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: metabase.BucketName("bucket"),
						ObjectKey:  metabase.ObjectKey(fmt.Sprintf("%04d", i+1)),
						Version:    1,
						StreamID:   uuid.UUID{1},
					},
					Status: metabase.CommittedVersioned,
				})
			}

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			result, err := db.ListObjects(ctx, metabase.ListObjects{
				ProjectID:   obj.ProjectID,
				BucketName:  "bucket",
				Recursive:   false,
				Pending:     false,
				AllVersions: false,
				Cursor: metabase.ListObjectsCursor{
					Key:     "0000",
					Version: 1,
				},
				Limit: 100,
			})
			require.NoError(t, err)
			t.Log(len(result.Objects))
			require.True(t, result.More)
		})
	})
}

func TestListObjects_Requery_SkipPrefix(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		const bucketName = "bucket"

		insertObject := func(objects *[]metabase.RawObject, key metabase.ObjectKey) {
			*objects = append(*objects, metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: bucketName,
					ObjectKey:  key,
					Version:    1,
					StreamID:   uuid.UUID{1},
				},
				Status: metabase.CommittedUnversioned,
			})
		}

		t.Run("Basic", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			var objects []metabase.RawObject

			// The maximum number of queries that ListObjects performs is 10 + the list limit.
			// Given a list limit of 3 and a batch size of 5, a maximum of 65 objects will be
			// retrieved from the DB. (The batch size used by ListObjects is the limit + 1 +
			// QueryExtraForNonRecursive (whose minimum value is 1).)
			//
			// Ensure that ListObjects skips prefixes by inserting so many prefixed objects
			// that the last non-prefixed object could not be returned otherwise.

			insertObject(&objects, "a")
			for i := range 64 {
				insertObject(&objects, metabase.ObjectKey("b/"+fmt.Sprintf("%02d", i)))
			}
			insertObject(&objects, "c")

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:   projectID,
					BucketName:  bucketName,
					Recursive:   false,
					Pending:     false,
					AllVersions: false,
					// We use a limit of 3 to ensure that all 3 types of entries are returned:
					// the entry before the prefix ("a"), the prefix ("b/"), and the entry after the prefix ("c").
					Limit: 3,
					Params: metabase.ListObjectsParams{
						MinBatchSize:              4,
						QueryExtraForNonRecursive: 1,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						objectEntryFromRawLatest(objects[0]),
						prefixEntry("b/"),
						objectEntryFromRawLatest(objects[len(objects)-1]),
					},
					More: false,
				},
			}.Check(ctx, t, db)
		})

		t.Run("Unskippable prefix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			var objects []metabase.RawObject

			insertObject(&objects, "a")
			for i := range 64 {
				insertObject(&objects, metabase.ObjectKey("a\xff"+fmt.Sprintf("%02d", i)))
			}
			insertObject(&objects, "b")

			require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

			// Ensure that no odd behavior results from skipping a prefix using the delimiter "\xff".
			// Any delimiter composed of one or more sequences of "\xff" is special because it is
			// impossible to skip. While skipping "a/" advances the cursor to "a0", attempting to skip
			// "a\xff" must cause ListObjects to stop querying.

			firstEntry := objectEntryFromRawLatest(objects[0])
			firstEntry.ObjectKey = ""

			metabasetest.ListObjects{
				Opts: metabase.ListObjects{
					ProjectID:   projectID,
					BucketName:  bucketName,
					Recursive:   false,
					Pending:     false,
					AllVersions: false,
					Limit:       3,
					Prefix:      "a",
					Delimiter:   "\xff",
					Params: metabase.ListObjectsParams{
						MinBatchSize:              4,
						QueryExtraForNonRecursive: 1,
					},
				},
				Result: metabase.ListObjectsResult{
					Objects: []metabase.ObjectEntry{
						firstEntry,
						prefixEntry("\xff"),
					},
					More: false,
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestListObjects_Requery_DeleteMarkers(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		const limit = 1
		const versionsPerObject = 20

		objects := []metabase.RawObject{}
		for i := 0; i < versionsPerObject; i++ {
			for k := 0; k <= versionsPerObject; k++ {
				obj := metabase.RawObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: metabase.BucketName("bucket"),
						ObjectKey:  metabase.ObjectKey(fmt.Sprintf("o%03d", i)),
						Version:    metabase.Version(k + 1),
						StreamID:   uuid.UUID{1, byte(i), byte(k)},
					},
					Status: metabase.CommittedVersioned,
				}
				if k == versionsPerObject {
					obj.Status = metabase.DeleteMarkerVersioned
				}
				objects = append(objects, obj)
			}
		}

		require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

		for _, recursive := range []bool{false, true} {
			for _, pending := range []bool{false, true} {
				for _, allversions := range []bool{false, true} {
					name := fmt.Sprintf("recursive=%v,pending=%v,all=%v", recursive, pending, allversions)
					t.Run(name, func(t *testing.T) {

						opts := metabase.ListObjects{
							ProjectID:   obj.ProjectID,
							BucketName:  "bucket",
							Recursive:   recursive,
							Limit:       limit,
							Pending:     pending,
							AllVersions: allversions,
						}
						opts.Params.MinBatchSize = 1
						opts.Params.QueryExtraForNonRecursive = 1

						result, err := db.ListObjects(ctx, opts)
						require.NoError(t, err)

						t.Log(len(result.Objects))
					})
				}
			}
		}
	})
}

func TestListObjects_Includes(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		data1 := metabasetest.RandEncryptedUserData()
		data2 := metabasetest.RandEncryptedUserData()
		data2.EncryptedETag = nil
		data3 := metabasetest.RandEncryptedUserData()
		data3.EncryptedMetadata = nil

		var objects []metabase.RawObject
		for i, data := range []metabase.EncryptedUserData{data1, data2, data3} {
			objects = append(objects, metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: metabase.BucketName("bucket"),
					ObjectKey:  metabase.ObjectKey(fmt.Sprint(i)),
					Version:    1,
					StreamID:   uuid.UUID{byte(i + 1)},
				},
				EncryptedUserData: data,
				Status:            metabase.CommittedVersioned,
			})
		}
		require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

		obj1 := objectEntryFromRaw(objects[0])
		obj1.IsLatest = true
		obj2 := objectEntryFromRaw(objects[1])
		obj2.IsLatest = true
		obj3 := objectEntryFromRaw(objects[2])
		obj3.IsLatest = true

		withZeroETag := func(entry metabase.ObjectEntry) metabase.ObjectEntry {
			entry.EncryptedETag = nil
			return entry
		}
		withZeroMetadata := func(entry metabase.ObjectEntry) metabase.ObjectEntry {
			entry.EncryptedMetadata = nil
			return entry
		}

		metabasetest.ListObjects{
			Opts: metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Limit:      100,

				IncludeETag:           true,
				IncludeCustomMetadata: true,
			},
			Result: metabase.ListObjectsResult{
				Objects: []metabase.ObjectEntry{
					obj1,
					obj2,
					obj3,
				},
				More: false,
			},
		}.Check(ctx, t, db)

		metabasetest.ListObjects{
			Opts: metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Limit:      100,

				IncludeETag:                 true,
				IncludeCustomMetadata:       true,
				IncludeETagOrCustomMetadata: true,
			},
			Result: metabase.ListObjectsResult{
				Objects: []metabase.ObjectEntry{
					obj1,
					obj2,
					obj3,
				},
				More: false,
			},
		}.Check(ctx, t, db)

		metabasetest.ListObjects{
			Opts: metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Limit:      100,

				IncludeETag: true,
			},
			Result: metabase.ListObjectsResult{
				Objects: []metabase.ObjectEntry{
					withZeroMetadata(obj1),
					withZeroMetadata(obj2),
					withZeroMetadata(obj3),
				},
				More: false,
			},
		}.Check(ctx, t, db)

		metabasetest.ListObjects{
			Opts: metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Limit:      100,

				IncludeCustomMetadata: true,
			},
			Result: metabase.ListObjectsResult{
				Objects: []metabase.ObjectEntry{
					withZeroETag(obj1),
					withZeroETag(obj2),
					withZeroETag(obj3),
				},
				More: false,
			},
		}.Check(ctx, t, db)

		metabasetest.ListObjects{
			Opts: metabase.ListObjects{
				ProjectID:  obj.ProjectID,
				BucketName: "bucket",
				Limit:      100,

				IncludeETagOrCustomMetadata: true,
			},
			Result: metabase.ListObjectsResult{
				Objects: []metabase.ObjectEntry{
					withZeroMetadata(obj1),
					obj2,
					obj3,
				},
				More: false,
			},
		}.Check(ctx, t, db)
	})
}

func TestListObjects_Delimiter(t *testing.T) {
	testListObjectsDelimiter(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, testCase listObjectsDelimiterTestCase) ([]metabase.ObjectEntry, error) {
		result, err := db.ListObjects(ctx, metabase.ListObjects{
			ProjectID:             testCase.projectID,
			BucketName:            testCase.bucketName,
			Prefix:                testCase.prefix,
			Delimiter:             testCase.delimiter,
			IncludeSystemMetadata: true,
		})
		return result.Objects, err
	})
}

func TestSkipPrefix(t *testing.T) {
	for _, tt := range []struct {
		prefix     metabase.ObjectKey
		expected   metabase.ObjectKey
		expectedOk bool
	}{
		{prefix: "", expectedOk: false},
		{prefix: "a", expected: "b", expectedOk: true},
		{prefix: "abc", expected: "abd", expectedOk: true},
		{prefix: "a\xff", expected: "b", expectedOk: true},
		{prefix: "\xff", expectedOk: false},
		{prefix: "\xff\xff", expectedOk: false},
	} {
		output, ok := metabase.SkipPrefix(tt.prefix)
		assert.Equal(t, tt.expected, output, "Prefix: %q", tt.prefix)
		assert.Equal(t, tt.expectedOk, ok, "Prefix: %q", tt.prefix)
	}
}

type listObjectsDelimiterTestCase struct {
	projectID  uuid.UUID
	bucketName metabase.BucketName
	delimiter  metabase.ObjectKey
	prefix     metabase.ObjectKey
}

func testListObjectsDelimiter(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, testCase listObjectsDelimiterTestCase) ([]metabase.ObjectEntry, error)) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		const delimiter = "###"
		const defaultDelimiter = metabase.Delimiter

		projectID := testrand.UUID()
		bucketName := metabase.BucketName(testrand.BucketName())

		requireResult := func(t *testing.T, expected, actual []metabase.ObjectEntry) {
			diff := cmp.Diff(expected, actual, metabasetest.DefaultTimeDiff(),
				// Iterators don't implement IsLatest.
				cmpopts.IgnoreFields(metabase.ObjectEntry{}, "IsLatest"),
			)
			require.Zero(t, diff)
		}

		objects := createObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
			"abc" + delimiter,
			"abc" + delimiter + "def",
			"abc" + delimiter + "def" + delimiter + "ghi",
			"abc" + defaultDelimiter + "def",
			"xyz" + delimiter + "uvw",
		})

		t.Run("Default delimiter", func(t *testing.T) {
			result, err := fn(ctx, t, db, listObjectsDelimiterTestCase{
				projectID:  projectID,
				bucketName: bucketName,
			})
			require.NoError(t, err)

			requireResult(t, []metabase.ObjectEntry{
				objects["abc"+delimiter],
				objects["abc"+delimiter+"def"],
				objects["abc"+delimiter+"def"+delimiter+"ghi"],
				prefixEntry("abc" + defaultDelimiter),
				objects["xyz"+delimiter+"uvw"],
			}, result)
		})

		t.Run("Root", func(t *testing.T) {
			result, err := fn(ctx, t, db, listObjectsDelimiterTestCase{
				projectID:  projectID,
				bucketName: bucketName,
				delimiter:  delimiter,
			})
			require.NoError(t, err)

			requireResult(t, []metabase.ObjectEntry{
				prefixEntry("abc" + delimiter),
				objects["abc"+defaultDelimiter+"def"],
				prefixEntry("xyz" + delimiter),
			}, result)
		})

		t.Run("1 level deep", func(t *testing.T) {
			result, err := fn(ctx, t, db, listObjectsDelimiterTestCase{
				projectID:  projectID,
				bucketName: bucketName,
				delimiter:  delimiter,
				prefix:     "abc" + delimiter,
			})
			require.NoError(t, err)

			requireResult(t, []metabase.ObjectEntry{
				withoutPrefix1("abc"+delimiter, objects["abc"+delimiter]),
				withoutPrefix1("abc"+delimiter, objects["abc"+delimiter+"def"]),
				prefixEntry("def" + delimiter),
			}, result)
		})

		t.Run("2 levels deep", func(t *testing.T) {
			result, err := fn(ctx, t, db, listObjectsDelimiterTestCase{
				projectID:  projectID,
				bucketName: bucketName,
				delimiter:  delimiter,
				prefix:     "abc" + delimiter + "def" + delimiter,
			})
			require.NoError(t, err)

			requireResult(t, []metabase.ObjectEntry{
				withoutPrefix1(
					"abc"+delimiter+"def"+delimiter,
					objects["abc"+delimiter+"def"+delimiter+"ghi"],
				),
			}, result)
		})

		t.Run("Prefix suffixed with partial delimiter", func(t *testing.T) {
			partialDelimiter := metabase.ObjectKey(delimiter[:len(delimiter)-1])
			remainingDelimiter := metabase.ObjectKey(delimiter[len(delimiter)-1:])

			result, err := fn(ctx, t, db, listObjectsDelimiterTestCase{
				projectID:  projectID,
				bucketName: bucketName,
				delimiter:  delimiter,
				prefix:     "abc" + partialDelimiter,
			})
			require.NoError(t, err)

			requireResult(t, []metabase.ObjectEntry{
				withoutPrefix1("abc"+partialDelimiter, objects["abc"+delimiter]),
				withoutPrefix1("abc"+partialDelimiter, objects["abc"+delimiter+"def"]),
				prefixEntry(remainingDelimiter + "def" + delimiter),
			}, result)
		})
	})
}
