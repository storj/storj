// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
		ObjectKey:  RandObjectKey(),
		Version:    12345,
		StreamID:   testrand.UUID(),
	}
}

// RandObjectKey returns a random object key.
func RandObjectKey() metabase.ObjectKey {
	return metabase.ObjectKey(testrand.Bytes(16))
}

// RandEncryptedKeyAndNonce generates random segment metadata.
func RandEncryptedKeyAndNonce(position int) metabase.EncryptedKeyAndNonce {
	return metabase.EncryptedKeyAndNonce{
		Position:          metabase.SegmentPosition{Index: uint32(position)},
		EncryptedKeyNonce: testrand.Nonce().Bytes(),
		EncryptedKey:      testrand.Bytes(32),
	}
}

// CreatePendingObject creates a new pending object with the specified number of segments.
func CreatePendingObject(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	object := BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   DefaultEncryption,
		},
	}.Check(ctx, t, db)

	CreateSegments(ctx, t, db, obj, nil, numberOfSegments)
	return object
}

// CreateObject creates a new committed object with the specified number of segments.
func CreateObject(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	CreatePendingObject(ctx, t, db, obj, numberOfSegments)

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}

// CreateObjectVersioned creates a new committed object with the specified number of segments.
func CreateObjectVersioned(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	CreatePendingObject(ctx, t, db, obj, numberOfSegments)

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
			Versioned:    true,
		},
	}.Check(ctx, t, db)
}

// CreateObjectVersionedOutOfOrder creates a new committed object with the specified number of segments.
func CreateObjectVersionedOutOfOrder(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte, expectVersion metabase.Version) metabase.Object {
	CreatePendingObject(ctx, t, db, obj, numberOfSegments)

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
			Versioned:    true,
		},
		ExpectVersion: expectVersion,
	}.Check(ctx, t, db)
}

// CreateExpiredObject creates a new committed expired object with the specified number of segments.
func CreateExpiredObject(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte, expiresAt time.Time) metabase.Object {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   DefaultEncryption,
			ExpiresAt:    &expiresAt,
		},
	}.Check(ctx, t, db)

	CreateSegments(ctx, t, db, obj, &expiresAt, numberOfSegments)

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}

// CreateFullObjectsWithKeys creates multiple objects with the specified keys.
func CreateFullObjectsWithKeys(ctx *testcontext.Context, t testing.TB, db *metabase.DB, projectID uuid.UUID, bucketName string, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.LoopObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.LoopObjectEntry, len(keys))
	for _, key := range keys {
		obj := RandObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key

		CreateObject(ctx, t, db, obj, 0)

		objects[key] = metabase.LoopObjectEntry{
			ObjectStream: obj,
			Status:       metabase.CommittedUnversioned,
			CreatedAt:    time.Now(),
		}
	}

	return objects
}

// CreateSegments creates multiple segments for the specified object.
func CreateSegments(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, expiresAt *time.Time, numberOfSegments byte) {
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

				ExpiresAt: expiresAt,

				Pieces: metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

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

// CreateVersionedObjectsWithKeys creates multiple versioned objects with the specified keys and versions,
// and returns a mapping of keys to final versions.
func CreateVersionedObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys map[metabase.ObjectKey][]metabase.Version) map[metabase.ObjectKey]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.ObjectEntry, len(keys))
	for key, versions := range keys {
		for _, version := range versions {
			obj := RandObjectStream()
			obj.ProjectID = projectID
			obj.BucketName = bucketName
			obj.ObjectKey = key
			obj.Version = version
			now := time.Now()

			CreateObjectVersioned(ctx, t, db, obj, 0)

			objects[key] = metabase.ObjectEntry{
				ObjectKey:  obj.ObjectKey,
				Version:    obj.Version,
				StreamID:   obj.StreamID,
				CreatedAt:  now,
				Status:     metabase.CommittedVersioned,
				Encryption: DefaultEncryption,
			}
		}
	}

	return objects
}

// CreatePendingObjectsWithKeys creates multiple versioned objects with the specified keys and versions,
// and returns a mapping of keys to all versions.
func CreatePendingObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys map[metabase.ObjectKey][]metabase.Version) map[metabase.ObjectKey]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.ObjectEntry, len(keys))
	for key, versions := range keys {
		for _, version := range versions {
			obj := RandObjectStream()
			obj.ProjectID = projectID
			obj.BucketName = bucketName
			obj.ObjectKey = key
			obj.Version = version
			now := time.Now()

			CreatePendingObject(ctx, t, db, obj, 0)

			k := key + ":" + metabase.ObjectKey(strconv.Itoa(int(version)))
			objects[k] = metabase.ObjectEntry{
				ObjectKey:  obj.ObjectKey,
				Version:    obj.Version,
				StreamID:   obj.StreamID,
				CreatedAt:  now,
				Status:     metabase.Pending,
				Encryption: DefaultEncryption,
			}
		}
	}

	return objects
}

