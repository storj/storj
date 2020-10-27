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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
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

		obsvr := &observer{objects: make(bucketsObjects)}

		testdata1 := generateTestdataObjects(ctx, t, false)
		// Call processSegment with testadata objects of the first project
		for _, objSeg := range testdata1.objSegments {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		testdata2 := generateTestdataObjects(ctx, t, false)
		// Call processSegment with testadata objects of the second project
		for _, objSeg := range testdata2.objSegments {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		// Inspect observer internal state to assert that it only has the state
		// related to the second project
		assertObserver(t, obsvr, testdata2)

		// Assert that objserver keep track global stats of all the segments which
		// have received through processSegment calls
		assert.Equal(t, testdata1.expectedInlineSegments+testdata2.expectedInlineSegments,
			obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata1.expectedInlineSegments+testdata2.expectedInlineSegments,
			obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata1.expectedRemoteSegments+testdata2.expectedRemoteSegments,
			obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("object without last segment", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var testdata = generateTestdataObjects(ctx, t, true)
		var obsvr = &observer{objects: make(bucketsObjects)}

		// Call processSegment with the testdata
		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assertObserver(t, obsvr, testdata)

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("objects in the same project with a random number segments", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			testdata = generateTestdataObjects(ctx, t, false)
			obsvr    = &observer{
				objects: make(bucketsObjects),
			}
		)

		for _, objSeg := range testdata.objSegments {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assertObserver(t, obsvr, testdata)

		// Assert observer global stats
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, testdata.expectedInlineSegments, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, testdata.expectedRemoteSegments, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("objects where one has segments before from", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			to                         = time.Now().Add(time.Hour)
			from                       = to.Add(-2 * time.Hour)
			diffFromTo                 = to.Sub(from)
			bucketName                 = "test-bucket"
			projectID                  = testrand.UUID()
			numSegmentsObjOutDateRange = rand.Intn(50) + 15
			numSegmentsBeforeDate      = rand.Intn(numSegmentsObjOutDateRange-1) + 1
			obsvr                      = observer{
				objects: make(bucketsObjects),
				from:    &from,
				to:      &to,
			}
		)

		pathObjOutDateRange, objSegmentsRefs := createNewObjectSegments(
			ctx, t, numSegmentsObjOutDateRange, &projectID, bucketName, true, false,
		)

		for i := 0; i < numSegmentsObjOutDateRange; i++ {
			if i < numSegmentsBeforeDate {
				// Assign a creation date before the from
				decrement := -time.Duration(rand.Int63n(math.MaxInt64-1) + 1)
				creationDate := from.Add(decrement)
				objSegmentsRefs[i].CreationDate = creationDate
				continue
			}

			// Assign a creation date between from and to (both included)
			increment := time.Duration(rand.Int63n(int64(diffFromTo) + 1))
			objSegmentsRefs[i].CreationDate = from.Add(increment)
		}

		numSegmentsObjInDateRange := rand.Intn(50) + 15
		var pathObjInDateRange metabase.ObjectKey
		{ // Object with all the segments with creation date between the from/to range
			var otherObjSegments []*metainfo.Segment
			pathObjInDateRange, otherObjSegments = createNewObjectSegments(
				ctx, t, numSegmentsObjInDateRange, &projectID, bucketName, true, false,
			)

			objSegmentsRefs = append(objSegmentsRefs, otherObjSegments...)
		}

		totalSegments := len(objSegmentsRefs)
		rand.Shuffle(totalSegments, func(i, j int) {
			objSegmentsRefs[i], objSegmentsRefs[j] = objSegmentsRefs[j], objSegmentsRefs[i]
		})

		for _, objSeg := range objSegmentsRefs {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assert.Equal(t, projectID, obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, 1, len(obsvr.objects), "objects number")
		require.Contains(t, obsvr.objects, bucketName, "bucket in objects map")
		require.Equal(t, 2, len(obsvr.objects[bucketName]), "objects in object map")
		require.Contains(t, obsvr.objects[bucketName], pathObjOutDateRange, "path in bucket objects map")
		obj := obsvr.objects[bucketName][pathObjOutDateRange]
		assert.Zero(t, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
		assert.True(t, obj.hasLastSegment, "Object.hasLastSegment")
		assert.True(t, obj.skip, "Object.skip")

		require.Contains(t, obsvr.objects[bucketName], pathObjInDateRange, "path in bucket objects map")
		obj = obsvr.objects[bucketName][pathObjInDateRange]
		assert.Zero(t, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
		assert.True(t, obj.hasLastSegment, "Object.hasLastSegment")
		assert.False(t, obj.skip, "Object.skip")

		// Assert observer global stats
		assert.Equal(t, 2, obsvr.inlineSegments, "inlineSegments")
		assert.Equal(t, 2, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, totalSegments-2, obsvr.remoteSegments, "remoteSegments")
	})

	t.Run("objects where one has segments after to", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			to                         = time.Now().Add(time.Hour)
			from                       = to.Add(-2 * time.Hour)
			diffFromTo                 = to.Sub(from)
			bucketName                 = "test-bucket"
			projectID                  = testrand.UUID()
			numSegmentsObjOutDateRange = rand.Intn(50) + 15
			numSegmentsBeforeDate      = rand.Intn(numSegmentsObjOutDateRange-1) + 1
			obsvr                      = observer{
				objects: make(bucketsObjects),
				from:    &from,
				to:      &to,
			}
		)

		pathObjOutDateRange, objSegmentsRefs := createNewObjectSegments(
			ctx, t, numSegmentsObjOutDateRange, &projectID, bucketName, false, true,
		)

		for i := 0; i < numSegmentsObjOutDateRange; i++ {
			if i < numSegmentsBeforeDate {
				// Assign a creation date after the to
				increment := time.Duration(rand.Int63n(math.MaxInt64-1) + 1)
				creationDate := to.Add(increment)
				objSegmentsRefs[i].CreationDate = creationDate
				continue
			}

			// Assign a creation date between from and to (both included)
			increment := time.Duration(rand.Int63n(int64(diffFromTo) + 1))
			objSegmentsRefs[i].CreationDate = from.Add(increment)
		}

		numSegmentsObjInDateRange := rand.Intn(50) + 15
		var pathObjInDateRange metabase.ObjectKey
		{ // Object with all the segments with creation date between the from/to range
			var otherObjSegments []*metainfo.Segment
			pathObjInDateRange, otherObjSegments = createNewObjectSegments(
				ctx, t, numSegmentsObjInDateRange, &projectID, bucketName, false, true,
			)

			objSegmentsRefs = append(objSegmentsRefs, otherObjSegments...)
		}

		totalSegments := len(objSegmentsRefs)
		rand.Shuffle(totalSegments, func(i, j int) {
			objSegmentsRefs[i], objSegmentsRefs[j] = objSegmentsRefs[j], objSegmentsRefs[i]
		})

		for _, objSeg := range objSegmentsRefs {
			err := obsvr.processSegment(ctx, objSeg)
			require.NoError(t, err)
		}

		// Assert observer internal state
		assert.Equal(t, projectID, obsvr.lastProjectID, "lastProjectID")
		assert.Equal(t, 1, len(obsvr.objects), "objects number")
		require.Contains(t, obsvr.objects, bucketName, "bucket in objects map")
		require.Equal(t, 2, len(obsvr.objects[bucketName]), "objects in object map")
		require.Contains(t, obsvr.objects[bucketName], pathObjOutDateRange, "path in bucket objects map")
		obj := obsvr.objects[bucketName][pathObjOutDateRange]
		assert.Equal(t, numSegmentsObjOutDateRange, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
		assert.True(t, obj.hasLastSegment, "Object.hasLastSegment")
		assert.True(t, obj.skip, "Object.skip")

		require.Contains(t, obsvr.objects[bucketName], pathObjInDateRange, "path in bucket objects map")
		obj = obsvr.objects[bucketName][pathObjInDateRange]
		assert.Equal(t, numSegmentsObjInDateRange, obj.expectedNumberOfSegments, "Object.expectedNumSegments")
		assert.True(t, obj.hasLastSegment, "Object.hasLastSegment")
		assert.False(t, obj.skip, "Object.skip")

		// Assert observer global stats
		assert.Zero(t, obsvr.inlineSegments, "inlineSegments")
		assert.Zero(t, obsvr.lastInlineSegments, "lastInlineSegments")
		assert.Equal(t, totalSegments, obsvr.remoteSegments, "remoteSegments")
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
		objSeg := metainfo.Segment{
			Location: metabase.SegmentLocation{
				ProjectID:  testrand.UUID(),
				BucketName: "bucket1",
				Index:      metabase.LastSegmentIndex,
				ObjectKey:  metabase.ObjectKey("path1"),
			},
			CreationDate: tt.pointerCreateDate,
		}
		err := observer.processSegment(ctx, &objSeg)
		require.NoError(t, err)

		objectsMap, ok := observer.objects["bucket1"]
		require.True(t, ok)

		object, ok := objectsMap["path1"]
		require.True(t, ok)

		require.Equal(t, tt.skipObject, object.skip)
	}
}

func TestObserver_processSegment_switch_project(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db := teststore.New()

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

func TestObserver_processSegment_single_project(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	type object struct {
		bucket           string
		segments         []int
		numberOfSegments int
		expected         string
	}

	project1 := testrand.UUID().String()
	tests := []struct {
		objects []object
	}{
		// expected = `object.expectedNumberOfSegments`_`object.segments`_`object.hasLastSegment`
		{
			objects: []object{},
		},
		{
			objects: []object{
				{bucket: "b1", segments: []int{lastSegment}, numberOfSegments: 0, expected: "0_000_l"},
				{bucket: "b1", segments: []int{lastSegment}, numberOfSegments: 1, expected: "1_000_l"},
				{bucket: "b2", segments: []int{0}, numberOfSegments: 0, expected: "0_100_0"},
				{bucket: "b1", segments: []int{0}, numberOfSegments: 5, expected: "0_100_0"},
				{bucket: "b3", segments: []int{0, 1, 2, lastSegment}, numberOfSegments: 4, expected: "4_111_l"},
				{bucket: "b1", segments: []int{0, 1, 2}, numberOfSegments: 0, expected: "0_111_0"},
				{bucket: "b5", segments: []int{2, lastSegment}, numberOfSegments: 1, expected: "1_001_l"},
				{bucket: "b1", segments: []int{2}, numberOfSegments: 1, expected: "0_001_0"},
				{bucket: "b1", segments: []int{0, lastSegment}, numberOfSegments: 3, expected: "3_100_l"},
			},
		},
	}

	for i, tt := range tests {
		i := i
		tt := tt
		t.Run("#"+strconv.Itoa(i), func(t *testing.T) {
			// need boltdb to have DB with concurrent access support
			db := teststore.New()

			for i, ttObject := range tt.objects {
				for _, segment := range ttObject.segments {
					streamMeta := &pb.StreamMeta{}

					segmentIndex := "s" + strconv.Itoa(segment)
					if segment == lastSegment {
						segmentIndex = "l"
						streamMeta.NumberOfSegments = int64(ttObject.numberOfSegments)
					}
					path := storj.JoinPaths(project1, segmentIndex, ttObject.bucket, "path"+strconv.Itoa(i))
					metadata, err := pb.Marshal(streamMeta)
					require.NoError(t, err)

					pointerBytes, err := pb.Marshal(&pb.Pointer{
						Metadata: metadata,
					})
					require.NoError(t, err)
					err = db.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
					require.NoError(t, err)
				}
			}

			observer := &observer{
				db:      db,
				objects: make(bucketsObjects),
				writer:  csv.NewWriter(new(bytes.Buffer)),
			}
			err := observer.detectZombieSegments(ctx)
			require.NoError(t, err)

			for i, ttObject := range tt.objects {
				objectsMap, ok := observer.objects[ttObject.bucket]
				require.True(t, ok)

				object, ok := objectsMap[metabase.ObjectKey("path"+strconv.Itoa(i))]
				require.True(t, ok)

				expectedParts := strings.Split(ttObject.expected, "_")
				expectedNumberOfSegments, err := strconv.Atoi(expectedParts[0])
				require.NoError(t, err)
				assert.Equal(t, expectedNumberOfSegments, object.expectedNumberOfSegments)

				expectedSegments := bitArray{}
				for i, char := range expectedParts[1] {
					if char == '_' {
						break
					}
					if char == '1' {
						err := expectedSegments.Set(i)
						require.NoError(t, err)
					}
				}
				assert.Equal(t, expectedSegments, object.segments)

				expectedLastSegment := expectedParts[2] == "l"
				assert.Equal(t, expectedLastSegment, object.hasLastSegment)
			}
		})
	}
}

func TestObserver_findZombieSegments(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	allSegments64 := string(bytes.ReplaceAll(make([]byte, 64), []byte{0}, []byte{'1'}))

	tests := []struct {
		segments                 string
		expectedNumberOfSegments int
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
			singleObjectMap := make(map[metabase.ObjectKey]*object)
			segments := bitArray{}
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
				lastProjectID: testrand.UUID(),
				zombieBuffer:  make([]int, 0),
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
) (objectKey metabase.ObjectKey, _ []*metainfo.Segment) {
	t.Helper()

	var (
		objectID   = metabase.ObjectKey(testrand.UUID().String())
		references = make([]*metainfo.Segment, 0, numSegments)
	)

	for i := 0; i < (numSegments - 1); i++ {
		references = append(references, &metainfo.Segment{
			Location: metabase.SegmentLocation{
				ProjectID:  *projectID,
				BucketName: bucketName,
				Index:      int64(i),
				ObjectKey:  objectID,
			},
			CreationDate: time.Now(),
		})
	}

	var pointerNumSegments int
	if withNumSegments {
		pointerNumSegments = numSegments
	}

	references = append(references, &metainfo.Segment{
		Location: metabase.SegmentLocation{
			ProjectID:  *projectID,
			BucketName: bucketName,
			Index:      metabase.LastSegmentIndex,
			ObjectKey:  objectID,
		},
		Inline:                   inline,
		MetadataNumberOfSegments: pointerNumSegments,
		CreationDate:             time.Now(),
	})

	return objectID, references
}

type testdataObjects struct {
	// expectations
	expectedNumSegments    int
	expectedInlineSegments int
	expectedRemoteSegments int
	expectedObjects        bucketsObjects

	// data used for calling processSegment
	objSegments []*metainfo.Segment
	projectID   *uuid.UUID
}

// generateTestdataObjects generate a testdataObjecst with a random number of
// segments of a random number of objects and buckets but under the same
// project.
//
// When withoutLastSegment is true, there will be objects without last segment,
// otherwise all of them will have a last segment.
func generateTestdataObjects(
	ctx context.Context, t *testing.T, withoutLastSegment bool) testdataObjects {
	t.Helper()

	var (
		testdata = testdataObjects{
			expectedObjects: make(bucketsObjects),
		}
		bucketName              = "0"
		numObjs                 = rand.Intn(10) + 2
		projID                  = testrand.UUID()
		withoutLastSegmentCount = 0
	)

	testdata.projectID = &projID

	for i := 0; i < numObjs; i++ {
		var (
			inline          = (rand.Int() % 2) == 0
			withNumSegments = (rand.Int() % 2) == 0
			numSegments     = rand.Intn(1000) + 2
		)

		if rand.Int()%2 == 0 {
			bucketName = fmt.Sprintf("bucket-%d", i)
		}
		objPath, objSegmentsProj := createNewObjectSegments(
			ctx, t, numSegments, &projID, bucketName, inline, withNumSegments,
		)
		testdata.objSegments = append(testdata.objSegments, objSegmentsProj...)

		expectedObj := findOrCreate(bucketName, objPath, testdata.expectedObjects)

		// segments mask doesn't contain the last segment, hence numSegments-1
		b := make([]byte, ((numSegments-1)+8-1)/8)
		for x := 0; x < numSegments-1; x++ {
			bitIndex, byteIndex := x%8, x/8
			b[byteIndex] |= byte(1) << bitIndex
		}
		expectedObj.segments = bitArray(b)

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
				expectedObj.expectedNumberOfSegments = numSegments
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

	assert.Equal(t, *testdata.projectID, obsvr.lastProjectID, "lastProjectID")
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
