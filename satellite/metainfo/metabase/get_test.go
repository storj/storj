// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestGetObjectExactVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetObjectExactVersion{
					Opts: metabase.GetObjectExactVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get not existing version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 0)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        11,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
				},
				Version: 1,
			}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 0)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLatestVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetObjectLatestVersion{
					Opts: metabase.GetObjectLatestVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
				},
				Version: 1,
			}.Check(ctx, t, db)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 0)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("Get latest object version from multiple", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			firstVersion := obj
			createObject(ctx, t, db, firstVersion, 0)
			secondVersion := metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  obj.ObjectKey,
				Version:    2,
				StreamID:   obj.StreamID,
			}
			createObject(ctx, t, db, secondVersion, 0)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream: secondVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: firstVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
				{
					ObjectStream: secondVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByPosition(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByPosition{
				Opts:     metabase.GetSegmentByPosition{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Segment missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.Error,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 1)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 1,
				},
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			// check non existing segment in existing object
			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 2,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}
