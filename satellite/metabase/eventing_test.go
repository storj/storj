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
	"storj.io/storj/shared/s3event"
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

func TestBucketEventingOutboxWrites(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("test requires TiDB")
		}

		tidbAdapter := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter)

		t.Run("CommitObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			committed, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:  obj,
					TransmitEvent: true,
				},
			}.Run(ctx, t, db, obj, 1)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectCreatedPut.Name(),
					ObjectStream:   committed.ObjectStream,
					TotalPlainSize: committed.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("CommitObject no event when TransmitEvent false", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{}.Run(ctx, t, db, obj, 1)

			metabasetest.VerifyBucketEvents{}.Check(ctx, t, db)
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			committed, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:  obj,
					TransmitEvent: true,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectCreatedPut.Name(),
					ObjectStream:   committed.ObjectStream,
					TotalPlainSize: committed.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("DeleteObjectExactVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			committed := metabasetest.CreateObject(ctx, t, db, obj, 1)

			_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: committed.Location(),
				Version:        committed.Version,
				TransmitEvent:  true,
			})
			require.NoError(t, err)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   committed.ObjectStream,
					TotalPlainSize: committed.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("DeleteObjectLastCommittedPlain", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			committed := metabasetest.CreateObject(ctx, t, db, obj, 1)

			_, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: obj.Location(),
				TransmitEvent:  true,
			})
			require.NoError(t, err)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   committed.ObjectStream,
					TotalPlainSize: committed.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("DeleteObjectLastCommittedVersioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.CreateObjectVersioned(ctx, t, db, obj, 1)

			_, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: obj.Location(),
				Versioned:      true,
				TransmitEvent:  true,
			})
			require.NoError(t, err)

			// The delete marker gets a new StreamID and Version assigned by the
			// database, so we can only verify the known fields directly.
			events, err := tidbAdapter.TestingGetAllBucketEvents(ctx)
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.Equal(t, s3event.ObjectRemovedDeleteMarkerCreated.Name(), events[0].EventName)
			require.Equal(t, obj.ProjectID, events[0].ProjectID)
			require.Equal(t, obj.BucketName, events[0].BucketName)
			require.Equal(t, obj.ObjectKey, events[0].ObjectKey)
		})

		t.Run("DeleteAllBucketObjects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID = obj1.ProjectID
			obj2.BucketName = obj1.BucketName
			committed1 := metabasetest.CreateObject(ctx, t, db, obj1, 0)
			committed2 := metabasetest.CreateObject(ctx, t, db, obj2, 0)

			_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: metabase.BucketLocation{
					ProjectID:  obj1.ProjectID,
					BucketName: obj1.BucketName,
				},
				BatchSize:     100,
				TransmitEvent: true,
			})
			require.NoError(t, err)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   committed1.ObjectStream,
					TotalPlainSize: committed1.TotalPlainSize,
				},
				{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   committed2.ObjectStream,
					TotalPlainSize: committed2.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("DeleteAllBucketObjects no event when TransmitEvent false", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.CreateObject(ctx, t, db, obj, 0)

			_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: metabase.BucketLocation{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				BatchSize: 100,
			})
			require.NoError(t, err)

			metabasetest.VerifyBucketEvents{}.Check(ctx, t, db)
		})

		t.Run("FinishCopyObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.CreateObject(ctx, t, db, obj, 1)

			newObj := metabasetest.RandObjectStream()
			newObj.ProjectID = obj.ProjectID

			copyObj, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   metabase.Object{ObjectStream: obj, SegmentCount: 1},
				CopyObjectStream: &newObj,
				FinishObject: &metabase.FinishCopyObject{
					ObjectStream:          obj,
					NewBucket:             newObj.BucketName,
					NewStreamID:           newObj.StreamID,
					NewEncryptedObjectKey: newObj.ObjectKey,
					NewSegmentKeys:        []metabase.EncryptedKeyAndNonce{metabasetest.RandEncryptedKeyAndNonce(0)},
					TransmitEvent:         true,
				},
			}.Run(ctx, t, db)

			metabasetest.VerifyBucketEvents{Expected: []metabase.BucketEvent{
				{
					EventName:      s3event.ObjectCreatedCopy.Name(),
					ObjectStream:   copyObj.ObjectStream,
					TotalPlainSize: copyObj.TotalPlainSize,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("FinishMoveObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			committed := metabasetest.CreateObject(ctx, t, db, obj, 0)

			newObj := metabasetest.RandObjectStream()
			newObj.ProjectID = obj.ProjectID

			err := db.FinishMoveObject(ctx, metabase.FinishMoveObject{
				ObjectStream:          obj,
				NewBucket:             newObj.BucketName,
				NewEncryptedObjectKey: newObj.ObjectKey,
				TransmitEvent:         true,
			})
			require.NoError(t, err)

			// Move emits two events: delete at old location, create at new location.
			// The new version is assigned by the database, so we verify known fields.
			events, err := tidbAdapter.TestingGetAllBucketEvents(ctx)
			require.NoError(t, err)
			require.Len(t, events, 2)

			eventsByName := map[string]metabase.BucketEvent{}
			for _, e := range events {
				eventsByName[e.EventName] = e
			}

			deleteEvent := eventsByName[s3event.ObjectRemovedDelete.Name()]
			require.Equal(t, obj.ProjectID, deleteEvent.ProjectID)
			require.Equal(t, obj.BucketName, deleteEvent.BucketName)
			require.Equal(t, obj.ObjectKey, deleteEvent.ObjectKey)
			require.Equal(t, obj.Version, deleteEvent.Version)
			require.Equal(t, obj.StreamID, deleteEvent.StreamID)
			require.Equal(t, committed.TotalPlainSize, deleteEvent.TotalPlainSize)

			copyEvent := eventsByName[s3event.ObjectCreatedCopy.Name()]
			require.Equal(t, obj.ProjectID, copyEvent.ProjectID)
			require.Equal(t, newObj.BucketName, copyEvent.BucketName)
			require.Equal(t, newObj.ObjectKey, copyEvent.ObjectKey)
			require.Equal(t, obj.StreamID, copyEvent.StreamID)
			require.Equal(t, committed.TotalPlainSize, copyEvent.TotalPlainSize)
		})
	})
}
