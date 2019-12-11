// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite/metainfo"
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestObserver_processSegment(t *testing.T) {
	t.Run("valid objects of different projects", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		obsvr := &observer{
			objects: make(bucketsObjects),
		}

		testdata1 := generateTestdataObjects(ctx.Context, t, false, false)
		// Call processSegment with testadata objects of the first project
		for _, objSeg := range testdata1.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		testdata2 := generateTestdataObjects(ctx.Context, t, false, false)
		// Call processSegment with testadata objects of the second project
		for _, objSeg := range testdata2.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Inspect observer internal state to assert that it only has the state
		// related to the second project
		assertObserver(t, obsvr, testdata2)

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
			testdata = generateTestdataObjects(ctx.Context, t, true, false)
			obsvr    = &observer{
				objects: make(bucketsObjects),
			}
		)

		// Call processSegment with the testdata
		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assertObserver(t, obsvr, testdata)

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("object with 65 segments without expected number of segments", func(t *testing.T) {
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
				ctx.Context, t, numSegments, &projectID, bucketName, false, false,
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

	t.Run("object with 65 segments with expected number of segments", func(t *testing.T) {
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
				ctx.Context, t, numSegments, &projectID, bucketName, false, true,
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
					assert.Equal(t, numSegments, int(obj.expectedNumberOfSegments), "Object.expectedNumSegments")
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

	t.Run("objects with at least one has more than 65 segments", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			testdata = generateTestdataObjects(ctx.Context, t, false, true)
			obsvr    = &observer{
				objects: make(bucketsObjects),
			}
		)

		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assertObserver(t, obsvr, testdata)

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
	})
}

func TestObserver_processSegment_from_to(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var (
		notSet = time.Time{}
		now    = time.Now()
	)

	tests := []struct {
		from              time.Time
		to                time.Time
		pointerCreateDate time.Time
		skipObject        bool
	}{
		// not skipped
		{notSet, notSet, now, false},
		{notSet, now, now, false},
		{now, now, now, false},
		{now, notSet, now, false},
		{now.Add(-time.Minute), now.Add(time.Minute), now, false},
		{now.Add(-time.Minute), now.Add(time.Minute), now.Add(time.Minute), false},
		{now.Add(-time.Minute), now.Add(time.Minute), now.Add(-time.Minute), false},

		// skipped
		{notSet, now, now.Add(time.Second), true},
		{now, notSet, now.Add(-time.Second), true},
		{now.Add(-time.Minute), now.Add(time.Minute), now.Add(time.Hour), true},
		{now.Add(-time.Minute), now.Add(time.Minute), now.Add(-time.Hour), true},
	}
	for _, tt := range tests {
		var from *time.Time
		var to *time.Time
		if tt.from != notSet {
			from = &tt.from
		}
		if tt.to != notSet {
			to = &tt.to
		}
		observer := &observer{
			objects: make(bucketsObjects),
			from:    from,
			to:      to,
		}
		path := metainfo.ScopedPath{
			ProjectID:           testrand.UUID(),
			Segment:             "l",
			BucketName:          "bucket1",
			EncryptedObjectPath: "path1",
		}
		pointer := &pb.Pointer{
			CreationDate: tt.pointerCreateDate,
		}
		err := observer.processSegment(ctx, path, pointer)
		require.NoError(t, err)

		objectsMap, ok := observer.objects["bucket1"]
		require.True(t, ok)

		object, ok := objectsMap["path1"]
		require.True(t, ok)

		require.Equal(t, tt.skipObject, object.skip)
	}
}

func TestObserver_analyzeProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	allSegments64 := string(bytes.ReplaceAll(make([]byte, 64), []byte{0}, []byte{'1'}))

	tests := []struct {
		segments                 string
		expectedNumberOfSegments byte
		segmentsAfter            string
	}{
		// this visualize which segments will be NOT selected as zombie segments

		// known number of segments
		{"11111_l", 6, "11111_l"}, // #0
		{"00000_l", 1, "00000_l"}, // #1
		{"1111100", 6, "0000000"}, // #2
		{"11011_l", 6, "00000_0"}, // #3
		{"11011_l", 3, "11000_l"}, // #4
		{"11110_l", 6, "00000_0"}, // #5
		{"00011_l", 4, "00000_0"}, // #6
		{"10011_l", 4, "00000_0"}, // #7
		{"11011_l", 4, "00000_0"}, // #8

		// unknown number of segments
		{"11111_l", 0, "11111_l"}, // #9
		{"00000_l", 0, "00000_l"}, // #10
		{"10000_l", 0, "10000_l"}, // #11
		{"1111100", 0, "0000000"}, // #12
		{"00111_l", 0, "00000_l"}, // #13
		{"10111_l", 0, "10000_l"}, // #14
		{"10101_l", 0, "10000_l"}, // #15
		{"11011_l", 0, "11000_l"}, // #16

		// special cases
		{allSegments64 + "_l", 65, allSegments64 + "_l"}, // #16
	}
	for testNum, tt := range tests {
		testNum := testNum
		tt := tt
		t.Run("case_"+strconv.Itoa(testNum), func(t *testing.T) {
			bucketObjects := make(bucketsObjects)
			singleObjectMap := make(map[storj.Path]*object)
			segments := bitmask(0)
			for i, char := range tt.segments {
				if char == '_' {
					break
				}
				if char == '1' {
					err := segments.Set(i)
					require.NoError(t, err)
				}
			}

			object := &object{
				segments:                 segments,
				hasLastSegment:           strings.HasSuffix(tt.segments, "_l"),
				expectedNumberOfSegments: tt.expectedNumberOfSegments,
			}
			singleObjectMap["test-path"] = object
			bucketObjects["test-bucket"] = singleObjectMap

			observer := &observer{
				objects:       bucketObjects,
				lastProjectID: testrand.UUID().String(),
				zombieBuffer:  make([]int, 0, maxNumOfSegments),
			}
			err := observer.findZombieSegments(object)
			require.NoError(t, err)
			indexes := observer.zombieBuffer

			segmentsAfter := tt.segments
			for _, segmentIndex := range indexes {
				if segmentIndex == lastSegment {
					segmentsAfter = segmentsAfter[:len(segmentsAfter)-1] + "0"
				} else {
					segmentsAfter = segmentsAfter[:segmentIndex] + "0" + segmentsAfter[segmentIndex+1:]
				}
			}

			require.Equalf(t, tt.segmentsAfter, segmentsAfter, "segments before and after comparison failed: want %s got %s, case %d ", tt.segmentsAfter, segmentsAfter, testNum)
		})
	}
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
	ctx context.Context, t *testing.T, numSegments int, projectID *uuid.UUID, bucketName string, inline bool, withNumSegments bool,
) (objectPath string, _ []segmentRef) {
	t.Helper()

	var (
		objectID        = testrand.UUID().String()
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
func generateTestdataObjects(
	ctx context.Context, t *testing.T, withoutLastSegment bool, withMoreThanMaxNumSegments bool,
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
		numMaxGeneratedSegments         = 10
	)

	if withMoreThanMaxNumSegments {
		numMaxGeneratedSegments = 100
	}

	testdata.projectID = &projID

	for i := 0; i < numObjs; i++ {
		var (
			inline          = (rand.Int() % 2) == 0
			withNumSegments = (rand.Int() % 2) == 0
			numSegments     = rand.Intn(numMaxGeneratedSegments) + 2
		)

		if numSegments > (maxNumOfSegments + 1) {
			withMoreThanMaxNumSegmentsCount++
		}

		// If withMoreThanMaxNumSegments is true and all the objects created in all
		// the previous iterations have less or equal than maximum number of segments
		// and this is the last iteration then force that the object crated in this
		// iteration has more segments than the maximum.
		if withMoreThanMaxNumSegments &&
			withMoreThanMaxNumSegmentsCount == 0 &&
			i == (numObjs-1) {
			numSegments += maxNumOfSegments
			withMoreThanMaxNumSegmentsCount++
		}

		if rand.Int()%2 == 0 {
			bucketName = fmt.Sprintf("bucket-%d", i)
		}
		objPath, objSegmentsProj := createNewObjectSegments(
			ctx, t, numSegments, &projID, bucketName, inline, withNumSegments,
		)
		testdata.objSegments = append(testdata.objSegments, objSegmentsProj...)

		expectedObj := findOrCreate(bucketName, objPath, testdata.expectedObjects)

		// only create segments mask if the number of segments is less or equal than
		// maxNumOfSegments + 1 because the last segment isn't in the bitmask
		if numSegments <= (maxNumOfSegments + 1) {
			// segments mask doesn't contain the last segment, hence we move 1 bit more
			expectedObj.segments = math.MaxUint64 >> (maxNumOfSegments - numSegments + 1)
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

// assertObserver assert the observer values with the testdata ones.
func assertObserver(t *testing.T, obsvr *observer, testdata testdataObjects) {
	t.Helper()

	assert.Equal(t, testdata.projectID.String(), obsvr.lastProjectID, "lastProjectID")
	if assert.Equal(t, len(testdata.expectedObjects), len(obsvr.objects), "objects number") {
		for bucket, bucketObjs := range obsvr.objects {
			expBucketObjs, ok := testdata.expectedObjects[bucket]
			if !assert.Truef(t, ok, "bucket '%s' shouldn't exist in objects map", bucket) {
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
}

func TestObserver_processSegment_switch_project(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// need bolddb to have DB with concurrent access support
	db, err := metainfo.NewStore(zaptest.NewLogger(t), "bolt://"+ctx.File("pointers.db"))
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	buffer := new(bytes.Buffer)
	writer := csv.NewWriter(buffer)
	defer ctx.Check(writer.Error)

	observer, err := newObserver(db, writer, nil, nil)
	require.NoError(t, err)

	// project IDs are pregenerated to avoid issues with iteration order
	now := time.Now()
	project1 := "7176d6a8-3a83-7ae7-e084-5fdbb1a17ac1"
	project2 := "890dd9f9-6461-eb1b-c3d1-73af7252b9a4"

	// zombie segment for project 1
	_, err = makeSegment(ctx, db, storj.JoinPaths(project1, "s0", "bucket1", "path1"), now)
	require.NoError(t, err)

	// zombie segment for project 2
	_, err = makeSegment(ctx, db, storj.JoinPaths(project2, "s0", "bucket1", "path1"), now)
	require.NoError(t, err)

	err = observer.detectZombieSegments(ctx)
	require.NoError(t, err)

	writer.Flush()

	result := buffer.String()
	for _, projectID := range []string{project1, project2} {
		encodedPath := base64.StdEncoding.EncodeToString([]byte("path1"))
		pathPrefix := strings.Join([]string{projectID, "s0", "bucket1", encodedPath, now.UTC().Format(time.RFC3339Nano)}, ",")
		assert.Containsf(t, result, pathPrefix, "entry for projectID %s not found: %s", projectID)
	}
}
