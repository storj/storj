// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// TestListObjects_ManyVersionsPerKey covers listing over a key whose version count
// exceeds the query batch size, which requires iterating inside a single object_key
// group across batches. This exercises the per-key fallback of the TiDB local-reorder
// mode and the version-skip machinery of the other implementations.
func TestListObjects_ManyVersionsPerKey(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		const versionsOfHotKey = 300 // larger than the default MinBatchSize of 100

		var entries []metabase.ObjectEntry
		addObject := func(key metabase.ObjectKey, version metabase.Version) {
			entries = append(entries, metabase.ObjectEntry{
				ObjectKey: key,
				Version:   version,
				StreamID:  uuid.UUID{1, byte(version >> 8), byte(version)},
				Status:    metabase.CommittedVersioned,
			})
		}
		addObject("a-before", 1)
		addObject("b-hot", 1)
		for v := 2; v <= versionsOfHotKey; v++ {
			addObject("b-hot", metabase.Version(v))
		}
		addObject("c-after", 1)
		addObject("c-after", 2)

		require.NoError(t, db.TestingBatchInsertObjects(ctx, objectEntriesToRawObjects(entries)))
		naive := NewNaiveObjectsDB(entries)

		check := func(opts metabase.ListObjects) metabase.ListObjectsResult {
			opts.ProjectID = uuid.UUID{1}
			opts.BucketName = "b"
			opts.Recursive = true

			expResult, expErr := naive.ListObjects(ctx, opts)
			gotResult, gotErr := db.ListObjects(ctx, opts)
			require.Equal(t, expErr, gotErr, fmt.Sprintf("%#v", opts))
			require.Equal(t, expResult, gotResult, fmt.Sprintf("%#v", opts))
			return gotResult
		}

		for _, allVersions := range []bool{true, false} {
			for _, limit := range []int{1, 10, 1000} {
				opts := metabase.ListObjects{
					AllVersions: allVersions,
					Limit:       limit,
				}
				check(opts)

				// resume from inside the hot key's version range
				for _, version := range []metabase.Version{1, 2, 150, 299, 300} {
					opts.Cursor = metabase.ListObjectsCursor{Key: "b-hot", Version: version}
					check(opts)
				}

				// full drain via cursors
				opts.Cursor = metabase.ListObjectsCursor{}
				for {
					result := check(opts)
					if !result.More || len(result.Objects) == 0 {
						break
					}
					last := result.Objects[len(result.Objects)-1]
					opts.Cursor = metabase.ListObjectsCursor{Key: last.ObjectKey, Version: last.Version}
				}
			}
		}
	})
}

// TestListObjects_CursorAtVersionHeavyKey covers resuming a listing at a key that has
// more versions than the requery budget (batchSize × requeryLimit) can rescan. Pending
// and descending all-versions listings must restart the scan exactly at the client
// cursor instead of rescanning the cursor key's versions, otherwise resuming at (or
// paging past) a version-heavy key fails with "too many requeries".
func TestListObjects_CursorAtVersionHeavyKey(t *testing.T) {
	metabasetest.Run(t, testListObjectsCursorAtVersionHeavyKey)
}

// TestListObjects_CursorAtVersionHeavyKey_LocalReorder runs the same scenarios with the
// TiDB local-reorder list mode, which the default test configuration does not cover.
// The mode has no effect on other adapters.
func TestListObjects_CursorAtVersionHeavyKey_LocalReorder(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName: "metabase-tests",
		DefaultListMode: metabase.ListModeLocalReorder,
	}, testListObjectsCursorAtVersionHeavyKey)
}

func testListObjectsCursorAtVersionHeavyKey(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	// With Limit=5, MinBatchSize=1 and QueryExtraForNonRecursive=1 a page has a requery
	// budget of 15 queries with a batch size of at most 10 rows, so it can rescan at
	// most ~150 rows of the cursor key; 200 versions exceed that.
	const versionsOfHotKey = 200

	var entries []metabase.ObjectEntry
	addObject := func(key metabase.ObjectKey, version metabase.Version, status metabase.ObjectStatus) {
		entries = append(entries, metabase.ObjectEntry{
			ObjectKey: key,
			Version:   version,
			StreamID:  uuid.UUID{1, byte(version >> 8), byte(version)},
			Status:    status,
		})
	}

	for v := metabase.Version(1); v <= versionsOfHotKey; v++ {
		addObject("d/hot", v, metabase.CommittedVersioned)
		addObject("p-hot", v, metabase.Pending)
		addObject("v-hot", v, metabase.CommittedVersioned)
	}
	addObject("d/z-after", 1, metabase.CommittedVersioned)
	addObject("q-after", 1, metabase.Pending)
	addObject("w-after", 1, metabase.CommittedVersioned)

	require.NoError(t, db.TestingBatchInsertObjects(ctx, objectEntriesToRawObjects(entries)))
	naive := NewNaiveObjectsDB(entries)

	for _, recursive := range []bool{true, false} {
		check := func(opts metabase.ListObjects) {
			opts.ProjectID = uuid.UUID{1}
			opts.BucketName = "b"
			opts.Recursive = recursive
			opts.AllVersions = true
			opts.Limit = 5
			opts.Params.MinBatchSize = 1
			opts.Params.QueryExtraForNonRecursive = 1

			expResult, expErr := naive.ListObjects(ctx, opts)
			gotResult, gotErr := db.ListObjects(ctx, opts)
			require.Equal(t, expErr, gotErr, fmt.Sprintf("recursive=%v %#v", recursive, opts))
			require.Equal(t, expResult, gotResult, fmt.Sprintf("recursive=%v %#v", recursive, opts))
		}

		// An ascending pending listing resuming past the hot key: the metainfo endpoint
		// maps a key-only cursor to version MaxVersion.
		check(metabase.ListObjects{
			Pending: true,
			Cursor:  metabase.ListObjectsCursor{Key: "p-hot", Version: metabase.MaxVersion},
		})

		// A descending all-versions listing resuming below all of the hot key's versions.
		check(metabase.ListObjects{
			Cursor: metabase.ListObjectsCursor{Key: "v-hot", Version: 1},
		})

		// Same, inside a prefix.
		check(metabase.ListObjects{
			Prefix: "d/",
			Cursor: metabase.ListObjectsCursor{Key: "d/hot", Version: 1},
		})

		// IsLatest of the hot key must survive resuming from mid-stack, top-of-stack,
		// above the key's actual maximum version, and MaxVersion cursors.
		for _, version := range []metabase.Version{2, 100, versionsOfHotKey, versionsOfHotKey + 1, metabase.MaxVersion} {
			check(metabase.ListObjects{
				Cursor: metabase.ListObjectsCursor{Key: "v-hot", Version: version},
			})
			check(metabase.ListObjects{
				Prefix: "d/",
				Cursor: metabase.ListObjectsCursor{Key: "d/hot", Version: version},
			})
			check(metabase.ListObjects{
				Pending: true,
				Cursor:  metabase.ListObjectsCursor{Key: "p-hot", Version: version},
			})
		}
	}
}
