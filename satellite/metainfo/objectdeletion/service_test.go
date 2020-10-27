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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/metainfo/objectdeletion"
)

func TestService_Delete_SingleObject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// mock the object that we want to delete
	item := &metabase.ObjectLocation{
		ProjectID:  testrand.UUID(),
		BucketName: "bucketname",
		ObjectKey:  "encrypted",
	}

	objectNotFound := &metabase.ObjectLocation{
		ProjectID:  testrand.UUID(),
		BucketName: "object-not-found",
		ObjectKey:  "object-missing",
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
		MaxConcurrentRequests:    200,
	}

	var testCases = []struct {
		segmentType             string
		isValidObject           bool
		largestSegmentIdx       int64
		numPiecesPerSegment     int32
		expectedPointersDeleted int
		expectedKeyDeleted      int
		expectedPiecesToDelete  int32
	}{
		{"single-segment", true, 0, 3, 1, 1, 3},
		{"multi-segment", true, 5, 2, 6, 6, 12},
		{"inline-segment", true, 0, 0, 1, 1, 0},
		{"mixed-segment", true, 5, 3, 6, 6, 15},
		{"zombie-segment", true, 5, 2, 5, 5, 10},
		{"single-segment", false, 0, 3, 1, 1, 0},
	}

	for _, tt := range testCases {
		tt := tt // quiet linting
		t.Run(tt.segmentType, func(t *testing.T) {
			pointerDBMock, err := newPointerDB([]*metabase.ObjectLocation{item}, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, false)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedKeys, err := service.DeletePointers(ctx, []*metabase.ObjectLocation{item})
			if !tt.isValidObject {
				pointers, deletedKeys, err = service.DeletePointers(ctx, []*metabase.ObjectLocation{objectNotFound})
			}
			require.NoError(t, err)
			require.Len(t, pointers, tt.expectedPointersDeleted)
			require.Len(t, deletedKeys, tt.expectedKeyDeleted)

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
	item := &metabase.ObjectLocation{
		ProjectID:  testrand.UUID(),
		BucketName: "bucketname",
		ObjectKey:  "encrypted",
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
		MaxConcurrentRequests:    200,
	}

	var testCases = []struct {
		segmentType            string
		largestSegmentIdx      int64
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
			reqs := []*metabase.ObjectLocation{item}
			pointerDBMock, err := newPointerDB(reqs, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, true)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			pointers, deletedKeys, err := service.DeletePointers(ctx, reqs)
			require.Error(t, err)
			require.Len(t, pointers, 0)
			require.Len(t, deletedKeys, 0)

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

	items := make([]*metabase.ObjectLocation, 0, 100)

	for i := 0; i < 10; i++ {
		item := &metabase.ObjectLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "bucketname",
			ObjectKey:  metabase.ObjectKey("encrypted" + strconv.Itoa(i)),
		}
		items = append(items, item)
	}

	config := objectdeletion.Config{
		MaxObjectsPerRequest:     100,
		ZombieSegmentsPerRequest: 3,
		MaxConcurrentRequests:    200,
	}

	var testCases = []struct {
		segmentType             string
		largestSegmentIdx       int64
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

			pointers, deletedKeys, err := service.DeletePointers(ctx, items)
			require.NoError(t, err)
			require.Len(t, pointers, tt.expectedPointersDeleted)
			require.Len(t, deletedKeys, tt.expectedPointersDeleted)

			piecesToDeleteByNodes := objectdeletion.GroupPiecesByNodeID(pointers)
			totalPiecesToDelete := 0
			for _, pieces := range piecesToDeleteByNodes {
				totalPiecesToDelete += len(pieces)
			}
			require.Equal(t, tt.expectedPiecesToDelete, int32(totalPiecesToDelete))
		})
	}
}

func calcExpectedPieces(segmentType string, numRequests int, batchSize int, largestSegmentIdx int64, numPiecesPerSegment int) int {
	numSegments := int(largestSegmentIdx) + 1

	totalPieces := numRequests * numSegments * numPiecesPerSegment

	switch segmentType {
	case "mixed-segment":
		return totalPieces - numPiecesPerSegment
	case "zombie-segment":
		return numRequests * int(largestSegmentIdx) * numPiecesPerSegment
	default:
		return totalPieces
	}

}

