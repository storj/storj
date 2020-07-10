// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/objectdeletion"
)

func TestService_Delete_SingleObject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// mock the object that we want to delete
	item := &objectdeletion.ObjectIdentifier{
		ProjectID:     testrand.UUID(),
		Bucket:        []byte("bucketname"),
		EncryptedPath: []byte("encrypted"),
	}

	objectNotFound := &objectdeletion.ObjectIdentifier{
		ProjectID:     testrand.UUID(),
		Bucket:        []byte("object-not-found"),
		EncryptedPath: []byte("object-missing"),
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
	}

	var testCases = []struct {
		segmentType             string
		isValidObject           bool
		largestSegmentIdx       int
		numPiecesPerSegment     int32
		expectedPointersDeleted int
		expectedPathDeleted     int
		expectedPiecesToDelete  int32
	}{
		{"single-segment", true, 0, 3, 1, 1, 3},
		{"multi-segment", true, 5, 2, 6, 6, 12},
		{"inline-segment", true, 0, 0, 1, 1, 0},
		{"mixed-segment", true, 5, 3, 6, 6, 15},
		{"zombie-segment", true, 5, 2, 5, 5, 10},
		{"single-segment", false, 0, 3, 0, 1, 0},
	}

	for _, tt := range testCases {
		tt := tt // quiet linting
		t.Run(tt.segmentType, func(t *testing.T) {
			pointerDBMock, err := newPointerDB([]*objectdeletion.ObjectIdentifier{item}, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, false)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedPaths, err := service.DeletePointers(ctx, []*objectdeletion.ObjectIdentifier{item})
			if !tt.isValidObject {
				pointers, deletedPaths, err = service.DeletePointers(ctx, []*objectdeletion.ObjectIdentifier{objectNotFound})
			}
			require.NoError(t, err)
			require.Len(t, pointers, tt.expectedPointersDeleted)
			require.Len(t, deletedPaths, tt.expectedPathDeleted)

			piecesToDeleteByNodes := objectdeletion.GroupPiecesByNodeID(pointers)

			totalPiecesToDelete := 0
			for _, pieces := range piecesToDeleteByNodes {
				totalPiecesToDelete += len(pieces)
			}
			require.Equal(t, tt.expectedPiecesToDelete, int32(totalPiecesToDelete))
		})
	}
}

func TestService_Delete_SingleObject_Failure(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// mock the object that we want to delete
	item := &objectdeletion.ObjectIdentifier{
		ProjectID:     testrand.UUID(),
		Bucket:        []byte("bucketname"),
		EncryptedPath: []byte("encrypted"),
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
	}

	var testCases = []struct {
		segmentType            string
		largestSegmentIdx      int
		numPiecesPerSegment    int32
		expectedPiecesToDelete int32
	}{
		{"single-segment", 0, 1, 0},
		{"mixed-segment", 5, 3, 0},
		{"zombie-segment", 5, 2, 0},
	}

	for _, tt := range testCases {
		tt := tt // quiet linting
		t.Run(tt.segmentType, func(t *testing.T) {
			reqs := []*objectdeletion.ObjectIdentifier{item}
			pointerDBMock, err := newPointerDB(reqs, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, true)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedPaths, err := service.DeletePointers(ctx, reqs)
			require.Error(t, err)
			require.Len(t, pointers, 0)
			require.Len(t, deletedPaths, 0)

			piecesToDeleteByNodes := objectdeletion.GroupPiecesByNodeID(pointers)

			totalPiecesToDelete := 0
			for _, pieces := range piecesToDeleteByNodes {
				totalPiecesToDelete += len(pieces)
			}
			require.Equal(t, tt.expectedPiecesToDelete, int32(totalPiecesToDelete))
		})
	}
}

