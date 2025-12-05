// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"flag"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

var listObjectsExhaustive = flag.Bool("exhaustive", false, "exhaustively test list objects implementation")

func TestListObjects_Exhaustive(t *testing.T) {
	if !*listObjectsExhaustive {
		t.Skip(`Use "go test -run TestListObjects_Exhaustive -exhaustive" to run this test.`)
	}

	entries := generateExhaustiveTestData()
	raw := objectEntriesToRawObjects(entries)
	naive := NewNaiveObjectsDB(entries)

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		require.NoError(t, db.TestingBatchInsertObjects(ctx, raw))

		check := func(opts metabase.ListObjects) {
			expResult, expErr := naive.ListObjects(ctx, opts)
			gotResult, gotErr := db.ListObjects(ctx, opts)

			require.Equal(t, expErr, gotErr, fmt.Sprintf("%#v", opts))
			require.Equal(t, expResult, gotResult, fmt.Sprintf("%#v", opts))
		}

		var opts metabase.ListObjects
		opts.ProjectID = uuid.UUID{1}
		opts.BucketName = "b"
		for _, opts.Delimiter = range []metabase.ObjectKey{"/", "AA", "\xff", "\xff\xff"} {
			for _, opts.Prefix = range []metabase.ObjectKey{"", "A", "B", "AA/", "BB/"} {
				for _, opts.Pending = range []bool{true, false} {
					for _, opts.AllVersions = range []bool{true, false} {
						for _, opts.Recursive = range []bool{true, false} {
							for _, opts.Limit = range []int{1, 3, 7} {
								opts.Cursor.Key = ""
								opts.Cursor.Version = 0
								check(opts)

								opts.Cursor.Version = metabase.MaxVersion
								check(opts)

								opts.Cursor.Version = 4
								check(opts)

								opts.Cursor.Version = 5
								check(opts)

								for i := range entries {
									entry := &entries[i]
									opts.Cursor.Key = entry.ObjectKey

									opts.Cursor.Version = 0
									check(opts)

									opts.Cursor.Version = entry.Version
									check(opts)

									opts.Cursor.Version = metabase.MaxVersion
									check(opts)

									opts.Cursor.Version = entry.Version - 1
									check(opts)

									opts.Cursor.Version = entry.Version + 1
									check(opts)
								}
							}
						}
					}
				}
			}
		}
	})
}

func objectEntriesToRawObjects(entries []metabase.ObjectEntry) (rs []metabase.RawObject) {
	rs = make([]metabase.RawObject, len(entries))
	for i := range rs {
		entry := &entries[i]
		rs[i] = metabase.RawObject{
			ObjectStream: metabase.ObjectStream{
				ProjectID:  uuid.UUID{1},
				BucketName: "b",
				ObjectKey:  entry.ObjectKey,
				Version:    entry.Version,
				StreamID:   entry.StreamID,
			},
			Status: entry.Status,
		}
	}
	return rs
}

func generateExhaustiveTestData() []metabase.ObjectEntry {
	cornerBytes := []byte{0, 1, 'A', 'B', 254, 255}
	streamID := uuid.UUID{1}
	entries := []metabase.ObjectEntry{}
	for _, a := range cornerBytes {
		entries = append(entries,
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a}),
				Version:   1,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a}),
				Version:   2,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a}),
				Version:   3,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a, a, 0x00}),
				Version:   1,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a, a, 0xFF}),
				Version:   1,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
			metabase.ObjectEntry{
				ObjectKey: metabase.ObjectKey([]byte{a, a, '/'}),
				Version:   1,
				StreamID:  streamID,
				Status:    metabase.CommittedVersioned,
			},
		)
		if a == 'B' {
			entries = append(entries,
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a}),
					Version:   4,
					StreamID:  streamID,
					Status:    metabase.DeleteMarkerVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, 0x00}),
					Version:   2,
					StreamID:  streamID,
					Status:    metabase.DeleteMarkerVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, 0xFF}),
					Version:   2,
					StreamID:  streamID,
					Status:    metabase.DeleteMarkerVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/'}),
					Version:   2,
					StreamID:  streamID,
					Status:    metabase.DeleteMarkerVersioned,
				},
			)
		}

		for _, b := range cornerBytes {
			entries = append(entries,
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/', b}),
					Version:   1,
					StreamID:  streamID,
					Status:    metabase.CommittedVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/', b}),
					Version:   2,
					StreamID:  streamID,
					Status:    metabase.CommittedVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/', b}),
					Version:   3,
					StreamID:  streamID,
					Status:    metabase.CommittedVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/', b, 0x00}),
					Version:   1,
					StreamID:  streamID,
					Status:    metabase.CommittedVersioned,
				},
				metabase.ObjectEntry{
					ObjectKey: metabase.ObjectKey([]byte{a, a, '/', b, 0xFF}),
					Version:   1,
					StreamID:  streamID,
					Status:    metabase.CommittedVersioned,
				},
			)
		}
	}

	return entries
}
