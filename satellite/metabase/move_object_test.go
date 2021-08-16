// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestBeginMoveObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginMoveObject{
					Opts: metabase.BeginMoveObject{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginMoveObject{
				Opts: metabase.BeginMoveObject{
					ObjectLocation: obj.Location(),
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("begin move object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedMetadataNonce := testrand.Nonce()
			expectedMetadataKey := testrand.Bytes(265)
			expectedObject := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        expectedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: expectedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 10)

			var encKeyAndNonces []metabase.EncryptedKeyAndNonce
			expectedRawSegments := make([]metabase.RawSegment, 10)
			for i := range expectedRawSegments {
				expectedRawSegments[i] = metabasetest.DefaultRawSegment(expectedObject.ObjectStream, metabase.SegmentPosition{
					Index: uint32(i),
				})
				expectedRawSegments[i].PlainOffset = int64(i) * int64(expectedRawSegments[i].PlainSize)
				expectedRawSegments[i].EncryptedSize = 1060

				encKeyAndNonces = append(encKeyAndNonces, metabase.EncryptedKeyAndNonce{
					EncryptedKeyNonce: expectedRawSegments[i].EncryptedKeyNonce,
					EncryptedKey:      expectedRawSegments[i].EncryptedKey,
					Position:          expectedRawSegments[i].Position,
				})
			}

			metabasetest.BeginMoveObject{
				Opts: metabase.BeginMoveObject{
					Version:        expectedObject.Version,
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginMoveObjectResult{
					StreamID:                  expectedObject.StreamID,
					EncryptedMetadataKey:      expectedMetadataKey,
					EncryptedMetadataKeyNonce: expectedMetadataNonce[:],
					EncryptedKeysNonces:       encKeyAndNonces,
					EncryptionParameters:      expectedObject.Encryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})
	})
}