func TestService_Delete_MultipleObject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	items := make([]*objectdeletion.ObjectIdentifier, 0, 100)

	for i := 0; i < 10; i++ {
		item := &objectdeletion.ObjectIdentifier{
			ProjectID:     testrand.UUID(),
			Bucket:        []byte("bucketname"),
			EncryptedPath: []byte("encrypted" + strconv.Itoa(i)),
		}
		items = append(items, item)
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
	}

	var testCases = []struct {
		segmentType             string
		largestSegmentIdx       int
		numPiecesPerSegment     int32
		expectedPointersDeleted int
		expectedPiecesToDelete  int32
	}{
		{"single-segment", 0, 3, 10, 30},
		{"multi-segment", 5, 2, 60, 120},
		{"inline-segment", 0, 0, 10, 0},
		{"mixed-segment", 5, 3, 60, 177},
		{"zombie-segment", 5, 2, 50, 100},
	}

	for _, tt := range testCases {
		tt := tt // quiet linting
		t.Run(tt.segmentType, func(t *testing.T) {
			pointerDBMock, err := newPointerDB(items, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, false)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedPaths, err := service.DeletePointers(ctx, items)
			require.NoError(t, err)
			require.Len(t, pointers, tt.expectedPointersDeleted)
			require.Len(t, deletedPaths, tt.expectedPointersDeleted)

			piecesToDeleteByNodes := objectdeletion.GroupPiecesByNodeID(pointers)
			totalPiecesToDelete := 0
			for _, pieces := range piecesToDeleteByNodes {
				totalPiecesToDelete += len(pieces)
			}
			require.Equal(t, tt.expectedPiecesToDelete, int32(totalPiecesToDelete))
		})
	}
}

func calcExpectedPieces(segmentType string, numRequests int, batchSize int, largestSegmentIdx int, numPiecesPerSegment int) int {
	numSegments := largestSegmentIdx + 1

	totalPieces := numRequests * numSegments * numPiecesPerSegment

	switch segmentType {
	case "mixed-segment":
		return totalPieces - numPiecesPerSegment
	case "zombie-segment":
		return numRequests * largestSegmentIdx * numPiecesPerSegment
	default:
		return totalPieces
	}

}

func TestService_Delete_Batch(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var testCases = []struct {
		description         string
		segmentType         string
		numRequests         int
		batchSize           int
		largestSegmentIdx   int
		numPiecesPerSegment int32
	}{
		{"single-request", "single-segment", 1, 1, 0, 3},
		{"single-request", "multi-segment", 1, 1, 5, 2},
		{"single-request", "inline-segment", 1, 1, 0, 0},
		{"single-request", "mixed-segment", 1, 1, 5, 3},
		{"single-request", "zombie-segment", 1, 1, 5, 2},

		{"multi-request", "single-segment", 10, 2, 0, 3},
		{"multi-request", "multi-segment", 10, 2, 5, 2},
		{"multi-request", "inline-segment", 10, 2, 0, 0},
		{"multi-request", "mixed-segment", 10, 3, 5, 3},
		{"multi-request", "zombie-segment", 10, 2, 5, 2},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			config := objectdeletion.Config{
				MaxObjectsPerRequest:     tt.batchSize,
				ZombieSegmentsPerRequest: 3,
			}

			requests := createRequests(tt.numRequests)
			expectedPiecesToDelete := calcExpectedPieces(tt.segmentType, tt.numRequests, tt.batchSize, tt.largestSegmentIdx, int(tt.numPiecesPerSegment))

			pointerDBMock, err := newPointerDB(requests, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, false)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedPaths, err := service.Delete(ctx, requests...)
			require.NoError(t, err)

			report := objectdeletion.GenerateReport(ctx, logger, requests, deletedPaths)
			require.False(t, report.HasFailures())

			piecesToDeleted := objectdeletion.GroupPiecesByNodeID(pointers)

			require.Equal(t, expectedPiecesToDelete, len(piecesToDeleted))
		})
	}

}

const (
	lastSegmentIdx  = -1
	firstSegmentIdx = 0
)

type pointerDBMock struct {
	pointers map[string]*pb.Pointer
	hasError bool
}

func newPointerDB(objects []*objectdeletion.ObjectIdentifier, segmentType string, numSegments int, numPiecesPerSegment int32, hasError bool) (*pointerDBMock, error) {
	var (
		pointers []*pb.Pointer
		err      error
	)

	segmentMap := map[string]struct{ lastSegment, firstSegment, inlineSegment bool }{
		"single-segment": {true, false, false},
		"multi-segment":  {true, true, false},
		"inline-segment": {true, false, true},
		"mixed-segment":  {true, true, true},
		"zombie-segment": {false, true, false},
	}

	option, ok := segmentMap[segmentType]
	if !ok {
		return nil, errs.New("unsupported segment type")
	}

	paths := [][]byte{}
	for _, obj := range objects {
		paths = append(paths, createPaths(obj, numSegments)...)
	}

	pointers, err = createMockPointers(option.lastSegment, option.firstSegment, option.inlineSegment, paths, numPiecesPerSegment, numSegments)
	if err != nil {
		return nil, err
	}

	pointerDB := &pointerDBMock{
		pointers: make(map[string]*pb.Pointer, len(paths)),
		hasError: hasError,
	}
	for i, p := range paths {
		pointerDB.pointers[string(p)] = pointers[i]
	}

	return pointerDB, nil
}