// CreateVersionedObjectsWithKeysAll creates multiple versioned objects with the specified keys and versions,
// and returns a mapping of keys to a slice of all versions.
func CreateVersionedObjectsWithKeysAll(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys map[metabase.ObjectKey][]metabase.Version) map[metabase.ObjectKey][]metabase.ObjectEntry {
	objects := make(map[metabase.ObjectKey][]metabase.ObjectEntry, len(keys))
	for key, versions := range keys {
		items := []metabase.ObjectEntry{}
		for _, version := range versions {
			obj := RandObjectStream()
			obj.ProjectID = projectID
			obj.BucketName = bucketName
			obj.ObjectKey = key
			obj.Version = version
			now := time.Now()

			CreateObjectVersioned(ctx, t, db, obj, 0)

			items = append(items, metabase.ObjectEntry{
				ObjectKey:  obj.ObjectKey,
				Version:    obj.Version,
				StreamID:   obj.StreamID,
				CreatedAt:  now,
				Status:     metabase.CommittedVersioned,
				Encryption: DefaultEncryption,
			})
		}
		// sort by version descending
		sort.Slice(items, func(i, k int) bool {
			return items[i].Less(items[k])
		})
		objects[key] = items
	}

	return objects
}

// CreateTestObject is for testing metabase.CreateTestObject.
type CreateTestObject struct {
	BeginObjectExactVersion *metabase.BeginObjectExactVersion
	CommitObject            *metabase.CommitObject
	ExpectVersion           metabase.Version
	CreateSegment           func(object metabase.Object, index int) metabase.Segment
}

// Run runs the test.
func (co CreateTestObject) Run(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) (metabase.Object, []metabase.Segment) {
	boeOpts := metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   DefaultEncryption,
	}
	if co.BeginObjectExactVersion != nil {
		boeOpts = *co.BeginObjectExactVersion
	}

	object, err := db.TestingBeginObjectExactVersion(ctx, boeOpts)
	require.NoError(t, err)

	createdSegments := []metabase.Segment{}
	for i := byte(0); i < numberOfSegments; i++ {
		if co.CreateSegment != nil {
			segment := co.CreateSegment(object, int(i))
			createdSegments = append(createdSegments, segment)
		} else {
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

			commitSegmentOpts := metabase.CommitSegment{
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
			}

			CommitSegment{
				Opts: commitSegmentOpts,
			}.Check(ctx, t, db)

			segment, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: commitSegmentOpts.StreamID,
				Position: commitSegmentOpts.Position,
			})
			require.NoError(t, err)

			createdSegments = append(createdSegments, metabase.Segment{
				StreamID: obj.StreamID,
				Position: commitSegmentOpts.Position,

				CreatedAt:  segment.CreatedAt,
				RepairedAt: nil,
				ExpiresAt:  nil,

				RootPieceID:       commitSegmentOpts.RootPieceID,
				EncryptedKeyNonce: commitSegmentOpts.EncryptedKeyNonce,
				EncryptedKey:      commitSegmentOpts.EncryptedKey,

				EncryptedSize: commitSegmentOpts.EncryptedSize,
				PlainSize:     commitSegmentOpts.PlainSize,
				PlainOffset:   commitSegmentOpts.PlainOffset,
				EncryptedETag: commitSegmentOpts.EncryptedETag,

				Redundancy: commitSegmentOpts.Redundancy,

				InlineData: nil,
				Pieces:     commitSegmentOpts.Pieces,

				Placement: segment.Placement,
			})
		}
	}

	coOpts := metabase.CommitObject{
		ObjectStream: obj,
	}
	if co.CommitObject != nil {
		coOpts = *co.CommitObject
	}

	createdObject := CommitObject{
		Opts:          coOpts,
		ExpectVersion: co.ExpectVersion,
	}.Check(ctx, t, db)

	return createdObject, createdSegments
}

// CreateObjectCopy is for testing object copy.
type CreateObjectCopy struct {
	OriginalObject metabase.Object
	// if empty, creates fake segments if necessary
	OriginalSegments []metabase.Segment
	FinishObject     *metabase.FinishCopyObject
	CopyObjectStream *metabase.ObjectStream

	NewDisallowDelete bool
	NewVersioned      bool
}

