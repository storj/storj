// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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
			deletedSegmentPaths, err := createDeletedSegmentPaths(requests, tt.numDeletedPaths)
			require.NoError(t, err)
			report := objectdeletion.GenerateReport(ctx, logger, requests, deletedSegmentPaths)
			require.Equal(t, tt.expectedFailure, report.HasFailures())
		})
	}

}

func createDeletedSegmentPaths(requests []*objectdeletion.ObjectIdentifier, numDeleted int) ([][]byte, error) {
	if numDeleted > len(requests) {
		return nil, errs.New("invalid argument")
	}
	deletedSegmentPaths := make([][]byte, 0, numDeleted)
	for i := 0; i < numDeleted; i++ {
		path, err := requests[i].SegmentPath(int64(testrand.Intn(10)))
		if err != nil {
			return nil, err
		}
		deletedSegmentPaths = append(deletedSegmentPaths, path)
	}
	return deletedSegmentPaths, nil
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
