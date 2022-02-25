// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// RandObjectStream returns a random object stream.
func RandObjectStream() metabase.ObjectStream {
	return metabase.ObjectStream{
		ProjectID:  testrand.UUID(),
		BucketName: testrand.BucketName(),
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}
}

// CreatePendingObject creates a new pending object with the specified number of segments.
func CreatePendingObject(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   DefaultEncryption,
		},
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{i + 1},
				Pieces: []metabase.Piece{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			},
		}.Check(ctx, t, db)

		CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{1},
				Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1024,
				PlainSize:     512,
				PlainOffset:   0,
				Redundancy:    DefaultRedundancy,
			},
		}.Check(ctx, t, db)
	}
}

// CreateObject creates a new committed object with the specified number of segments.
func CreateObject(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   DefaultEncryption,
		},
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{i + 1},
				Pieces: []metabase.Piece{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			},
		}.Check(ctx, t, db)

		CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{1},
				Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1024,
				PlainSize:     512,
				PlainOffset:   0,
				Redundancy:    DefaultRedundancy,
			},
		}.Check(ctx, t, db)
	}

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}

// CreateExpiredObject creates a new committed expired object with the specified number of segments.
func CreateExpiredObject(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte, expiresAt time.Time) metabase.Object {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   DefaultEncryption,
			ExpiresAt:    &expiresAt,
		},
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{i + 1},
				Pieces: []metabase.Piece{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			},
		}.Check(ctx, t, db)

		CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				ExpiresAt:    &expiresAt,
				RootPieceID:  storj.PieceID{1},
				Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1024,
				PlainSize:     512,
				PlainOffset:   0,
				Redundancy:    DefaultRedundancy,
			},
		}.Check(ctx, t, db)
	}

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}

// CreateFullObjectsWithKeys creates multiple objects with the specified keys.
func CreateFullObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.LoopObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.LoopObjectEntry, len(keys))
	for _, key := range keys {
		obj := RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key

		CreateObject(ctx, t, db, obj, 0)

		objects[key] = metabase.LoopObjectEntry{
			ObjectStream: obj,
			Status:       metabase.Committed,
			CreatedAt:    time.Now(),
		}
	}

	return objects
}

// CreateTestObject is for testing metabase.CreateTestObject.
type CreateTestObject struct {
	BeginObjectExactVersion *metabase.BeginObjectExactVersion
	CommitObject            *metabase.CommitObject
	// TODO add BeginSegment, CommitSegment
}

// Run runs the test.
func (co CreateTestObject) Run(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	boeOpts := metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   DefaultEncryption,
	}
	if co.BeginObjectExactVersion != nil {
		boeOpts = *co.BeginObjectExactVersion
	}

	BeginObjectExactVersion{
		Opts:    boeOpts,
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{i + 1},
				Pieces: []metabase.Piece{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			},
		}.Check(ctx, t, db)

		CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				ExpiresAt:    boeOpts.ExpiresAt,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{1},
				Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1060,
				PlainSize:     512,
				PlainOffset:   int64(i) * 512,
				Redundancy:    DefaultRedundancy,
			},
		}.Check(ctx, t, db)
	}

	coOpts := metabase.CommitObject{
		ObjectStream: obj,
	}
	if co.CommitObject != nil {
		coOpts = *co.CommitObject
	}

	return CommitObject{
		Opts: coOpts,
	}.Check(ctx, t, db)
}

// CreateObjectCopy is for testing object copy.
type CreateObjectCopy struct {
	OriginalObject   metabase.Object
	FinishObject     *metabase.FinishCopyObject
	CopyObjectStream *metabase.ObjectStream
}

// Run creates the copy.
func (cc CreateObjectCopy) Run(ctx *testcontext.Context, t testing.TB, db *metabase.DB) (metabase.Object, []metabase.RawSegment) {

	var copyStream metabase.ObjectStream
	if cc.CopyObjectStream != nil {
		copyStream = *cc.CopyObjectStream
	} else {
		copyStream = RandObjectStream()
	}

	newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, cc.OriginalObject.SegmentCount)
	expectedSegments := make([]metabase.RawSegment, cc.OriginalObject.SegmentCount*2)
	expectedEncryptedSize := 1060

	for i := 0; i < int(cc.OriginalObject.SegmentCount); i++ {
		newEncryptedKeysNonces[i] = metabase.EncryptedKeyAndNonce{
			Position:          metabase.SegmentPosition{Index: uint32(i)},
			EncryptedKeyNonce: testrand.Nonce().Bytes(),
			EncryptedKey:      testrand.Bytes(32),
		}

		copyIndex := i + int(cc.OriginalObject.SegmentCount)
		expectedSegments[i] = DefaultRawSegment(cc.OriginalObject.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})

		// TODO: place this calculation in metabasetest.
		expectedSegments[i].PlainOffset = int64(int32(i) * expectedSegments[i].PlainSize)
		// TODO: we should use the same value for encrypted size in both test methods.
		expectedSegments[i].EncryptedSize = int32(expectedEncryptedSize)

		expectedSegments[copyIndex] = metabase.RawSegment{}
		expectedSegments[copyIndex].StreamID = copyStream.StreamID
		expectedSegments[copyIndex].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
		expectedSegments[copyIndex].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
		expectedSegments[copyIndex].EncryptedSize = expectedSegments[i].EncryptedSize
		expectedSegments[copyIndex].Position = expectedSegments[i].Position
		expectedSegments[copyIndex].RootPieceID = expectedSegments[i].RootPieceID
		expectedSegments[copyIndex].Redundancy = expectedSegments[i].Redundancy
		expectedSegments[copyIndex].PlainSize = expectedSegments[i].PlainSize
		expectedSegments[copyIndex].PlainOffset = expectedSegments[i].PlainOffset
		expectedSegments[copyIndex].CreatedAt = time.Now().UTC()
	}

	opts := cc.FinishObject
	if opts == nil {
		opts = &metabase.FinishCopyObject{
			NewStreamID:                  copyStream.StreamID,
			NewBucket:                    copyStream.BucketName,
			ObjectStream:                 cc.OriginalObject.ObjectStream,
			NewSegmentKeys:               newEncryptedKeysNonces,
			NewEncryptedObjectKey:        []byte(copyStream.ObjectKey),
			NewEncryptedMetadataKeyNonce: testrand.Nonce().Bytes(),
			NewEncryptedMetadataKey:      testrand.Bytes(32),
		}
	}
	FinishCopyObject{
		Opts:    *opts,
		ErrText: "",
	}.Check(ctx, t, db)

	copyObj := cc.OriginalObject
	copyObj.StreamID = copyStream.StreamID
	copyObj.ObjectKey = copyStream.ObjectKey
	copyObj.EncryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
	copyObj.EncryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce
	copyObj.BucketName = copyStream.BucketName
	copyObj.ZombieDeletionDeadline = nil

	return copyObj, expectedSegments
}
