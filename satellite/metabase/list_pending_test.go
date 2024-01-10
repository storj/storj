// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestIteratePendingObjectsWithObjectKey(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

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
			metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			object := metabase.RawObject{
				ObjectStream: pending,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
			}

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

			metabasetest.Verify{Objects: []metabase.RawObject{object}}.Check(ctx, t, db)
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

				metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

				objects[i] = metabase.RawObject{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Pending,

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
				}
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
			metabasetest.CreatePendingObject(ctx, t, db, pending, 0)

			object := metabase.RawObject{
				ObjectStream: pending,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
			}

			metabasetest.IteratePendingObjectsByKey{
				Opts: metabase.IteratePendingObjectsByKey{
					ObjectLocation: pending.Location(),
				},
				Result: []metabase.ObjectEntry{objectEntryFromRaw(object)},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{object}}.Check(ctx, t, db)
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

				metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

				objects[i] = metabase.RawObject{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Pending,

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
				}
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

				metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

				objects[i] = metabase.RawObject{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Pending,

					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieDeadline,
				}
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
			metabasetest.CreatePendingObject(ctx, t, db, pendingStream1, 0)
			pendingObject1 := metabase.RawObject{
				ObjectStream: pendingStream1,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
			}
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
			metabasetest.CreatePendingObject(ctx, t, db, pendingStream2, 0)
			pendingObject2 := metabase.RawObject{
				ObjectStream: pendingStream2,
				CreatedAt:    now,
				Status:       metabase.Pending,

				Encryption:             metabasetest.DefaultEncryption,
				ZombieDeletionDeadline: &zombieDeadline,
			}
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
