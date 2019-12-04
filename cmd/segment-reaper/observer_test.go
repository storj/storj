// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
)

func Test_observer_analyzeProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tests := []struct {
		segments                 string
		expectedNumberOfSegments byte
		hasLastSegment           bool
		skip                     bool
		brokenObject             bool
	}{
		{"11110000", 5, true, false, false}, // #0
		{"00111000", 0, true, false, true},  // #1
		// TODO add more cases
	}
	for i, tt := range tests {
		t.Run("case_"+strconv.Itoa(i), func(t *testing.T) {
			bucketObjects := make(bucketsObjects)
			singleObjectMap := make(map[storj.Path]*object)
			segments := bitmask(0)
			for i, char := range tt.segments {
				if char == '1' {
					err := segments.Set(i)
					require.NoError(t, err)
				}
			}
			singleObjectMap["test-path"] = &object{
				segments:                 segments,
				hasLastSegment:           tt.hasLastSegment,
				expectedNumberOfSegments: tt.expectedNumberOfSegments,
				skip:                     tt.skip,
			}
			bucketObjects["test-bucket"] = singleObjectMap

			observer := &observer{
				lastProjectID: testrand.UUID().String(),
				objects:       bucketObjects,
			}
			brokenObject := false
			err := observer.analyzeProject(ctx, func(ctx context.Context, projectID, segmentIndex, bucket, path string) error {
				brokenObject = true
				return nil
			})
			require.NoError(t, err)

			require.Equal(t, tt.brokenObject, brokenObject, "case %s failed", i)
		})
	}
}
