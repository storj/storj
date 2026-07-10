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
