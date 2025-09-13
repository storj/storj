// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestFinishCopySegments(t *testing.T) {

	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.Spanner {
			t.Skip("works only with Spanner at the moment")
		}

		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.FinishCopySegments{
					Opts: metabase.FinishCopySegments{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("successful copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := metabasetest.RandObjectStream()
			object := metabasetest.CreateObject(ctx, t, db, stream, 3)
			object.CreatedAt = time.Now()
			zombieDeadline := time.Now().Add(24 * time.Hour)

			pendingStream := metabasetest.RandObjectStream()
			pendingStream.Version = metabase.NextVersion

			expectedKeys := []metabase.EncryptedKeyAndNonce{}
			expectedSegments := []metabase.RawSegment{}
			for i := range int(object.SegmentCount) {
				expectedSegments = append(expectedSegments, metabasetest.DefaultRawSegment(stream, metabase.SegmentPosition{Index: uint32(i)}))
			}

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, int(object.SegmentCount))

			for i, segment := range segments {
				expectedSegments[i].PlainOffset = segment.PlainOffset

				segment := metabase.RawSegment(segment)
				segment.StreamID = pendingStream.StreamID
				segment.Position.Part = 100

				keys := metabasetest.RandEncryptedKeyAndNonce(i)
				keys.EncryptedETag = testrand.Bytes(32)
				expectedKeys = append(expectedKeys, keys)

				segment.EncryptedKey = keys.EncryptedKey
				segment.EncryptedKeyNonce = keys.EncryptedKeyNonce
				segment.EncryptedETag = keys.EncryptedETag

				expectedSegments = append(expectedSegments, segment)
			}

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: pendingStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			pendingStream.Version = 1

			metabasetest.FinishCopySegments{
				Opts: metabase.FinishCopySegments{
					ObjectStream: pendingStream,

					SourceStreamID: stream.StreamID,
					StartOffset:    0,
					EndOffset:      object.TotalPlainSize,
					PartNumber:     100,
					NewSegmentKeys: expectedKeys,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					{
						ObjectStream:           pendingStream,
						CreatedAt:              time.Now(),
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})

		t.Run("invalid end offset", func(t *testing.T) {
			object, segments := metabasetest.CreateTestObject{}.Run(ctx, t, db, obj, 5)

			keys := []metabase.EncryptedKeyAndNonce{}
			for _, segment := range segments {
				keys = append(keys, metabasetest.RandEncryptedKeyAndNonce(int(segment.Position.Index)))
			}

			object.CreatedAt = time.Now()
			zombieDeadline := time.Now().Add(24 * time.Hour)

			pendingStream := metabasetest.RandObjectStream()
			pendingStream.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: pendingStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			pendingStream.Version = 1

			metabasetest.FinishCopySegments{
				Opts: metabase.FinishCopySegments{
					ObjectStream: pendingStream,

					SourceStreamID: object.StreamID,
					StartOffset:    0,
					EndOffset:      object.TotalPlainSize + 100,
					PartNumber:     33,
					NewSegmentKeys: keys,
				},
				ErrText: "metabase: last segment end offset " + strconv.Itoa(int(object.TotalPlainSize)) + " does not match expected end offset " + strconv.Itoa(int(object.TotalPlainSize)+100),
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					{
						ObjectStream:           pendingStream,
						CreatedAt:              time.Now(),
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})
	})
}
