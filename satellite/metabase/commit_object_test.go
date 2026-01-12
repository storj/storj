// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestCommitObject_TimestampVersioning(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("commit without version change", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// When committing an there is no newer version, we should keep the same version
			// to avoid changing the primary key unnecessarily.
			objectStream := metabasetest.RandObjectStream()
			objectStream.Version = metabase.NextVersion
			object1 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			require.Greater(t, object1.Version, int64(0))

			committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: object1.ObjectStream,
				},
				ExpectVersion: object1.Version,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(committed),
			}.Check(ctx, t, db)
		})

		t.Run("commit with version change", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// When committing an there are newer version, we should create a new version
			// to ensure correct ordering.
			objectStream := metabasetest.RandObjectStream()
			objectStream.Version = metabase.NextVersion

			object1 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			assert.Greater(t, object1.Version, int64(0))

			objectStream2 := objectStream
			objectStream2.StreamID = testrand.UUID()
			object2 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream2,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			assert.Greater(t, object2.Version, object1.Version)

			committed1 := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: object1.ObjectStream,
				},
			}.Check(ctx, t, db)
			assert.Greater(t, committed1.Version, object2.Version)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(committed1, object2),
			}.Check(ctx, t, db)
		})
		t.Run("commit inline object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// When committing an there are newer version, we should create a new version
			// to ensure correct ordering.
			objectStream := metabasetest.RandObjectStream()
			objectStream.Version = metabase.NextVersion

			object1 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			assert.Greater(t, object1.Version, int64(0))

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(24)
			inlineData := []byte{1, 2, 3, 4}

			objectStream2 := objectStream
			objectStream2.StreamID = testrand.UUID()
			object2 := metabasetest.CommitInlineObject{Opts: metabase.CommitInlineObject{
				ObjectStream: objectStream2,
				Encryption:   metabasetest.DefaultEncryption,
				CommitInlineSegment: metabase.CommitInlineSegment{
					ObjectStream:      objectStream2,
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,
					PlainSize:         4,
					InlineData:        inlineData,
				},
			}}.Check(ctx, t, db)
			assert.Greater(t, object2.Version, object1.Version)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object1, object2),
				Segments: []metabase.RawSegment{
					{
						StreamID:          object2.StreamID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						CreatedAt:         object2.CreatedAt,
						PlainSize:         4,
						EncryptedSize:     int32(len(inlineData)),
						InlineData:        inlineData,
					},
				},
			}.Check(ctx, t, db)
		})
	}, metabasetest.WithTimestampVersioning)
}

