// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestUpdateSegmentPieces(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		now := time.Now()

		validPieces := []metabase.Piece{{
			Number:      1,
			StorageNode: testrand.NodeID(),
		}}

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts:     metabase.UpdateSegmentPieces{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: pieces missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces: piece number 1 is missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: duplicated piece number 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewRedundancy zero", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewRedundancy zero",
			}.Check(ctx, t, db)
		})

		t.Run("NewPieces vs NewRedundancy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					NewRedundancy: metabasetest.DefaultRedundancy,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "number of new pieces is less than new redundancy repair shares value",
			}.Check(ctx, t, db)
		})

		t.Run("NewPieces: piece number 1 is missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: duplicated piece number 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segment not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 1},
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces:     validPieces,
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)
		})

		t.Run("segment pieces column was changed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.CreateObject(ctx, t, db, obj, 1)

			newRedundancy := storj.RedundancyScheme{
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  1,
				TotalShares:    4,
			}

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     validPieces,
					NewRedundancy: newRedundancy,
					NewPieces: metabase.Pieces{
						metabase.Piece{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrValueChanged,
				ErrText:  "segment remote_alias_pieces field was changed",
			}.Check(ctx, t, db)

			// verify that original pieces and redundancy did not change
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(obj),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:          obj.StreamID,
						RootPieceID:       storj.PieceID{1},
						CreatedAt:         now,
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},
						EncryptedSize:     1024,
						PlainOffset:       0,
						PlainSize:         512,

						Redundancy: metabasetest.DefaultRedundancy,
						Pieces:     metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 1)

			segment, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)

			expectedPieces := metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: testrand.NodeID(),
				},
				metabase.Piece{
					Number:      2,
					StorageNode: testrand.NodeID(),
				},
			}

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     segment.Pieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces:     expectedPieces,
				},
			}.Check(ctx, t, db)

			expectedSegment := segment
			expectedSegment.Pieces = expectedPieces
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces and repair at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 1)

			segment, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)

			expectedPieces := metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: testrand.NodeID(),
				},
				metabase.Piece{
					Number:      2,
					StorageNode: testrand.NodeID(),
				},
			}

			repairedAt := now.Add(time.Hour)
			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     segment.Pieces,
					NewRedundancy: segment.Redundancy,
					NewPieces:     expectedPieces,
					NewRepairedAt: repairedAt,
				},
			}.Check(ctx, t, db)

			expectedSegment := segment
			expectedSegment.Pieces = expectedPieces
			expectedSegment.RepairedAt = &repairedAt

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)

			segment = segments[0]

			require.NoError(t, err)
			diff := cmp.Diff(expectedSegment, segment, metabasetest.DefaultTimeDiff())
			require.Zero(t, diff)

			segment, err = db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)
			diff = cmp.Diff(expectedSegment, segment, metabasetest.DefaultTimeDiff())
			require.Zero(t, diff)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestSetObjectExactVersionRetention(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()
		loc := objStream.Location()

		activeRetention := metabase.Retention{
			Mode:        storj.ComplianceMode,
			RetainUntil: time.Now().Add(time.Hour),
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, retention metabase.Retention) metabase.Object {
			obj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return obj
		}

		t.Run("Set retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := objStream

			// obj1 and obj3 exist to ensure that SetObjectExactVersionRetention
			// does not select the first or last version instead of the version
			// it is given.
			obj1 := createObject(t, objStream, metabase.Retention{})
			objStream.Version++
			obj2 := createObject(t, objStream, metabase.Retention{})
			objStream.Version++
			obj3 := createObject(t, objStream, metabase.Retention{})

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        obj2.Version,
					Retention:      activeRetention,
				},
			}.Check(ctx, t, db)

			// Ensure that retention periods can be extended.
			extendedRetention := activeRetention
			extendedRetention.RetainUntil = extendedRetention.RetainUntil.Add(time.Hour)
			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        obj2.Version,
					Retention:      extendedRetention,
				},
			}.Check(ctx, t, db)
			obj2.Retention = extendedRetention

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(obj1),
					metabase.RawObject(obj2),
					metabase.RawObject(obj3),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Remove retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := objStream

			noRetentionObj := createObject(t, objStream, metabase.Retention{})

			objStream.Version++
			expiredRetentionObj := createObject(t, objStream, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Now().Add(-time.Minute),
			})

			objStream.Version++
			activeRetentionObj := createObject(t, objStream, activeRetention)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        noRetentionObj.Version,
				},
			}.Check(ctx, t, db)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        expiredRetentionObj.Version,
				},
			}.Check(ctx, t, db)
			expiredRetentionObj.Retention = metabase.Retention{}

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        activeRetentionObj.Version,
				},
				ErrClass: &metabase.ErrObjectLock,
				ErrText:  "an active retention configuration cannot be removed",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(noRetentionObj),
					metabase.RawObject(expiredRetentionObj),
					metabase.RawObject(activeRetentionObj),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Shorten retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := createObject(t, objStream, activeRetention)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        obj.Version,
					Retention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: activeRetention.RetainUntil.Add(-time.Minute),
					},
				},
				ErrClass: &metabase.ErrObjectLock,
				ErrText:  "retention period cannot be shortened",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Invalid retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := createObject(t, objStream, activeRetention)

			check := func(retention metabase.Retention, errText string) {
				metabasetest.SetObjectExactVersionRetention{
					Opts: metabase.SetObjectExactVersionRetention{
						ObjectLocation: loc,
						Version:        obj.Version,
						Retention:      retention,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  errText,
				}.Check(ctx, t, db)
			}

			check(metabase.Retention{
				Mode: storj.ComplianceMode,
			}, "retention period expiration must be set if retention mode is set")

			check(metabase.Retention{
				RetainUntil: activeRetention.RetainUntil,
			}, "retention period expiration must not be set if retention mode is not set")

			check(metabase.Retention{
				Mode:        storj.RetentionMode(2),
				RetainUntil: activeRetention.RetainUntil,
			}, "retention mode must be 0 (none) or 1 (compliance), but it was 2")

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        objStream.Version,
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			obj := createObject(t, objStream, metabase.Retention{})

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        obj.Version + 1,
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        pending.Version,
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  "Object Lock settings must only be placed on committed objects",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(pending)},
			}.Check(ctx, t, db)
		})

		t.Run("Object with TTL", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expiresAt := time.Now().Add(time.Minute)

			ttlObj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					ExpiresAt:    &expiresAt,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: loc,
					Version:        ttlObj.Version,
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  "Object Lock settings must not be placed on an object with an expiration date",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(ttlObj)},
			}.Check(ctx, t, db)
		})
	})
}

