// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
)

func Test_observer_analyzeProject(t *testing.T) {
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

		// unknown number of segments
		{"11111_l", 0, "11111_l"}, // #8
		{"00000_l", 0, "00000_l"}, // #9
		{"10000_l", 0, "10000_l"}, // #10
		{"1111100", 0, "0000000"}, // #11
		{"00111_l", 0, "00000_l"}, // #12
		{"10111_l", 0, "10000_l"}, // #13
		{"10101_l", 0, "10000_l"}, // #14
		{"11011_l", 0, "11000_l"}, // #15

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

			require.Equalf(t, tt.segmentsAfter, segmentsAfter, "segments before and after comparison faild: want %s got %s, case %d ", tt.segmentsAfter, segmentsAfter, testNum)
		})
	}
}