func TestCommitObject(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "metabase-tests",
		MaxNumberOfParts: 10000,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenarios() {
				t.Log(i)

				stream := obj
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))
				opts := metabase.CommitObject{
					ObjectStream: stream,
					Encryption:   metabasetest.DefaultEncryption,

					OverrideEncryptedMetadata: true,
					EncryptedUserData:         scenario.EncryptedUserData,
				}

				metabasetest.CommitObject{
					Opts:     opts,
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  scenario.ErrText,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("version without pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
				},
			}.Check(ctx, t, db)

			// disallow for double commit
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("disallow delete but nothing to delete", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
					DisallowDelete:            true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("disallow delete when committing unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			unversionedStream := obj
			unversionedStream.Version = 3
			unversionedObject := metabasetest.CreateObject(ctx, t, db, unversionedStream, 0)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					EncryptedUserData: userData,
					DisallowDelete:    true,
				},
				ErrClass: &metabase.ErrPermissionDenied,
				ErrText:  "no permissions to delete existing object",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversionedObject),
					metabasetest.ObjectsToRaw(object)[0],
				},
			}.Check(ctx, t, db)
		})

		t.Run("assign plain_offset", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   999999,

					Redundancy: metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   999999,

					Redundancy: metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("large object over 2 GB", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: math.MaxInt32,
					PlainSize:     math.MaxInt32,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: math.MaxInt32,
					PlainSize:     math.MaxInt32,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,
						PlainOffset:   math.MaxInt32,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("commit with encryption", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   storj.EncryptionParameters{},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption is missing",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption.BlockSize is negative or zero",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
						BlockSize:   -1,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption.BlockSize is negative or zero",
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
						BlockSize:   512,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with encryption (no override)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					// set different encryption than with BeginObjectExactVersion
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncNull,
						BlockSize:   512,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with metadata (no overwrite)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with metadata (overwrite)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,

					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with empty metadata (overwrite)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: metabasetest.RandEncryptedUserData(),
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,

					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabase.EncryptedUserData{},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with retention configuration", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: now.Add(time.Minute),
			}

			objA := metabasetest.RandObjectStream()
			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objA,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Check(ctx, t, db)

			objectA := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objA,
				},
			}.Check(ctx, t, db)

			metabasetest.EqualRetention(t, retention, objectA.Retention)

			// use negative version to go through different code path
			objB := metabasetest.RandObjectStream()
			objB.Version = -1
			objectB := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objB,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Check(ctx, t, db)

			objectB = metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectB.ObjectStream,
				},
			}.Check(ctx, t, db)

			metabasetest.EqualRetention(t, retention, objectA.Retention)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(objectA, objectB)}.Check(ctx, t, db)
		})

		t.Run("commit with retention configuration and expiration", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			future := time.Now().Add(time.Minute)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					Retention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: future,
					},
					TestingBypassVerify: true,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					ExpiresAt:    &future,
				},
				ErrClass: &metabase.Error,
				ErrText:  "object expiration must not be set if Object Lock configuration is set",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(object)},
			}.Check(ctx, t, db)
		})

		t.Run("commit with delay", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			maxCommitDelay := 50 * time.Millisecond
			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,

					MaxCommitDelay: &maxCommitDelay,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(object)},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithSkipPendingObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("commit without pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:              obj,
					Encryption:                metabasetest.DefaultEncryption,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserData(),
					SkipPendingObject:         true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,

					Redundancy: metabasetest.DefaultRedundancy,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with multiple segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   999999,

					Redundancy: metabasetest.DefaultRedundancy,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   999999,

					Redundancy: metabasetest.DefaultRedundancy,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: now.Add(time.Minute),
			}

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					Retention:         retention,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with legal hold", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					LegalHold:         true,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit without pending object with expires at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			expiresAt := now.Add(time.Hour)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					ExpiresAt:         &expiresAt,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit replaces existing unversioned object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// create existing unversioned object
			metabasetest.CreateObject(ctx, t, db, obj, 0)

			// commit new object with SkipPendingObject
			newObj := obj
			newObj.StreamID = testrand.UUID()

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      newObj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("double commit with SkipPendingObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			// second commit should succeed and replace the first
			newObj := obj
			newObj.StreamID = testrand.UUID()

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      newObj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit with SkipPendingObject + retention + expires_at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			now := time.Now()
			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: now.Add(time.Minute),
			}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:      obj,
					Encryption:        metabasetest.DefaultEncryption,
					SkipPendingObject: true,
					Retention:         retention,
					ExpiresAt:         &now,
				},
				ErrText: "metabase: object expiration must not be set if Object Lock configuration is set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectVersioned(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		obj.Version = metabase.NextVersion

		t.Run("Commit versioned only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			zombieExpiration := time.Now().Add(24 * time.Hour)

			v1 := obj
			v1Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			v2Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			v3Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Object, v2Object, v3Object)}.Check(ctx, t, db)

			v1Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 1,
			}.Check(ctx, t, db)
			v1.Version = v3.Version + 1

			v2Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 2,
			}.Check(ctx, t, db)
			v2.Version = v1.Version + 1

			v3Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 3,
			}.Check(ctx, t, db)
			v3.Version = v2.Version + 1

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Committed, v3Committed)}.Check(ctx, t, db)
		})

		t.Run("Commit unversioned then versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			zombieExpiration := time.Now().Add(24 * time.Hour)

			v1 := obj
			v1Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			v2Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			v3Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			v4Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			// allow having multiple pending objects

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Object, v2Object, v3Object, v4Object)}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v3.Version = 5

			v1Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v1.Version = 5

			// The latter commit should overwrite the v3.
			// When pending objects table is enabled, then objects
			// get the version during commit, hence the latest version
			// will be the max.

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Object, v4Object)}.Check(ctx, t, db)

			v2Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
					Versioned:    true,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v2.Version = 6

			v4Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
					Versioned:    true,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v4.Version = 7

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Committed, v4Committed)}.Check(ctx, t, db)
		})

		t.Run("Commit versioned then unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			zombieExpiration := time.Now().Add(24 * time.Hour)

			v1 := obj
			v1Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			v2Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			v3Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			v4Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Object, v2Object, v3Object, v4Object)}.Check(ctx, t, db)

			v1Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v1.Version = 5

			v3Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v3.Version = 6

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Object, v3Committed, v4Object)}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v2.Version = 7

			v4Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v4.Version = 7

			// committing v4 should overwrite the previous unversioned commit (v2),
			// so v2 is not in the result check
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v3Committed, v4Committed)}.Check(ctx, t, db)
		})

		t.Run("Commit mixed versioned and unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			zombieExpiration := time.Now().Add(24 * time.Hour)

			v1 := obj
			v1Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			v2Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			v3Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			v4Object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Object, v2Object, v3Object, v4Object)}.Check(ctx, t, db)

			v1Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v1.Version = 5

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Object, v3Object, v4Object)}.Check(ctx, t, db)

			v2Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v2.Version = 6

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Committed, v3Object, v4Object)}.Check(ctx, t, db)

			v3Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v3.Version = 7

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v2Committed, v3Committed, v4Object)}.Check(ctx, t, db)

			v4Committed := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
				},
				ExpectVersion: 8,
			}.Check(ctx, t, db)
			v4.Version = 8

			// committing v4 should overwrite the previous unversioned commit (v2),
			// so v2 is not in the result check
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(v1Committed, v3Committed, v4Committed)}.Check(ctx, t, db)
		})

		t.Run("Commit large number mixed versioned and unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// half the commits are versioned half are unversioned
			numCommits := 1000

			if db.Implementation() == dbutil.Spanner {
				t.Log("TODO(spanner): spanner emulator is too slow for this test, reducing the number to 50")
				numCommits = 50
			}

			objs := make([]*metabase.ObjectStream, numCommits)
			for i := 0; i < numCommits; i++ {
				v := obj
				objs[i] = &v

				zombieExpiration := time.Now().Add(24 * time.Hour)
				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream:           v,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					Version: metabase.Version(i + 1),
				}.Check(ctx, t, db)
				v.Version = metabase.Version(i + 1)
			}

			var unversionedObject metabase.Object
			rawObjects := make([]metabase.RawObject, 0, len(objs))
			for i := range objs {
				versioned := i%2 == 0

				object := metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: *objs[i],
						Versioned:    versioned,
					},
				}.Check(ctx, t, db)

				if versioned {
					rawObjects = append(rawObjects, metabase.RawObject(object))
				} else {
					unversionedObject = object
				}
			}

			// all the unversioned commits overwrite previous unversioned commits,
			// so the result should only contain a single/last unversioned commit.
			rawObjects = append(rawObjects, metabase.RawObject(unversionedObject))

			metabasetest.Verify{Objects: rawObjects}.Check(ctx, t, db)
		})

		t.Run("Commit pending objects with negative version", func(t *testing.T) {
			obj := metabasetest.RandObjectStream()

			expectedObjects := []metabase.RawObject{}
			for i := range 5 {
				obj.Version = metabase.Version(-1 * testrand.Int63n(math.MaxInt64))
				pendingObject := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				object := metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: pendingObject.ObjectStream,
						Versioned:    true,
					},
					ExpectVersion: metabase.Version(i + 1),
				}.Check(ctx, t, db)

				expectedObjects = append(expectedObjects, metabase.RawObject(object))
			}
			metabasetest.Verify{
				Objects: expectedObjects,
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithIncorrectPartSize(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-test",
		MinPartSize:      5 * memory.MiB,
		MaxNumberOfParts: 1000,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("part size less then 5MB", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 2 * memory.MiB.Int32(),
					PlainSize:     2 * memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 2 * memory.MiB.Int32(),
					PlainSize:     2 * memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "size of part number 0 is below minimum threshold, got: 2.0 MiB, min: 5.0 MiB",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 2 * memory.MiB.Int32(),
						PlainSize:     2 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 2 * memory.MiB.Int32(),
						PlainSize:     2 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("size validation with part with multiple segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: memory.MiB.Int32(),
					PlainSize:     memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: memory.MiB.Int32(),
					PlainSize:     memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 0},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),
						PlainOffset:   memory.MiB.Int64(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("size validation with multiple parts", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()
			partsSizes := []memory.Size{6 * memory.MiB, 1 * memory.MiB, 1 * memory.MiB}

			for i, size := range partsSizes {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: uint32(i + 1), Index: 1},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: size.Int32(),
						PlainSize:     size.Int32(),
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)
			}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "size of part number 2 is below minimum threshold, got: 1.0 MiB, min: 5.0 MiB",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 6 * memory.MiB.Int32(),
						PlainSize:     6 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 2, Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 3, Index: 1},
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithIncorrectAmountOfParts(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-test",
		MinPartSize:      5 * memory.MiB,
		MaxNumberOfParts: 3,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("number of parts check", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := object.CreatedAt

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			var segments []metabase.RawSegment

			for i := 1; i < 5; i++ {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: uint32(i), Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 6 * memory.MiB.Int32(),
						PlainSize:     6 * memory.MiB.Int32(),
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				segments = append(segments, metabase.RawSegment{
					StreamID:  obj.StreamID,
					Position:  metabase.SegmentPosition{Part: uint32(i), Index: 0},
					CreatedAt: now,

					RootPieceID:       rootPieceID,
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 6 * memory.MiB.Int32(),
					PlainSize:     6 * memory.MiB.Int32(),

					Redundancy: metabasetest.DefaultRedundancy,

					Pieces: pieces,
				})
			}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "exceeded maximum number of parts: 3",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects:  metabasetest.ObjectsToRaw(object),
				Segments: segments,
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("invalid order", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			pos10 := metabase.SegmentPosition{Part: 1, Index: 0}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {0 1} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos10,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {1 0} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {0 0} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segments missing in database", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments and database does not match: {0 0}: segment not committed",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("delete segments that are not in proofs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			rootPieceID01 := testrand.PieceID()
			pieces01 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos01,
					RootPieceID: rootPieceID01,
					Pieces:      pieces01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						RootPieceID:       rootPieceID01,
						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete inline segments that are not in proofs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			data00 := testrand.Bytes(32)
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			data01 := testrand.Bytes(1024)
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     pos00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					PlainSize:   512,
					PlainOffset: 0,

					InlineData: data00,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     pos01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					PlainSize:   512,
					PlainOffset: 0,

					InlineData: data01,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						InlineData: data01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("updated plain offset", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			rootPieceID01 := testrand.PieceID()
			pieces01 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos01,
					RootPieceID: rootPieceID01,
					Pieces:      pieces01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						RootPieceID:       rootPieceID01,
						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("fixed segment size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos10 := metabase.SegmentPosition{Part: 1, Index: 0}
			rootPieceID10 := testrand.PieceID()
			pieces10 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey10 := testrand.Bytes(32)
			encryptedKeyNonce10 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos10,
					RootPieceID: rootPieceID10,
					Pieces:      pieces10,

					EncryptedKey:      encryptedKey10,
					EncryptedKeyNonce: encryptedKeyNonce10,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos10,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos10,
						CreatedAt: now,

						RootPieceID:       rootPieceID10,
						EncryptedKey:      encryptedKey10,
						EncryptedKeyNonce: encryptedKeyNonce10,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces10,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("skipped fixed segment size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos02 := metabase.SegmentPosition{Part: 0, Index: 2}
			rootPieceID02 := testrand.PieceID()
			pieces02 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey02 := testrand.Bytes(32)
			encryptedKeyNonce02 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos02,
					RootPieceID: rootPieceID02,
					Pieces:      pieces02,

					EncryptedKey:      encryptedKey02,
					EncryptedKeyNonce: encryptedKeyNonce02,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos02,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos02,
						CreatedAt: now,

						RootPieceID:       rootPieceID02,
						EncryptedKey:      encryptedKey02,
						EncryptedKeyNonce: encryptedKeyNonce02,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces02,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitInlineObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		obj.Version = 0

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenarios() {
				t.Log(i)

				stream := obj
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))
				opts := metabase.CommitInlineObject{
					ObjectStream:      stream,
					EncryptedUserData: scenario.EncryptedUserData,
					Encryption:        metabasetest.DefaultEncryption,

					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream:      obj,
						InlineData:        []byte{1, 2, 3},
						EncryptedKey:      []byte{1, 2, 3},
						EncryptedKeyNonce: []byte{1, 2, 3},
					},
				}

				metabasetest.CommitInlineObject{
					Opts:     opts,
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  scenario.ErrText,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey: testrand.Bytes(32),
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,

					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						PlainSize:   512,
						PlainOffset: -1,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)
		})

		t.Run("commit inline object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			inlineData := testrand.Bytes(100)

			object := metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					CommitInlineSegment: metabase.CommitInlineSegment{
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						PlainSize:         512,
						InlineData:        inlineData,
					},
				},
				ExpectVersion: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: now1,

						RootPieceID:       storj.PieceID{},
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: int32(len(inlineData)),
						PlainOffset:   0,
						PlainSize:     512,
						InlineData:    inlineData,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("overwrite", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// create object to override
			objA := obj

			objA.Version = 123
			metabasetest.CreateObject(ctx, t, db, objA, 2)

			objB := obj
			objB.StreamID = testrand.UUID()

			now1 := time.Now()
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			inlineData := testrand.Bytes(100)

			object := metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: objB,
					Encryption:   metabasetest.DefaultEncryption,
					CommitInlineSegment: metabase.CommitInlineSegment{
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						PlainSize:         512,
						InlineData:        inlineData,
					},
				},
				ExpectVersion: objA.Version,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  objB.StreamID,
						CreatedAt: now1,

						RootPieceID:       storj.PieceID{},
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: int32(len(inlineData)),
						PlainOffset:   0,
						PlainSize:     512,
						InlineData:    inlineData,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit inline object versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			obj := obj

			expectedObjects := []metabase.RawObject{}
			expectedSegments := []metabase.RawSegment{}
			for i := 0; i < 3; i++ {
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				inlineData := testrand.Bytes(100)

				obj.StreamID = testrand.UUID()
				expectedObjects = append(expectedObjects, metabase.RawObject(metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						CommitInlineSegment: metabase.CommitInlineSegment{
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,
							PlainSize:         512,
							InlineData:        inlineData,
						},

						Versioned: true,
					},
					ExpectVersion: metabase.Version(i + 1),
				}.Check(ctx, t, db)))

				expectedSegments = append(expectedSegments, metabase.RawSegment{
					StreamID:  obj.StreamID,
					CreatedAt: now1,

					RootPieceID:       storj.PieceID{},
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: int32(len(inlineData)),
					PlainOffset:   0,
					PlainSize:     512,
					InlineData:    inlineData,
				})
			}

			metabasetest.Verify{
				Objects:  expectedObjects,
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})

		commitInlineSeg := metabase.CommitInlineSegment{
			EncryptedKey:      testrand.Bytes(32),
			EncryptedKeyNonce: testrand.Bytes(32),
			PlainSize:         512,
			InlineData:        testrand.Bytes(100),
		}

		t.Run("retention", func(t *testing.T) {
			t.Run("success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(time.Minute),
				}

				object := metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						Retention:           retention,
					},
					ExpectVersion: 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(object),
					Segments: []metabase.RawSegment{{
						StreamID:          obj.StreamID,
						CreatedAt:         object.CreatedAt,
						EncryptedKeyNonce: commitInlineSeg.EncryptedKeyNonce,
						EncryptedKey:      commitInlineSeg.EncryptedKey,
						EncryptedSize:     int32(len(commitInlineSeg.InlineData)),
						PlainSize:         commitInlineSeg.PlainSize,
						InlineData:        commitInlineSeg.InlineData,
					}},
				}.Check(ctx, t, db)
			})

			t.Run("invalid retention configuration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				check := func(mode storj.RetentionMode, retainUntil time.Time, errText string) {
					metabasetest.CommitInlineObject{
						Opts: metabase.CommitInlineObject{
							ObjectStream:        obj,
							Encryption:          metabasetest.DefaultEncryption,
							CommitInlineSegment: commitInlineSeg,
							Retention: metabase.Retention{
								Mode:        mode,
								RetainUntil: retainUntil,
							},
						},
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  errText,
					}.Check(ctx, t, db)
				}

				check(storj.ComplianceMode, time.Time{}, "retention period expiration must be set if retention mode is set")
				check(storj.NoRetention, time.Now().Add(time.Minute), "retention period expiration must not be set if retention mode is not set")
				check(storj.GovernanceMode+1, time.Now().Add(time.Minute), "invalid retention mode 3")

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("retention configuration with TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				expires := time.Now().Add(time.Minute)

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						Retention: metabase.Retention{
							Mode:        storj.ComplianceMode,
							RetainUntil: time.Now().Add(time.Minute),
						},
						ExpiresAt: &expires,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if Retention is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("legal hold", func(t *testing.T) {
			t.Run("success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(time.Minute),
				}

				object := metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						LegalHold:           true,
						// An object's legal hold status and retention mode are stored as a
						// single value in the database. A retention period is provided here
						// to test that these properties are properly encoded.
						Retention: retention,
					},
					ExpectVersion: 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(object),
					Segments: []metabase.RawSegment{{
						StreamID:          obj.StreamID,
						CreatedAt:         object.CreatedAt,
						EncryptedKeyNonce: commitInlineSeg.EncryptedKeyNonce,
						EncryptedKey:      commitInlineSeg.EncryptedKey,
						EncryptedSize:     int32(len(commitInlineSeg.InlineData)),
						PlainSize:         commitInlineSeg.PlainSize,
						InlineData:        commitInlineSeg.InlineData,
					}},
				}.Check(ctx, t, db)
			})

			t.Run("with TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				expires := time.Now().Add(time.Minute)

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						LegalHold:           true,
						ExpiresAt:           &expires,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if LegalHold is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})
	})
}

