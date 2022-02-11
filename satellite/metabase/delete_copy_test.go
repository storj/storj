package metabase_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteCopy(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		for _, numberOfSegments := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%d segments", numberOfSegments), func(t *testing.T) {
				t.Run("delete copy", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjStream, byte(numberOfSegments))

					copyObj, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					// check that copy went OK
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObj),
						},
						Segments: append(segmentsToRaw(originalSegments), copySegments...),
						Copies: []metabase.RawCopy{
							{
								StreamID:         copyObj.StreamID,
								AncestorStreamID: originalObj.StreamID,
							},
						},
					}.Normalize().Check(ctx, t, db)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        copyObj.Version,
						ObjectLocation: copyObj.Location(),
					})
					require.NoError(t, err)

					// Verify that we are back at the original single object
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
						},
						Segments: segmentsToRaw(originalSegments),
					}.Normalize().Check(ctx, t, db)
				})

				t.Run("delete one of two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					numberOfSegments := 0
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, _ := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        copyObject1.Version,
						ObjectLocation: copyObject1.Location(),
					})
					require.NoError(t, err)

					// Verify that only one of the copies is deleted
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObject2),
						},
						Segments: append(segmentsToRaw(originalSegments), copySegments2...),
						Copies: []metabase.RawCopy{
							{
								StreamID:         copyObject2.StreamID,
								AncestorStreamID: originalObj.StreamID,
							},
						},
					}.Normalize().Check(ctx, t, db)
				})

				t.Run("delete original", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					numberOfSegments := 0
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, _ := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        originalObj.Version,
						ObjectLocation: originalObj.Location(),
					})
					require.NoError(t, err)

					// verify that the copy is left
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject),
						},
						Segments: copySegments,
					}.Normalize().Check(ctx, t, db)
				})

				t.Run("delete original and leave two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					numberOfSegments := 0
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, _ := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, copySegments1 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        originalObj.Version,
						ObjectLocation: originalObj.Location(),
					})
					require.NoError(t, err)

					// verify that two functioning copies are left and the original object is gone
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject1),
							metabase.RawObject(copyObject2),
						},
						Segments: append(copySegments1, copySegments2...),
						Copies: []metabase.RawCopy{
							{
								StreamID:         maxUUID(copyObject1.StreamID, copyObject2.StreamID),
								AncestorStreamID: minUUID(copyObject1.StreamID, copyObject2.StreamID),
							},
						},
					}.Normalize().Check(ctx, t, db)
				})
			})
		}
	})
}

func segmentsToRaw(segments []metabase.Segment) []metabase.RawSegment {
	rawSegments := []metabase.RawSegment{}

	for _, segment := range segments {
		rawSegments = append(rawSegments, metabase.RawSegment(segment))
	}

	return rawSegments
}

func maxUUID(uuid1 uuid.UUID, uuid2 uuid.UUID) uuid.UUID {
	for i := 0; i < 16; i++ {
		diff := int(uuid1[i]) - int(uuid2[i])

		if diff > 0 {
			return uuid1
		}

		if diff < 0 {
			return uuid2
		}
	}

	// both are equal
	return uuid1
}

func minUUID(uuid1 uuid.UUID, uuid2 uuid.UUID) uuid.UUID {
	for i := 0; i < 16; i++ {
		diff := int(uuid1[i]) - int(uuid2[i])

		if diff < 0 {
			return uuid1
		}

		if diff > 0 {
			return uuid2
		}
	}

	// both are equal
	return uuid1
}