// Run creates the copy.
func (cc CreateObjectCopy) Run(ctx *testcontext.Context, t testing.TB, db *metabase.DB) (copyObj metabase.Object, expectedOriginalSegments []metabase.RawSegment, expectedCopySegments []metabase.RawSegment) {
	var copyStream metabase.ObjectStream
	if cc.CopyObjectStream != nil {
		copyStream = *cc.CopyObjectStream
	} else {
		copyStream = RandObjectStream()
	}

	newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, cc.OriginalObject.SegmentCount)
	expectedOriginalSegments = make([]metabase.RawSegment, cc.OriginalObject.SegmentCount)
	expectedCopySegments = make([]metabase.RawSegment, cc.OriginalObject.SegmentCount)
	expectedEncryptedSize := 1060

	for i := 0; i < int(cc.OriginalObject.SegmentCount); i++ {
		newEncryptedKeysNonces[i] = RandEncryptedKeyAndNonce(i)

		expectedOriginalSegments[i] = DefaultRawSegment(cc.OriginalObject.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})

		// TODO: place this calculation in metabasetest.
		expectedOriginalSegments[i].PlainOffset = int64(int32(i) * expectedOriginalSegments[i].PlainSize)
		// TODO: we should use the same value for encrypted size in both test methods.
		expectedOriginalSegments[i].EncryptedSize = int32(expectedEncryptedSize)

		expectedCopySegments[i] = metabase.RawSegment{}
		expectedCopySegments[i].StreamID = copyStream.StreamID
		expectedCopySegments[i].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
		expectedCopySegments[i].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
		expectedCopySegments[i].EncryptedSize = expectedOriginalSegments[i].EncryptedSize
		expectedCopySegments[i].Position = expectedOriginalSegments[i].Position
		expectedCopySegments[i].RootPieceID = expectedOriginalSegments[i].RootPieceID
		expectedCopySegments[i].Redundancy = expectedOriginalSegments[i].Redundancy
		expectedCopySegments[i].PlainSize = expectedOriginalSegments[i].PlainSize
		expectedCopySegments[i].PlainOffset = expectedOriginalSegments[i].PlainOffset
		expectedCopySegments[i].CreatedAt = time.Now().UTC()
		if len(expectedOriginalSegments[i].InlineData) > 0 {
			expectedCopySegments[i].InlineData = expectedOriginalSegments[i].InlineData
		} else {
			expectedCopySegments[i].InlineData = []byte{}
		}

		expectedCopySegments[i].Pieces = make(metabase.Pieces, len(expectedOriginalSegments[i].Pieces))
		copy(expectedCopySegments[i].Pieces, expectedOriginalSegments[i].Pieces)
	}

	opts := cc.FinishObject
	if opts == nil {
		opts = &metabase.FinishCopyObject{
			ObjectStream:                 cc.OriginalObject.ObjectStream,
			NewStreamID:                  copyStream.StreamID,
			NewBucket:                    copyStream.BucketName,
			NewSegmentKeys:               newEncryptedKeysNonces,
			NewEncryptedObjectKey:        copyStream.ObjectKey,
			NewEncryptedMetadataKeyNonce: testrand.Nonce(),
			NewEncryptedMetadataKey:      testrand.Bytes(32),

			NewDisallowDelete: cc.NewDisallowDelete,
			NewVersioned:      cc.NewVersioned,
		}
	}

	copyObj, err := db.FinishCopyObject(ctx, *opts)
	require.NoError(t, err)

	return copyObj, expectedOriginalSegments, expectedCopySegments
}

// SegmentsToRaw converts a slice of Segment to a slice of RawSegment.
func SegmentsToRaw(segments []metabase.Segment) []metabase.RawSegment {
	rawSegments := []metabase.RawSegment{}

	for _, segment := range segments {
		rawSegments = append(rawSegments, metabase.RawSegment(segment))
	}

	return rawSegments
}

// ObjectStreamToPending converts ObjectStream to PendingObjectStream.
func ObjectStreamToPending(objectStream metabase.ObjectStream) metabase.PendingObjectStream {
	return metabase.PendingObjectStream{
		ProjectID:  objectStream.ProjectID,
		BucketName: objectStream.BucketName,
		ObjectKey:  objectStream.ObjectKey,
		StreamID:   objectStream.StreamID,
	}
}
