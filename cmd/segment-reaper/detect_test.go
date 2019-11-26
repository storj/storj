// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
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
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage/teststore"
)

func TestObserver(t *testing.T) {
	t.Run("processSegment", func(t *testing.T) {
		var (
			obsvr = Observer{
				db:      teststore.New(),
				objects: make(ObjectsMap),
			}
			expectedNumSegments    int
			expectedInlineSegments int
			expectedRemoteSegments int
			expectedObjects        = map[string]map[storj.Path]*Object{}
			objSegments            []objectSegmentRef
		)

		projID, err := uuid.New()
		require.NoError(t, err)
		{ // Generate objects for testing
			numSegments := rand.Intn(10) + 1
			inline := (rand.Int() % 2) == 0
			withNumSegments := (rand.Int() % 2) == 0

			_, objSegmentsProj := createNewObjectSegments(t, numSegments, projID, "project1", inline, withNumSegments)
			objSegments = append(objSegments, objSegmentsProj...)

			expectedNumSegments += numSegments
			if inline {
				expectedInlineSegments++
				expectedRemoteSegments += (numSegments - 1)
			} else {
				expectedRemoteSegments += numSegments
			}

			// Reset project ID to create several objects in the same project
			projID, err = uuid.New()
			require.NoError(t, err)

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
				objPath, objSegmentsProj := createNewObjectSegments(t, numSegments, projID, bucketName, inline, withNumSegments)
				objSegments = append(objSegments, objSegmentsProj...)

				// TODO: use findOrCreate when cluster removal is merged
				var expectedObj *Object
				bucketObjects, ok := expectedObjects[bucketName]
				if !ok {
					expectedObj = &Object{}
					expectedObjects[bucketName] = map[storj.Path]*Object{
						storj.Path(objPath): expectedObj,
					}
				} else {
					expectedObj, ok = bucketObjects[storj.Path(objPath)]
					if !ok {
						expectedObj = &Object{}
						bucketObjects[storj.Path(objPath)] = expectedObj
					}
				}

				if withNumSegments {
					expectedObj.expectedNumberOfSegments = byte(numSegments)
				}

				expectedObj.hasLastSegment = true
				expectedObj.skip = false

				expectedNumSegments += numSegments
				if inline {
					expectedInlineSegments++
					expectedRemoteSegments += (numSegments - 1)
				} else {
					expectedRemoteSegments += numSegments
				}
			}
		}

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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
			for cluster, bucketObjs := range obsvr.objects {
				expBucketObjs, ok := expectedObjects[cluster.bucket]
				if !ok {
					t.Errorf("bucket '%s' shouldn't exist in objects map", cluster.bucket)
					continue
				}

				if !assert.Equalf(t, len(expBucketObjs), len(bucketObjs), "objects per bucket (%s) number", cluster.bucket) {
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

					// TODO: WIP#orange-v3-3243 Check segments field
				}
			}
		}
	})

	t.Run("analyzeProject", func(t *testing.T) {
		t.Run("object without last segment", func(t *testing.T) {
			var objectsMap ObjectsMap
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

				projID, err := uuid.New()
				require.NoError(t, err)
				objectsMap = ObjectsMap{
					Cluster{
						projectID: projID.String(),
						bucket:    bucketName,
					}: map[storj.Path]*Object{
						objPath: {
							segments:       segments,
							hasLastSegment: false,
						},
					},
				}
			}

			var (
				buf   = &bytes.Buffer{}
				obsvr = Observer{
					db:      teststore.New(),
					writer:  csv.NewWriter(buf),
					objects: objectsMap,
				}
			)

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			err := analyzeProject(ctx.Context, obsvr.db, obsvr.objects, obsvr.writer)
			require.NoError(t, err)

			// TODO: Add assertions for buf content
		})

		t.Run("object with non sequenced segments", func(t *testing.T) {
			var objectsMap ObjectsMap
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

				projID, err := uuid.New()
				require.NoError(t, err)
				objectsMap = ObjectsMap{
					Cluster{
						projectID: projID.String(),
						bucket:    bucketName,
					}: map[storj.Path]*Object{
						objPath: {
							segments:       segments,
							hasLastSegment: true,
						},
					},
				}
			}

			var (
				buf   = &bytes.Buffer{}
				obsvr = Observer{
					db:      teststore.New(),
					writer:  csv.NewWriter(buf),
					objects: objectsMap,
				}
			)

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			err := analyzeProject(ctx.Context, obsvr.db, obsvr.objects, obsvr.writer)
			require.NoError(t, err)

			// TODO: Add assertions for buf content
		})

		t.Run("object with unencrypted segments with different stored number", func(t *testing.T) {
			var objectsMap ObjectsMap
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

				projID, err := uuid.New()
				require.NoError(t, err)
				objectsMap = ObjectsMap{
					Cluster{
						projectID: projID.String(),
						bucket:    bucketName,
					}: map[storj.Path]*Object{
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
				obsvr = Observer{
					db:      teststore.New(),
					writer:  csv.NewWriter(buf),
					objects: objectsMap,
				}
			)

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			err := analyzeProject(ctx.Context, obsvr.db, obsvr.objects, obsvr.writer)
			require.NoError(t, err)

			// TODO: Add assertions for buf content
		})
	})
}

// objectSegmentRef is an object segment reference to be used for simulating
// calls observer.processSegment
type objectSegmentRef struct {
	path    metainfo.ScopedPath
	pointer *pb.Pointer
}

// crateNewObjectSegments creates a list of segment references which belongs to
// a same object.
//
// If inline is true the last segment will be of INLINE type.
//
// If withNumSegments is true the last segment pointer will have 3he
// NumberOfSegments set.
//
// It returns the object path and the list of object segment references.
func createNewObjectSegments(t *testing.T, numSegments int, projectID *uuid.UUID, bucketName string, inline bool, withNumSegments bool) (objectPath string, _ []objectSegmentRef) {
	t.Helper()

	var objectID string
	{
		id, err := uuid.New()
		require.NoError(t, err)
		objectID = id.String()
	}

	var (
		projectIDString = projectID.String()
		references      = make([]objectSegmentRef, 0, numSegments)
		encryptedPath   = fmt.Sprintf("%s-%s-%s", projectIDString, bucketName, objectID)
	)

	for i := 0; i < (numSegments - 1); i++ {
		references = append(references, objectSegmentRef{
			path: metainfo.ScopedPath{
				ProjectID:           *projectID,
				ProjectIDString:     projectIDString,
				BucketName:          bucketName,
				Segment:             fmt.Sprintf("s%d", i),
				EncryptedObjectPath: encryptedPath,
				Raw:                 fmt.Sprintf("%s/%s/%s/s%d", projectIDString, bucketName, objectID, i),
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

	references = append(references, objectSegmentRef{
		path: metainfo.ScopedPath{
			ProjectID:           *projectID,
			ProjectIDString:     projectIDString,
			BucketName:          bucketName,
			Segment:             "l",
			EncryptedObjectPath: encryptedPath,
			Raw:                 fmt.Sprintf("%s/%s/%s/l", projectIDString, bucketName, objectID),
		},
		pointer: &pb.Pointer{
			Type:     pointerType,
			Metadata: metadata,
		},
	})

	return encryptedPath, references
}