func (db *pointerDBMock) GetItems(ctx context.Context, paths [][]byte) ([]*pb.Pointer, error) {
	if db.hasError {
		return nil, errs.New("pointerDB failure")
	}
	pointers := make([]*pb.Pointer, len(paths))
	for i, p := range paths {
		pointers[i] = db.pointers[string(p)]
	}
	return pointers, nil
}

func (db *pointerDBMock) UnsynchronizedGetDel(ctx context.Context, paths [][]byte) ([][]byte, []*pb.Pointer, error) {
	pointers := make([]*pb.Pointer, len(paths))
	for i, p := range paths {
		pointers[i] = db.pointers[string(p)]
	}

	rand.Shuffle(len(pointers), func(i, j int) {
		pointers[i], pointers[j] = pointers[j], pointers[i]
		paths[i], paths[j] = paths[j], paths[i]
	})

	return paths, pointers, nil
}

func newPointer(pointerType pb.Pointer_DataType, numPiecesPerSegment int32) *pb.Pointer {
	pointer := &pb.Pointer{
		Type: pointerType,
	}
	if pointerType == pb.Pointer_REMOTE {
		remotePieces := make([]*pb.RemotePiece, 0, numPiecesPerSegment)
		for i := int32(0); i < numPiecesPerSegment; i++ {
			remotePieces = append(remotePieces, &pb.RemotePiece{
				PieceNum: i,
				NodeId:   testrand.NodeID(),
			})
		}
		pointer.Remote = &pb.RemoteSegment{
			RootPieceId:  testrand.PieceID(),
			RemotePieces: remotePieces,
		}
	}
	return pointer
}

func newLastSegmentPointer(pointerType pb.Pointer_DataType, numSegments int, numPiecesPerSegment int32) (*pb.Pointer, error) {
	pointer := newPointer(pointerType, numPiecesPerSegment)
	meta := &pb.StreamMeta{
		NumberOfSegments: int64(numSegments),
	}
	metaInBytes, err := pb.Marshal(meta)
	if err != nil {
		return nil, err
	}
	pointer.Metadata = metaInBytes
	return pointer, nil
}

func createMockPointers(hasLastSegment bool, hasFirstSegment bool, hasInlineSegments bool, paths [][]byte, numPiecesPerSegment int32, numSegments int) ([]*pb.Pointer, error) {
	pointers := make([]*pb.Pointer, 0, len(paths))

	isInlineAdded := false
	for _, p := range paths {
		_, segment, err := objectdeletion.ParseSegmentPath(p)
		if err != nil {
			return nil, err
		}

		if segment == lastSegmentIdx {
			if !hasLastSegment {
				pointers = append(pointers, nil)
			} else {
				lastSegmentPointer, err := newLastSegmentPointer(pb.Pointer_REMOTE, numSegments, numPiecesPerSegment)
				if err != nil {
					return nil, err
				}
				pointers = append(pointers, lastSegmentPointer)
			}
			continue
		}
		if !hasFirstSegment && segment == firstSegmentIdx {
			pointers = append(pointers, nil)
			continue
		}
		if hasInlineSegments && !isInlineAdded {
			pointers = append(pointers, newPointer(pb.Pointer_INLINE, 0))
			isInlineAdded = true
			continue
		}
		pointers = append(pointers, newPointer(pb.Pointer_REMOTE, numPiecesPerSegment))
	}

	return pointers, nil
}

func createPaths(object *objectdeletion.ObjectIdentifier, largestSegmentIdx int) [][]byte {
	paths := [][]byte{}
	for i := 0; i <= largestSegmentIdx; i++ {
		segmentIdx := i
		if segmentIdx == largestSegmentIdx {
			segmentIdx = lastSegmentIdx
		}
		paths = append(paths, createPath(object.ProjectID, object.Bucket, segmentIdx, object.EncryptedPath))
	}
	return paths
}

func createPath(projectID uuid.UUID, bucket []byte, segmentIdx int, encryptedPath []byte) []byte {
	segment := "l"
	if segmentIdx > lastSegmentIdx {
		segment = "s" + strconv.Itoa(segmentIdx)
	}

	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, segment)
	entries = append(entries, string(bucket))
	entries = append(entries, string(encryptedPath))
	return []byte(storj.JoinPaths(entries...))
}
