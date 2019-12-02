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
	"os"
	"testing"
	"time"

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

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestObserver_processSegment(t *testing.T) {
	t.Run("valid objects of different projects", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		obsvr := observer{
			objects: make(bucketsObjects),
		}

		testdata1 := generateTestdataObjects(t, ctx.Context, false, false)
		// Call processSegment with testadata objects of the first project
		for _, objSeg := range testdata1.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		testdata2 := generateTestdataObjects(t, ctx.Context, false, false)
		// Call processSegment with testadata objects of the second project
		for _, objSeg := range testdata2.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Inspect observer internal state to assert that it only has the state
		// related to the second project
		assert.Equal(t, testdata2.projectID.String(), obsvr.lastProjectID, "lastProjectID")
		if assert.Equal(t, len(testdata2.expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := testdata2.expectedObjects[bucket]
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

		// Assert that objserver keep track global stats of all the segments which
		// have received through processSegment calls
		assert.Equal(t,
			testdata1.expectedInlineSegments+testdata2.expectedInlineSegments,
			obsvr.inlineSegments,
			"inlineSegments",
		)
		assert.Equal(t,
			testdata1.expectedInlineSegments+testdata2.expectedInlineSegments,
			obsvr.lastInlineSegments,
			"lastInlineSegments",
		)
		assert.Equal(t,
			testdata1.expectedRemoteSegments+testdata2.expectedRemoteSegments,
			obsvr.remoteSegments,
			"remoteSegments",
		)
	})

	t.Run("object without last segment", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			testdata = generateTestdataObjects(t, ctx.Context, true, false)
			obsvr    = observer{
				objects: make(bucketsObjects),
			}
		)

		// Call processSegment with the testdata
		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assert.Equal(t, testdata.projectID.String(), obsvr.lastProjectID, "lastProjectID")
		if assert.Equal(t, len(testdata.expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := testdata.expectedObjects[bucket]
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

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("object with 65 segments", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			bucketName  = "a bucket"
			projectID   = testrand.UUID()
			numSegments = 65
			obsvr       = observer{
				objects: make(bucketsObjects),
			}
			objPath, objSegmentsRefs = createNewObjectSegments(
				t, ctx.Context, numSegments, &projectID, bucketName, false, false,
			)
		)

		for _, objSeg := range objSegmentsRefs {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assert.Equal(t, projectID.String(), obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, 1, len(obsvr.objects), "objects number")
		if assert.Contains(t, obsvr.objects, bucketName, "bucket in objects map") {
			if assert.Equal(t, 1, len(obsvr.objects[bucketName]), "objects in object map") {
				if assert.Contains(t, obsvr.objects[bucketName], objPath, "path in bucket objects map") {
					obj := obsvr.objects[bucketName][objPath]
					assert.Zero(t, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
					assert.True(t, obj.hasLastSegment, "Object.hasLastSegment")
					assert.False(t, obj.skip, "Object.skip")
				}
			}
		}

		// Assert observer global stats
		assert.Zero(t, obsvr.inlineSegments, "inlineSegments")
		assert.Zero(t, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, numSegments, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("objects with at least one has more than 64 segments", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			testdata = generateTestdataObjects(t, ctx.Context, false, true)
			obsvr    = observer{
				objects: make(bucketsObjects),
			}
		)

		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assert.Equal(t, testdata.projectID.String(), obsvr.lastProjectID, "lastProjectID")
		if assert.Equal(t, len(testdata.expectedObjects), len(obsvr.objects), "objects number") {
			for bucket, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := testdata.expectedObjects[bucket]
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

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
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
			var segments bitmask
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
			var segments bitmask
			{ // Calculate a unaligned number of segments
				unaligned := rand.Uint64()
				for {
					trailingZeros := bits.TrailingZeros64(unaligned)
					leadingZeros := bits.LeadingZeros64(unaligned)

					if (trailingZeros + leadingZeros) == 64 {
						continue
					}

					ones := bits.OnesCount64(unaligned)
					if (trailingZeros + leadingZeros + ones) == 64 {
						continue
					}

					segments = bitmask(unaligned)
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
				segments           bitmask
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
//nolint:golint
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

type testdataObjects struct {
	// expectations
	expectedNumSegments    int
	expectedInlineSegments int
	expectedRemoteSegments int
	expectedObjects        bucketsObjects

	// data used for calling processSegment
	objSegments []segmentRef
	projectID   *uuid.UUID
}

// generateTestdataObjects generate a testdataObjecst with a random number of
// segments of a random number of objects and buckets but under the same
// project.
//
// When withoutLastSegment is true, there will be objects without last segment,
// otherwise all of them will have a last segment.
//
// When withMoreThanMaxNumSegments is true, there will be objects with more
// segments than the maxNumOfSegments, otherwise all of them will have less or
// equal than it.
// nolint:golint
func generateTestdataObjects(
	t *testing.T, ctx context.Context, withoutLastSegment bool, withMoreThanMaxNumSegments bool,
) testdataObjects {
	t.Helper()

	var (
		testdata = testdataObjects{
			expectedObjects: make(bucketsObjects),
		}
		bucketName                      = "0"
		numObjs                         = rand.Intn(10) + 2
		projID                          = testrand.UUID()
		withoutLastSegmentCount         = 0
		withMoreThanMaxNumSegmentsCount = 0
		numMaxSegments                  = 10
	)

	if withMoreThanMaxNumSegments {
		numMaxSegments = 100
	}

	testdata.projectID = &projID

	for i := 0; i < numObjs; i++ {
		var (
			inline          = (rand.Int() % 2) == 0
			withNumSegments = (rand.Int() % 2) == 0
			numSegments     = rand.Intn(numMaxSegments) + 2
		)

		if numSegments > int(maxNumOfSegments) {
			withMoreThanMaxNumSegmentsCount++
		}

		// If withMoreThanMaxNumSegments is true and all the objects created in all
		// the previous iterations have less or equal than maximum number of segments
		// and this is the last iteration then force that the object crated in this
		// iteration has more segments than the maximum.
		if withMoreThanMaxNumSegments &&
			withMoreThanMaxNumSegmentsCount == 0 &&
			i == (numObjs-1) {
			numSegments += int(maxNumOfSegments)
			withMoreThanMaxNumSegmentsCount++
		}

		if rand.Int()%2 == 0 {
			bucketName = fmt.Sprintf("bucket-%d", i)
		}
		objPath, objSegmentsProj := createNewObjectSegments(
			t, ctx, numSegments, &projID, bucketName, inline, withNumSegments,
		)
		testdata.objSegments = append(testdata.objSegments, objSegmentsProj...)

		expectedObj := findOrCreate(bucketName, objPath, testdata.expectedObjects)

		// only create segments mask if the number of segments is less or equal than
		// maxNumOfSegments
		if numSegments <= int(maxNumOfSegments) {
			// segments mask doesn't contain the last segment, hence we move 1 bit more
			expectedObj.segments = math.MaxUint64 >> (int(maxNumOfSegments) - numSegments + 1)
			expectedObj.skip = false
		} else {
			expectedObj.skip = true
		}

		// If withoutLastSegment is true, then choose random objects without last
		// segment or	force to remove it from the object generated in the last
		// iteration if in any object of the previous iterations have the last
		// segment
		if withoutLastSegment &&
			((rand.Int()%2) == 0 || (i == (numObjs-1) && withoutLastSegmentCount == 0)) {
			withoutLastSegmentCount++
			expectedObj.hasLastSegment = false
			numSegments--
			testdata.objSegments = testdata.objSegments[:len(testdata.objSegments)-1]
			testdata.expectedRemoteSegments += numSegments
		} else {
			expectedObj.hasLastSegment = true

			if inline {
				testdata.expectedInlineSegments++
				testdata.expectedRemoteSegments += (numSegments - 1)
			} else {
				testdata.expectedRemoteSegments += numSegments
			}

			if withNumSegments {
				expectedObj.expectedNumberOfSegments = byte(numSegments)
			}
		}

		testdata.expectedNumSegments += numSegments
	}

	// Shuffle the segments for not having a object segments serial order
	rand.Shuffle(len(testdata.objSegments), func(i, j int) {
		testdata.objSegments[i], testdata.objSegments[j] = testdata.objSegments[j], testdata.objSegments[i]
	})

	return testdata
}
