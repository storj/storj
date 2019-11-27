// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage/teststore"
)

func TestObserver_processSegment(t *testing.T) {
	t.Run("valid objects of different projects", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			obsvr = observer{
				db:      teststore.New(),
				objects: make(bucketsObjects),
			}
			expectedNumSegments    int
			expectedInlineSegments int
			expectedRemoteSegments int
			expectedObjects        = map[string]map[storj.Path]*object{}
			objSegments            []segmentRef
			projID                 = testrand.UUID()
		)

		{ // Generate objects for testing
			numSegments := rand.Intn(10) + 1
			inline := (rand.Int() % 2) == 0
			withNumSegments := (rand.Int() % 2) == 0

			_, objSegmentsProj := createNewObjectSegments(
				t, ctx.Context, numSegments, &projID, "project1", inline, withNumSegments,
			)
			objSegments = append(objSegments, objSegmentsProj...)

			expectedNumSegments += numSegments
			if inline {
				expectedInlineSegments++
				expectedRemoteSegments += (numSegments - 1)
			} else {
				expectedRemoteSegments += numSegments
			}

			// Reset project ID to create several objects in the same project
			projID = testrand.UUID()

			var (
				bucketName = "0"
				numObjs    = rand.Intn(10) + 2
			)
			for i := 0; i < numObjs; i++ {
				numSegments = rand.Intn(10) + 1
				inline = (rand.Int() % 2) == 0
				withNumSegments = (rand.Int() % 2) == 0

				if rand.Int()%2 == 0 {
					bucketName = fmt.Sprintf("bucket-%d", i)
				}
				objPath, objSegmentsProj := createNewObjectSegments(
					t, ctx.Context, numSegments, &projID, bucketName, inline, withNumSegments,
				)
				objSegments = append(objSegments, objSegmentsProj...)

				// TODO: use findOrCreate when cluster removal is merged
				var expectedObj *object
				bucketObjects, ok := expectedObjects[bucketName]
				if !ok {
					expectedObj = &object{}
					expectedObjects[bucketName] = map[storj.Path]*object{
						objPath: expectedObj,
					}
				} else {
					expectedObj = &object{}
					bucketObjects[objPath] = expectedObj
				}

				if withNumSegments {
					expectedObj.expectedNumberOfSegments = byte(numSegments)
				}

				expectedObj.hasLastSegment = true
				expectedObj.skip = false
				// segments mask doesn't contain the last segment, hence we move 1 bit more
				expectedObj.segments = math.MaxUint64 >> (int(maxNumOfSegments) - numSegments + 1)

				expectedNumSegments += numSegments
				if inline {
					expectedInlineSegments++
					expectedRemoteSegments += (numSegments - 1)
				} else {
					expectedRemoteSegments += numSegments
				}
			}
		}

		for _, objSeg := range objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		assert.Equal(t, projID.String(), obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		// newObject if returns an inline segment is always the last
		assert.Equal(t, expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")

		if assert.Equal(t, len(expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := expectedObjects[bucket]
				if !ok {
					t.Errorf("bucket '%s' shouldn't exist in objects map", bucket)
					continue
				}

				if !assert.Equalf(t, len(expBucketObjs), len(bucketObjs), "objects per bucket (%s) number", bucket) {
					continue
				}

				for expPath, expObj := range expBucketObjs {
					if !assert.Contains(t, bucketObjs, expPath, "path in bucket objects map") {
						continue
					}

					obj := bucketObjs[expPath]
					assert.Equal(t, expObj.expectedNumberOfSegments, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
					assert.Equal(t, expObj.hasLastSegment, obj.hasLastSegment, "Object.hasLastSegment")
					assert.Equal(t, expObj.skip, obj.skip, "Object.skip")
					assert.Equal(t, expObj.segments, obj.segments, "Object.segments")
				}
			}
		}
	})

	t.Run("object without last segment", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			obsvr = observer{
				db:      teststore.New(),
				objects: make(bucketsObjects),
			}
			expectedNumSegments    int
			expectedInlineSegments int
			expectedRemoteSegments int
			expectedObjects        = map[string]map[storj.Path]*object{}
			objSegments            []segmentRef
			projID                 = testrand.UUID()
		)

		{ // Generate objects for testing
			var (
				bucketName         = "0"
				numObjs            = rand.Intn(10) + 2
				withoutLastSegment = 0
			)
			for i := 0; i < numObjs; i++ {
				var (
					numSegments     = rand.Intn(10) + 2
					inline          = (rand.Int() % 2) == 0
					withNumSegments = (rand.Int() % 2) == 0
				)

				if rand.Int()%2 == 0 {
					bucketName = fmt.Sprintf("bucket-%d", i)
				}
				objPath, objSegmentsProj := createNewObjectSegments(
					t, ctx.Context, numSegments, &projID, bucketName, inline, withNumSegments,
				)
				objSegments = append(objSegments, objSegmentsProj...)

				// TODO: use findOrCreate when cluster removal is merged
				var expectedObj *object
				bucketObjects, ok := expectedObjects[bucketName]
				if !ok {
					expectedObj = &object{}
					expectedObjects[bucketName] = map[storj.Path]*object{
						objPath: expectedObj,
					}
				} else {
					expectedObj = &object{}
					bucketObjects[objPath] = expectedObj
				}

				// segments mask doesn't contain the last segment, hence we move 1 bit more
				expectedObj.segments = math.MaxUint64 >> (int(maxNumOfSegments) - numSegments + 1)
				expectedObj.skip = false

				// random object without last segment or remove the segment of the
				// generated object if all the previous generated objects have the
				// last segment, then remove from this one
				if (rand.Int()%2) == 0 || (i == (numObjs-1) && withoutLastSegment == 0) {
					withoutLastSegment++
					expectedObj.hasLastSegment = false
					numSegments--
					objSegments = objSegments[:len(objSegments)-1]
					expectedRemoteSegments += numSegments
				} else {
					expectedObj.hasLastSegment = true

					if inline {
						expectedInlineSegments++
						expectedRemoteSegments += (numSegments - 1)
					} else {
						expectedRemoteSegments += numSegments
					}

					if withNumSegments {
						expectedObj.expectedNumberOfSegments = byte(numSegments)
					}
				}

				expectedNumSegments += numSegments
			}
		}

		for _, objSeg := range objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		assert.Equal(t, projID.String(), obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		// newObject if returns an inline segment is always the last
		assert.Equal(t, expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")

		if assert.Equal(t, len(expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := expectedObjects[bucket]
				if !ok {
					t.Errorf("bucket '%s' shouldn't exist in objects map", bucket)
					continue
				}

				if !assert.Equalf(t, len(expBucketObjs), len(bucketObjs), "objects per bucket (%s) number", bucket) {
					continue
				}

				for expPath, expObj := range expBucketObjs {
					if !assert.Contains(t, bucketObjs, expPath, "path in bucket objects map") {
						continue
					}

					obj := bucketObjs[expPath]
					assert.Equal(t, expObj.expectedNumberOfSegments, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
					assert.Equal(t, expObj.hasLastSegment, obj.hasLastSegment, "Object.hasLastSegment")
					assert.Equal(t, expObj.skip, obj.skip, "Object.skip")
					assert.Equal(t, expObj.segments, obj.segments, "Object.segments")
				}
			}
		}
	})

	t.Run("object with more than 64 segments", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			obsvr = observer{
				db:      teststore.New(),
				objects: make(bucketsObjects),
			}
			expectedNumSegments    int
			expectedInlineSegments int
			expectedRemoteSegments int
			expectedObjects        = map[string]map[storj.Path]*object{}
			objSegments            []segmentRef
			projID                 = testrand.UUID()
		)

		{ // Generate objects for testing
			var (
				bucketName            = "0"
				numObjs               = rand.Intn(10) + 2
				objWithNumSegsOverMax = false
			)
			for i := 0; i < numObjs || !objWithNumSegsOverMax; i++ {
				var (
					numSegments     = rand.Intn(100) + 1
					inline          = (rand.Int() % 2) == 0
					withNumSegments = (rand.Int() % 2) == 0
				)

				if rand.Int()%2 == 0 {
					bucketName = fmt.Sprintf("bucket-%d", i)
				}
				objPath, objSegmentsProj := createNewObjectSegments(
					t, ctx.Context, numSegments, &projID, bucketName, inline, withNumSegments,
				)
				objSegments = append(objSegments, objSegmentsProj...)

				// TODO: use findOrCreate when cluster removal is merged
				var expectedObj *object
				bucketObjects, ok := expectedObjects[bucketName]
				if !ok {
					expectedObj = &object{}
					expectedObjects[bucketName] = map[storj.Path]*object{
						objPath: expectedObj,
					}
				} else {
					expectedObj = &object{}
					bucketObjects[objPath] = expectedObj
				}

				if withNumSegments {
					expectedObj.expectedNumberOfSegments = byte(numSegments)
				}

				expectedObj.hasLastSegment = true

				if numSegments > int(maxNumOfSegments) {
					objWithNumSegsOverMax = true
					expectedObj.skip = true
				} else {
					// segments mask doesn't contain the last segment, hence we move 1 bit more
					expectedObj.segments = math.MaxUint64 >> (int(maxNumOfSegments) - numSegments + 1)
				}

				expectedNumSegments += numSegments
				if inline {
					expectedInlineSegments++
					expectedRemoteSegments += (numSegments - 1)
				} else {
					expectedRemoteSegments += numSegments
				}
			}
		}

		for _, objSeg := range objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		assert.Equal(t, projID.String(), obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		// newObject if returns an inline segment is always the last
		assert.Equal(t, expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")

		if assert.Equal(t, len(expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := expectedObjects[bucket]
				if !ok {
					t.Errorf("bucket '%s' shouldn't exist in objects map", bucket)
					continue
				}

				if !assert.Equalf(t, len(expBucketObjs), len(bucketObjs), "objects per bucket (%s) number", bucket) {
					continue
				}

				for expPath, expObj := range expBucketObjs {
					if !assert.Contains(t, bucketObjs, expPath, "path in bucket objects map") {
						continue
					}

					obj := bucketObjs[expPath]
					assert.Equal(t, expObj.expectedNumberOfSegments, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
					assert.Equal(t, expObj.hasLastSegment, obj.hasLastSegment, "Object.hasLastSegment")

					assert.Equal(t, expObj.skip, obj.skip, "Object.skip")
					if !expObj.skip {
						assert.Equal(t, expObj.segments, obj.segments, "Object.segments")
					}
				}
			}
		}
	})
}

func TestObsever_analyzeProject(t *testing.T) {
	t.Run("object without last segment", func(t *testing.T) {
		var bucketsObjs bucketsObjects
		{ // Generate an objects without last segment
			const (
				bucketName = "analyzeBucket"
				objPath    = storj.Path("analyzePath")
			)
			var segments uint64
			{
				numSegments := rand.Intn(62) + 1
				segments = math.MaxUint64 >> numSegments
			}

			bucketsObjs = bucketsObjects{
				bucketName: map[storj.Path]*object{
					objPath: {
						segments:       segments,
						hasLastSegment: false,
					},
				},
			}
		}

		var (
			buf   = &bytes.Buffer{}
			obsvr = observer{
				db:      teststore.New(),
				writer:  csv.NewWriter(buf),
				objects: bucketsObjs,
			}
		)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		err := obsvr.analyzeProject(ctx.Context)
		require.NoError(t, err)

		// TODO: Add assertions for buf content
	})

	t.Run("object with non sequenced segments", func(t *testing.T) {
		var bucketsObjs bucketsObjects
		{ // Generate an objects without last segment
			const (
				bucketName = "analyzeBucket"
				objPath    = storj.Path("analyzePath")
			)
			var segments uint64
			{ // Calculate a unaligned number of segments
				segments = rand.Uint64()
				for {
					trailingZeros := bits.TrailingZeros64(segments)
					leadingZeros := bits.LeadingZeros64(segments)

					if (trailingZeros + leadingZeros) == 64 {
						continue
					}

					ones := bits.OnesCount64(segments)
					if (trailingZeros + leadingZeros + ones) == 64 {
						continue
					}

					break
				}
			}

			bucketsObjs = bucketsObjects{
				bucketName: map[storj.Path]*object{
					objPath: {
						segments:       segments,
						hasLastSegment: true,
					},
				},
			}
		}

		var (
			buf   = &bytes.Buffer{}
			obsvr = observer{
				db:      teststore.New(),
				writer:  csv.NewWriter(buf),
				objects: bucketsObjs,
			}
		)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		err := obsvr.analyzeProject(ctx.Context)
		require.NoError(t, err)

		// TODO: Add assertions for buf content
	})

	t.Run("object with unencrypted segments with different stored number", func(t *testing.T) {
		var bucketsObjs bucketsObjects
		{ // Generate an object
			const (
				bucketName = "analyzeBucket"
				objPath    = storj.Path("analyzePath")
			)
			var (
				segments           uint64
				invalidNumSegments byte
			)
			{
				numSegments := rand.Intn(62) + 1
				segments = math.MaxUint64 >> numSegments

				for {
					numSeg := rand.Intn(65)
					if numSeg != numSegments {
						invalidNumSegments = byte(numSeg)
						break
					}
				}
			}

			bucketsObjs = bucketsObjects{
				bucketName: map[storj.Path]*object{
					objPath: {
						segments:                 segments,
						expectedNumberOfSegments: invalidNumSegments,
						hasLastSegment:           false,
					},
				},
			}
		}

		var (
			buf   = &bytes.Buffer{}
			obsvr = observer{
				db:      teststore.New(),
				writer:  csv.NewWriter(buf),
				objects: bucketsObjs,
			}
		)

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		err := obsvr.analyzeProject(ctx.Context)
		require.NoError(t, err)

		// TODO: Add assertions for buf content
	})
}

// segmentRef is an object segment reference to be used for simulating calls to
// observer.processSegment
type segmentRef struct {
	path    metainfo.ScopedPath
	pointer *pb.Pointer
}

// createNewObjectSegments creates a list of segment references which belongs to
// a same object.
//
// If inline is true the last segment will be of INLINE type.
//
// If withNumSegments is true the last segment pointer will have the
// NumberOfSegments set.
//
// It returns the object path and the list of object segment references.
func createNewObjectSegments(
	t *testing.T, ctx context.Context, numSegments int, projectID *uuid.UUID, bucketName string, inline bool, withNumSegments bool,
) (objectPath string, _ []segmentRef) {
	t.Helper()

	var objectID string
	{
		id := testrand.UUID()
		objectID = id.String()
	}

	var (
		projectIDString = projectID.String()
		references      = make([]segmentRef, 0, numSegments)
		encryptedPath   = fmt.Sprintf("%s-%s-%s", projectIDString, bucketName, objectID)
	)

	for i := 0; i < (numSegments - 1); i++ {
		raw, err := metainfo.CreatePath(ctx, *projectID, int64(i), []byte(bucketName), []byte(objectID))
		require.NoError(t, err)

		references = append(references, segmentRef{
			path: metainfo.ScopedPath{
				ProjectID:           *projectID,
				ProjectIDString:     projectIDString,
				BucketName:          bucketName,
				Segment:             fmt.Sprintf("s%d", i),
				EncryptedObjectPath: encryptedPath,
				Raw:                 raw,
			},
			pointer: &pb.Pointer{
				Type: pb.Pointer_REMOTE,
			},
		})
	}

	pointerType := pb.Pointer_REMOTE
	if inline {
		pointerType = pb.Pointer_INLINE
	}

	var pointerNumSegments int64
	if withNumSegments {
		pointerNumSegments = int64(numSegments)
	}

	metadata, err := proto.Marshal(&pb.StreamMeta{
		NumberOfSegments: pointerNumSegments,
	})
	require.NoError(t, err)

	raw, err := metainfo.CreatePath(ctx, *projectID, -1, []byte(bucketName), []byte(objectID))
	require.NoError(t, err)

	references = append(references, segmentRef{
		path: metainfo.ScopedPath{
			ProjectID:           *projectID,
			ProjectIDString:     projectIDString,
			BucketName:          bucketName,
			Segment:             "l",
			EncryptedObjectPath: encryptedPath,
			Raw:                 raw,
		},
		pointer: &pb.Pointer{
			Type:     pointerType,
			Metadata: metadata,
		},
	})

	return encryptedPath, references
}