func TestService_Delete_Batch(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var testCases = []struct {
		description         string
		segmentType         string
		numRequests         int
		batchSize           int
		largestSegmentIdx   int64
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
				MaxConcurrentRequests:    tt.batchSize * 2,
			}

			requests := createRequests(tt.numRequests)
			expectedPiecesToDelete := calcExpectedPieces(tt.segmentType, tt.numRequests, tt.batchSize, tt.largestSegmentIdx, int(tt.numPiecesPerSegment))

			pointerDBMock, err := newPointerDB(requests, tt.segmentType, tt.largestSegmentIdx, tt.numPiecesPerSegment, false)
			require.NoError(t, err)

			service, err := objectdeletion.NewService(zaptest.NewLogger(t), pointerDBMock, config)
			require.NoError(t, err)

			results, err := service.Delete(ctx, requests...)
			require.NoError(t, err)
			pointers := []*pb.Pointer{}
			for _, r := range results {
				p := r.DeletedPointers()
				pointers = append(pointers, p...)
				require.False(t, r.HasFailures())
			}

			piecesToDelete := objectdeletion.GroupPiecesByNodeID(pointers)
			require.Equal(t, expectedPiecesToDelete, len(piecesToDelete))
		})
	}

}

type pointerDBMock struct {
	pointers map[string]*pb.Pointer
	hasError bool
}

func newPointerDB(objects []*metabase.ObjectLocation, segmentType string, numSegments int64, numPiecesPerSegment int32, hasError bool) (*pointerDBMock, error) {
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

	keys := []metabase.SegmentKey{}
	for _, obj := range objects {
		newKeys, err := createKeys(obj, numSegments)
		if err != nil {
			return nil, err
		}
		keys = append(keys, newKeys...)
	}

	pointers, err = createMockPointers(option.lastSegment, option.firstSegment, option.inlineSegment, keys, numPiecesPerSegment, numSegments)
	if err != nil {
		return nil, err
	}

	pointerDB := &pointerDBMock{
		pointers: make(map[string]*pb.Pointer, len(keys)),
		hasError: hasError,
	}
	for i, p := range keys {
		pointerDB.pointers[string(p)] = pointers[i]
	}

	return pointerDB, nil
}

func (db *pointerDBMock) GetItems(ctx context.Context, keys []metabase.SegmentKey) ([]*pb.Pointer, error) {
	if db.hasError {
		return nil, errs.New("pointerDB failure")
	}
	pointers := make([]*pb.Pointer, len(keys))
	for i, p := range keys {
		pointers[i] = db.pointers[string(p)]
	}
	return pointers, nil
}

func (db *pointerDBMock) UnsynchronizedGetDel(ctx context.Context, keys []metabase.SegmentKey) ([]metabase.SegmentKey, []*pb.Pointer, error) {
	pointers := make([]*pb.Pointer, len(keys))
	for i, p := range keys {
		pointers[i] = db.pointers[string(p)]
	}

	rand.Shuffle(len(pointers), func(i, j int) {
		pointers[i], pointers[j] = pointers[j], pointers[i]
		keys[i], keys[j] = keys[j], keys[i]
	})

	return keys, pointers, nil
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

func newLastSegmentPointer(pointerType pb.Pointer_DataType, numSegments int64, numPiecesPerSegment int32) (*pb.Pointer, error) {
	pointer := newPointer(pointerType, numPiecesPerSegment)
	meta := &pb.StreamMeta{
		NumberOfSegments: numSegments,
	}
	metaInBytes, err := pb.Marshal(meta)
	if err != nil {
		return nil, err
	}
	pointer.Metadata = metaInBytes
	return pointer, nil
}

func createMockPointers(hasLastSegment bool, hasFirstSegment bool, hasInlineSegments bool, keys []metabase.SegmentKey, numPiecesPerSegment int32, numSegments int64) ([]*pb.Pointer, error) {
	pointers := make([]*pb.Pointer, 0, len(keys))

	isInlineAdded := false
	for _, p := range keys {
		segmentLocation, err := metabase.ParseSegmentKey(p)
		if err != nil {
			return nil, err
		}

		if segmentLocation.IsLast() {
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
		if !hasFirstSegment && segmentLocation.IsFirst() {
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

func createKeys(object *metabase.ObjectLocation, largestSegmentIdx int64) ([]metabase.SegmentKey, error) {
	keys := []metabase.SegmentKey{}
	for i := int64(0); i <= largestSegmentIdx; i++ {
		segmentIdx := i
		if segmentIdx == largestSegmentIdx {
			segmentIdx = metabase.LastSegmentIndex
		}

		segment, err := object.Segment(segmentIdx)
		if err != nil {
			return nil, err
		}

		keys = append(keys, segment.Encode())
	}
	return keys, nil
}
