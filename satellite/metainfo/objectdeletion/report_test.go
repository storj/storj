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

func createDeletedItems(requests []*objectdeletion.ObjectIdentifier, numDeleted int) ([]metabase.SegmentKey, []*pb.Pointer, error) {
	if numDeleted > len(requests) {
		return nil, nil, errs.New("invalid argument")
	}
	paths := make([]metabase.SegmentKey, 0, numDeleted)
	pointers := make([]*pb.Pointer, 0, numDeleted)
	for i := 0; i < numDeleted; i++ {
		path, err := requests[i].SegmentPath(int64(testrand.Intn(10)))
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, path)
		pointers = append(pointers, &pb.Pointer{})
	}
	return paths, pointers, nil
}

func createRequests(numRequests int) []*objectdeletion.ObjectIdentifier {
	requests := make([]*objectdeletion.ObjectIdentifier, 0, numRequests)

	for i := 0; i < numRequests; i++ {
		obj := objectdeletion.ObjectIdentifier{
			ProjectID:     testrand.UUID(),
			Bucket:        []byte("test"),
			EncryptedPath: []byte(strconv.Itoa(i) + "test"),
		}
		requests = append(requests, &obj)
	}

	return requests
}