func TestOverwriteLockedObject(t *testing.T) {
	// This tests a case where an object is committed to an unversioned bucket, but
	// an object version with an active Object Lock configuration is already present
	// in its place. We don't expect any unversioned objects to have Object Lock
	// configurations, but we must ensure that we handle them properly in case we
	// introduce a bug that allows them to exist.
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()

		t.Run("CommitObject", func(t *testing.T) {
			t.Run("Active retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, lockedSegs := metabasetest.CreateObjectWithRetention(ctx, t, db, objStream, 1, metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(time.Hour),
				})

				beginObjStream := objStream
				beginObjStream.Version = metabase.NextVersion
				obj := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: beginObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj.ObjectStream,
					},
					ErrClass: &metabase.ErrObjectLock,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(lockedObj), metabase.RawObject(obj)},
					Segments: []metabase.RawSegment{metabase.RawSegment(lockedSegs[0])},
				}.Check(ctx, t, db)
			})

			t.Run("Expired retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, objStream, 1, metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(-time.Minute),
				})

				beginObjStream := objStream
				beginObjStream.Version = metabase.NextVersion
				obj := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: beginObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				obj = metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj.ObjectStream,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{metabase.RawObject(obj)},
				}.Check(ctx, t, db)
			})
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			getExpectedInlineSegment := func(obj metabase.Object, commitInlineSeg metabase.CommitInlineSegment) metabase.RawSegment {
				return metabase.RawSegment{
					StreamID:          obj.StreamID,
					CreatedAt:         obj.CreatedAt,
					EncryptedKeyNonce: commitInlineSeg.EncryptedKeyNonce,
					EncryptedKey:      commitInlineSeg.EncryptedKey,
					EncryptedSize:     int32(len(commitInlineSeg.InlineData)),
					PlainSize:         commitInlineSeg.PlainSize,
					InlineData:        commitInlineSeg.InlineData,
				}
			}

			commitInlineSeg := metabase.CommitInlineSegment{
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Bytes(32),
				PlainSize:         512,
				InlineData:        testrand.Bytes(100),
			}

			t.Run("Active retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, lockedSegs := metabasetest.CreateObjectWithRetention(ctx, t, db, objStream, 1, metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(time.Hour),
				})

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        objStream,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
					},
					ErrClass: &metabase.ErrObjectLock,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(lockedObj)},
					Segments: []metabase.RawSegment{metabase.RawSegment(lockedSegs[0])},
				}.Check(ctx, t, db)
			})

			t.Run("Expired retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				objStream := metabasetest.RandObjectStream()
				lockedObj, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, objStream, 1, metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().Add(-time.Minute),
				})

				objStream2 := objStream
				objStream2.StreamID = testrand.UUID()
				inlineObj := metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        objStream2,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
					},
					ExpectVersion: lockedObj.Version,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(inlineObj)},
					Segments: []metabase.RawSegment{getExpectedInlineSegment(inlineObj, commitInlineSeg)},
				}.Check(ctx, t, db)
			})
		})
	})
}

