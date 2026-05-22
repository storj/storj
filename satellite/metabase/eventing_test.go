// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestReadBucketEventBatch(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("test requires TiDB")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter)

		insert := func(t *testing.T, key string) {
			t.Helper()
			require.NoError(t, adapter.TestingInsertBucketEvent(ctx, metabase.BucketEvent{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: "bucket",
					ObjectKey:  metabase.ObjectKey(key),
					Version:    1,
					StreamID:   testrand.UUID(),
				},
				TotalPlainSize: 100,
				EventName:      "ObjectCreated:Put",
			}))
		}

		t.Run("empty outbox returns no rows", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rows, err := adapter.ReadBucketEventBatch(ctx, 0, 10)
			require.NoError(t, err)
			require.Empty(t, rows)
		})

		t.Run("returns rows after afterID in order", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insert(t, "key1")
			insert(t, "key2")
			insert(t, "key3")

			all, err := adapter.ReadBucketEventBatch(ctx, 0, 10)
			require.NoError(t, err)
			require.Len(t, all, 3)

			// IDs must be strictly increasing.
			require.Less(t, all[0].ID, all[1].ID)
			require.Less(t, all[1].ID, all[2].ID)

			// afterID is exclusive: skip the first row.
			after, err := adapter.ReadBucketEventBatch(ctx, all[0].ID, 10)
			require.NoError(t, err)
			require.Len(t, after, 2)
			require.Equal(t, all[1].ID, after[0].ID)
			require.Equal(t, all[2].ID, after[1].ID)
		})

		t.Run("respects limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insert(t, "key1")
			insert(t, "key2")
			insert(t, "key3")

			rows, err := adapter.ReadBucketEventBatch(ctx, 0, 2)
			require.NoError(t, err)
			require.Len(t, rows, 2)
		})

		t.Run("decodes fields correctly", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectID := testrand.UUID()
			streamID := testrand.UUID()
			require.NoError(t, adapter.TestingInsertBucketEvent(ctx, metabase.BucketEvent{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: "my-bucket",
					ObjectKey:  "my/key",
					Version:    42,
					StreamID:   streamID,
				},
				TotalPlainSize: 1234,
				EventName:      "ObjectCreated:Put",
			}))

			rows, err := adapter.ReadBucketEventBatch(ctx, 0, 10)
			require.NoError(t, err)
			require.Len(t, rows, 1)

			r := rows[0]
			require.Equal(t, projectID, r.ProjectID)
			require.Equal(t, metabase.BucketName("my-bucket"), r.BucketName)
			require.Equal(t, metabase.ObjectKey("my/key"), r.ObjectKey)
			require.Equal(t, metabase.Version(42), r.Version)
			require.Equal(t, streamID, r.StreamID)
			require.Equal(t, int64(1234), r.TotalPlainSize)
			require.Equal(t, "ObjectCreated:Put", r.EventName)
			require.False(t, r.CreatedAt.IsZero())
		})
	})
}

func TestDeleteBucketEvents(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("test requires TiDB")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter)

		insert := func(t *testing.T) (id int64, event metabase.BucketEvent) {
			t.Helper()
			event = metabase.BucketEvent{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: "bucket",
					ObjectKey:  metabase.ObjectKey(testrand.UUID().String()),
					Version:    1,
					StreamID:   testrand.UUID(),
				},
				TotalPlainSize: 100,
				EventName:      "ObjectCreated:Put",
			}
			require.NoError(t, adapter.TestingInsertBucketEvent(ctx, event))
			rows, err := adapter.ReadBucketEventBatch(ctx, 0, 1000)
			require.NoError(t, err)
			return rows[len(rows)-1].ID, event
		}

		t.Run("no-op on empty slice", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insert(t)
			require.NoError(t, adapter.DeleteBucketEvents(ctx, nil))

			count, err := adapter.TestingCountBucketEvents(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, count)
		})

		t.Run("deletes only the specified IDs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			id1, _ := insert(t)
			_, event2 := insert(t)
			id3, _ := insert(t)

			require.NoError(t, adapter.DeleteBucketEvents(ctx, []int64{id1, id3}))

			remaining, err := adapter.TestingGetAllBucketEvents(ctx)
			require.NoError(t, err)
			require.Len(t, remaining, 1)
			require.Equal(t, event2.ObjectKey, remaining[0].ObjectKey)
		})

		t.Run("ignores non-existent IDs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insert(t)
			require.NoError(t, adapter.DeleteBucketEvents(ctx, []int64{999999999}))

			count, err := adapter.TestingCountBucketEvents(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, count)
		})
	})
}
