// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
)

const (
	broken    = true
	notBroken = false

	lastSegYES = true
	lastSegNO  = false
)

func Test_observer_analyzeProject(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tests := []struct {
		segments                 string
		expectedNumberOfSegments byte
		skip                     bool
		segmentsAfter            string
		brokenObject             bool
	}{
		// known number of segments
		{"11111_l", 6, false, "11111", notBroken}, // #0
		{"00000_l", 1, false, "00000", notBroken}, // #1
		{"1111100", 6, false, "11111", broken},    // #2
		{"11011_l", 6, false, "11011", broken},    // #3
		{"11011_l", 3, false, "00011", broken},    // #4
		{"11110_l", 6, false, "11110", broken},    // #5

		// unknown number of segments
		{"11111_l", 0, false, "00000", notBroken}, // #6
		{"1111100", 0, false, "11111", broken},    // #7
		{"00111_l", 0, false, "00111", broken},    // #8
		{"10111_l", 0, false, "00111", broken},    // #9
		{"11011_l", 0, false, "00011", broken},    // #10

		// skip
		{"1101100", 0, true, "1101100", notBroken}, // #11
	}
	for i, tt := range tests {
		t.Run("case_"+strconv.Itoa(i), func(t *testing.T) {
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
				skip:                     tt.skip,
			}
			singleObjectMap["test-path"] = object
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

			segmentsAfter := bitmask(0)
			for i, char := range tt.segmentsAfter {
				if char == '1' {
					err := segmentsAfter.Set(i)
					require.NoError(t, err)
				}
			}

			if object.segments != segmentsAfter {
				t.Fatalf("segments before and after comparison faild, case %d ", i)
			}

			require.Equal(t, tt.brokenObject, brokenObject, "case %d failed", i)
		})
	}
}
