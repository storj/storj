// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteExpiredObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := metabasetest.RandObjectStream()
		obj2 := metabasetest.RandObjectStream()
		obj3 := metabasetest.RandObjectStream()

		t.Run("none", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("partial objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			// pending object without expiration time
			pending1 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj1,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// pending object with expiration time in the past
			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// pending object with expiration time in the future
			pending3 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // the object with expiration time in the past is gone
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj1,
						CreatedAt:    pending1.CreatedAt,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: obj3,
						CreatedAt:    pending3.CreatedAt,
						ExpiresAt:    &futureTime,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("batch size", func(t *testing.T) {
			expiresAt := time.Now().Add(-30 * 24 * time.Hour)
			for i := 0; i < 32; i++ {
				_ = metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 3, expiresAt)
			}
			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now().Add(time.Hour),
					BatchSize:     4,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("committed objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			object1, _ := metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 1)
			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj2, 1)
			object3, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj3, 1)

			expectedObj1Segment := metabase.Segment{
				StreamID:          obj1.StreamID,
				RootPieceID:       storj.PieceID{1},
				CreatedAt:         now,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1060,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        metabasetest.DefaultRedundancy,
			}

			expectedObj3Segment := expectedObj1Segment
			expectedObj3Segment.StreamID = obj3.StreamID
			expectedObj3Segment.ExpiresAt = &futureTime

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // the object with expiration time in the past is gone
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

		t.Run("concurrent deletes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			pastTime := now.Add(-1 * time.Hour)

			for _, batchSize := range []int{0, 1, 2, 3, 8, 100} {
				for i := 0; i < 13; i++ {
					_ = metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 3, pastTime)
				}

				metabasetest.DeleteExpiredObjects{
					Opts: metabase.DeleteExpiredObjects{
						ExpiredBefore:     time.Now(),
						DeleteConcurrency: batchSize,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			}
		})
	})
}
