// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion_test

import (
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

func TestReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	var testCases = []struct {
		description     string
		numRequests     int
		numDeletedPaths int
		expectedFailure bool
	}{
		{"has-failure", 2, 1, true},
		{"all-deleted", 2, 2, false},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()
			requests := createRequests(tt.numRequests)
			paths, pointers, err := createDeletedItems(requests, tt.numDeletedPaths)
			require.NoError(t, err)

			report := objectdeletion.GenerateReport(ctx, logger, requests, paths, pointers)
			require.Equal(t, tt.expectedFailure, report.HasFailures())
		})
	}

}

func createDeletedItems(requests []*metabase.ObjectLocation, numDeleted int) ([]metabase.SegmentKey, []*pb.Pointer, error) {
	if numDeleted > len(requests) {
		return nil, nil, errs.New("invalid argument")
	}
	paths := make([]metabase.SegmentKey, 0, numDeleted)
	pointers := make([]*pb.Pointer, 0, numDeleted)
	for i := 0; i < numDeleted; i++ {
		segmentLocation, err := requests[i].Segment(int64(testrand.Intn(10)))
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, segmentLocation.Encode())
		pointers = append(pointers, &pb.Pointer{})
	}
	return paths, pointers, nil
}

func createRequests(numRequests int) []*metabase.ObjectLocation {
	requests := make([]*metabase.ObjectLocation, 0, numRequests)

	for i := 0; i < numRequests; i++ {
		obj := metabase.ObjectLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "test",
			ObjectKey:  metabase.ObjectKey(strconv.Itoa(i) + "test"),
		}
		requests = append(requests, &obj)
	}

	return requests
}