func TestConditionalWrites(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		commitObject := func(t *testing.T, objStream metabase.ObjectStream, versioned bool, ifNoneMatch []string, expectedErrClass *errs.Class, expectedErrText string) metabase.Object {
			return metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    versioned,
					IfNoneMatch:  ifNoneMatch,
				},
				ErrClass: expectedErrClass,
				ErrText:  expectedErrText,
			}.Check(ctx, t, db)
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, versioned bool, ifNoneMatch []string, expectedErrClass *errs.Class, expectedErrText string) metabase.Object {
			metabasetest.CreatePendingObject(ctx, t, db, objStream, 0)
			return commitObject(t, objStream, versioned, ifNoneMatch, expectedErrClass, expectedErrText)
		}

		commitInlineObject := func(t *testing.T, objStream metabase.ObjectStream, ifNoneMatch []string, expectedErrClass *errs.Class, expectedErrText string) {
			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					CommitInlineSegment: metabase.CommitInlineSegment{
						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),
						PlainSize:         512,
						InlineData:        testrand.Bytes(100),
					},
					IfNoneMatch: ifNoneMatch,
				},
				ErrClass: expectedErrClass,
				ErrText:  expectedErrText,
			}.Check(ctx, t, db)
		}

		t.Run("CommitObject not implemented", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			createObject(t, objStream, false, []string{"somethingelse"}, &metabase.ErrUnimplemented, "IfNoneMatch only supports a single value of '*'")
		})

		t.Run("CommitInlineObject not implemented", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()

			metabasetest.CreatePendingObject(ctx, t, db, objStream, 0)
			commitInlineObject(t, objStream, []string{"somethingelse"}, &metabase.ErrUnimplemented, "IfNoneMatch only supports a single value of '*'")
		})

		t.Run("CommitObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			object := createObject(t, objStream, false, []string{"*"}, nil, "")

			objStream2 := objStream
			objStream2.Version++
			pending := metabasetest.CreatePendingObject(ctx, t, db, objStream2, 0)
			commitObject(t, objStream2, false, []string{"*"}, &metabase.ErrFailedPrecondition, "object already exists")

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(pending),
				},
			}.Check(ctx, t, db)
		})

		t.Run("CommitObject versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			object := createObject(t, objStream, true, []string{"*"}, nil, "")

			objStream2 := objStream
			objStream2.Version = object.Version + 1
			pending := metabasetest.CreatePendingObject(ctx, t, db, objStream2, 0)
			commitObject(t, objStream2, true, []string{"*"}, &metabase.ErrFailedPrecondition, "object already exists")

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(pending),
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})

		t.Run("DisallowDelete", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			object := createObject(t, objStream, false, []string{"*"}, nil, "")

			objStream2 := objStream
			objStream2.Version = object.Version + 1
			pending := metabasetest.CreatePendingObject(ctx, t, db, objStream2, 0)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:   objStream2,
					DisallowDelete: true,
					IfNoneMatch:    []string{"*"},
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "object already exists",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(pending),
				},
			}.Check(ctx, t, db)
		})

		t.Run("CommitObject versioned delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			object := createObject(t, objStream, true, []string{"*"}, nil, "")

			now := time.Now()

			marker := objStream
			marker.Version = object.Version + 1

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: objStream.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			newObjStream := objStream
			newObjStream.Version = marker.Version + 1
			object2 := createObject(t, newObjStream, true, []string{"*"}, nil, "")

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: newObjStream,
						CreatedAt:    object2.CreatedAt,
						Status:       metabase.CommittedVersioned,
						Encryption:   object2.Encryption,
					},
					{
						ObjectStream: marker,
						CreatedAt:    now,
						Status:       metabase.DeleteMarkerVersioned,
					},
					{
						ObjectStream: objStream,
						CreatedAt:    object.CreatedAt,
						Status:       metabase.CommittedVersioned,
						Encryption:   object.Encryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			object := createObject(t, objStream, false, []string{"*"}, nil, "")
			commitInlineObject(t, objStream, []string{"*"}, &metabase.ErrFailedPrecondition, "object already exists")

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})

		t.Run("CopyObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectID := testrand.UUID()

			srcObjStream := metabasetest.RandObjectStream()
			srcObjStream.ProjectID = projectID
			srcObject := createObject(t, srcObjStream, false, nil, nil, "")

			dstObjStream := metabasetest.RandObjectStream()
			dstObjStream.ProjectID = projectID
			dstObject := createObject(t, dstObjStream, false, nil, nil, "")

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          srcObject.ObjectStream,
					NewStreamID:           dstObjStream.StreamID,
					NewBucket:             dstObjStream.BucketName,
					NewEncryptedObjectKey: dstObjStream.ObjectKey,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
						EncryptedMetadataEncryptedKey: testrand.Bytes(32),
					},
					IfNoneMatch: []string{"*"},
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "object already exists",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(srcObject),
					metabase.RawObject(dstObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Concurrent commits", func(t *testing.T) {
			if db.Implementation() != dbutil.Spanner {
				t.Skip("test requires Spanner")
			}

			requests := 10

			objStreams := make([]metabase.ObjectStream, requests)
			errors := make([]error, requests)

			objStream := metabasetest.RandObjectStream()
			objStream.Version = metabase.NextVersion

			var group errgroup.Group

			for i := 0; i < requests; i++ {
				i := i

				objStream.StreamID = testrand.UUID()
				objStreams[i] = objStream

				group.Go(func() error {
					pendingObject, err := db.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
						ObjectStream: objStreams[i],
						Encryption:   metabasetest.DefaultEncryption,
					})
					if err != nil {
						return err
					}
					_, err = db.CommitObject(ctx, metabase.CommitObject{
						ObjectStream: pendingObject.ObjectStream,
						IfNoneMatch:  []string{"*"},
					})
					errors[i] = err
					return nil
				})
			}

			require.NoError(t, group.Wait())

			var success, failed int

			for _, err := range errors {
				switch {
				case err == nil:
					success++
				case metabase.ErrFailedPrecondition.Has(err):
					failed++
				}
			}

			assert.Equal(t, 1, success)
			assert.Equal(t, requests-1, failed)
		})
	})
}
