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

func TestDeleteExpiredObjects(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := randObjectStream()
		obj2 := randObjectStream()
		obj3 := randObjectStream()

		now := time.Now()
		pastTime := now.Add(-1 * time.Hour)
		futureTime := now.Add(1 * time.Hour)

		t.Run("Empty metabase", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteExpiredObjects{}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete expired partial objects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			// pending object without expiration time
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj1,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			// pending object with expiration time in the past
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			// pending object with expiration time in the future
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeleteExpiredObjects{}.Check(ctx, t, db)

			Verify{ // the object with expiration time in the past is gone
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj1,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: obj3,
						CreatedAt:    now,
						ExpiresAt:    &futureTime,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete expired committed objects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object1 := CreateTestObject{}.Run(ctx, t, db, obj1, 1)
			CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   defaultTestEncryption,
				},
			}.Run(ctx, t, db, obj2, 1)
			object3 := CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   defaultTestEncryption,
				},
			}.Run(ctx, t, db, obj3, 1)

			expectedObj1Segment := metabase.Segment{
				StreamID:          obj1.StreamID,
				RootPieceID:       storj.PieceID{1},
				CreatedAt:         &now,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1060,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedObj3Segment := expectedObj1Segment
			expectedObj3Segment.StreamID = obj3.StreamID

			DeleteExpiredObjects{}.Check(ctx, t, db)

			Verify{ // the object with expiration time in the past is gone
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object3),
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedObj1Segment),
					metabase.RawSegment(expectedObj3Segment),
				},
			}.Check(ctx, t, db)
		})
	})
}