func TestSetObjectLastCommittedRetention(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()
		loc := objStream.Location()

		activeRetention := metabase.Retention{
			Mode:        storj.ComplianceMode,
			RetainUntil: time.Now().Add(time.Hour),
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, retention metabase.Retention) metabase.Object {
			obj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return obj
		}

		t.Run("Set retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := objStream

			obj1 := createObject(t, objStream, metabase.Retention{})
			objStream.Version++
			obj2 := createObject(t, objStream, metabase.Retention{})

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
					Retention:      activeRetention,
				},
			}.Check(ctx, t, db)

			// Ensure that retention periods can be extended.
			extendedRetention := activeRetention
			extendedRetention.RetainUntil = extendedRetention.RetainUntil.Add(time.Hour)
			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
					Retention:      extendedRetention,
				},
			}.Check(ctx, t, db)
			obj2.Retention = extendedRetention

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj1), metabase.RawObject(obj2)},
			}.Check(ctx, t, db)
		})

		t.Run("Remove retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := objStream

			noRetentionObj := createObject(t, objStream, metabase.Retention{})

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
				},
			}.Check(ctx, t, db)

			objStream.Version++
			expiredRetentionObj := createObject(t, objStream, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Now().Add(-time.Minute),
			})

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
				},
			}.Check(ctx, t, db)
			expiredRetentionObj.Retention = metabase.Retention{}

			objStream.Version++
			activeRetentionObj := createObject(t, objStream, activeRetention)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
				},
				ErrClass: &metabase.ErrObjectLock,
				ErrText:  "an active retention configuration cannot be removed",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(noRetentionObj),
					metabase.RawObject(expiredRetentionObj),
					metabase.RawObject(activeRetentionObj),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Shorten retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := createObject(t, objStream, activeRetention)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
					Retention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: activeRetention.RetainUntil.Add(-time.Minute),
					},
				},
				ErrClass: &metabase.ErrObjectLock,
				ErrText:  "retention period cannot be shortened",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Invalid retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := createObject(t, objStream, activeRetention)

			check := func(retention metabase.Retention, errText string) {
				metabasetest.SetObjectLastCommittedRetention{
					Opts: metabase.SetObjectLastCommittedRetention{
						ObjectLocation: loc,
						Retention:      retention,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  errText,
				}.Check(ctx, t, db)
			}

			check(metabase.Retention{
				Mode: storj.ComplianceMode,
			}, "retention period expiration must be set if retention mode is set")

			check(metabase.Retention{
				RetainUntil: activeRetention.RetainUntil,
			}, "retention period expiration must not be set if retention mode is not set")

			check(metabase.Retention{
				Mode:        storj.RetentionMode(2),
				RetainUntil: activeRetention.RetainUntil,
			}, "retention mode must be 0 (none) or 1 (compliance), but it was 2")

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: objStream.Location(),
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := objStream

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: pending.ObjectStream,
				},
			}.Check(ctx, t, db)

			objStream.Version++
			pending = metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: loc,
					Retention:      activeRetention,
				},
			}.Check(ctx, t, db)
			committed.Retention = activeRetention

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(committed), metabase.RawObject(pending)},
			}.Check(ctx, t, db)
		})

		t.Run("Object with TTL", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expiresAt := time.Now().Add(time.Minute)

			ttlObj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					ExpiresAt:    &expiresAt,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.SetObjectLastCommittedRetention{
				Opts: metabase.SetObjectLastCommittedRetention{
					ObjectLocation: objStream.Location(),
					Retention:      activeRetention,
				},
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  "Object Lock settings must not be placed on an object with an expiration date",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(ttlObj)},
			}.Check(ctx, t, db)
		})
	})
}
