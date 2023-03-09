// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
)

func TestSplitter(t *testing.T) {
	mkseg := func(streamID byte, pos uint64) segmentloop.Segment {
		return segmentloop.Segment{
			StreamID: uuid.UUID{0: streamID},
			Position: metabase.SegmentPositionFromEncoded(pos),
		}
	}

	mkstream := func(streamID byte, numSegments int) []segmentloop.Segment {
		var stream []segmentloop.Segment
		for i := 0; i < numSegments; i++ {
			stream = append(stream, mkseg(streamID, uint64(numSegments)))
		}
		return stream
	}

	intermix := func(segments []segmentloop.Segment) []segmentloop.Segment {
		segments = append([]segmentloop.Segment(nil), segments...)
		r.Shuffle(len(segments), func(i, j int) {
			segments[i], segments[j] = segments[j], segments[i]
		})
		return segments
	}

	combine := func(streams ...[]segmentloop.Segment) []segmentloop.Segment {
		return segmentsFromStreams(streams)
	}

	stream1 := mkstream(1, 3)
	stream2 := mkstream(2, 5)
	stream3 := mkstream(3, 1)
	stream4 := mkstream(4, 2)
	stream5 := mkstream(5, 4)

	for _, tt := range []struct {
		desc         string
		segments     []segmentloop.Segment
		numRanges    int
		expectRanges [][]segmentloop.Segment
	}{
		{
			desc:      "no segments",
			segments:  nil,
			numRanges: 2,
			expectRanges: [][]segmentloop.Segment{
				{},
				{},
			},
		},
		{
			desc:      "one stream over two ranges",
			segments:  stream1,
			numRanges: 2,
			expectRanges: [][]segmentloop.Segment{
				stream1,
				{},
			},
		},
		{
			desc:      "two streams over two ranges",
			segments:  combine(stream1, stream2),
			numRanges: 2,
			expectRanges: [][]segmentloop.Segment{
				stream1,
				stream2,
			},
		},
		{
			desc:      "three streams over two ranges",
			segments:  combine(stream1, stream2, stream3),
			numRanges: 2,
			expectRanges: [][]segmentloop.Segment{
				combine(stream1, stream2),
				stream3,
			},
		},
		{
			desc:      "three streams intermixed over two ranges",
			segments:  intermix(combine(stream1, stream2, stream3)),
			numRanges: 2,
			expectRanges: [][]segmentloop.Segment{
				combine(stream1, stream2),
				stream3,
			},
		},
		{
			desc:      "five streams intermixed over three ranges",
			segments:  intermix(combine(stream1, stream2, stream3, stream4, stream5)),
			numRanges: 3,
			expectRanges: [][]segmentloop.Segment{
				combine(stream1, stream2),
				combine(stream3, stream4),
				stream5,
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			const batchSize = 3

			splitter := RangeSplitter{Segments: tt.segments}

			providers, err := splitter.CreateRanges(tt.numRanges, batchSize)
			require.NoError(t, err)

			var actualRanges [][]segmentloop.Segment
			for _, provider := range providers {
				rangeSegments := []segmentloop.Segment{}
				err := provider.Iterate(context.Background(), func(segments []segmentloop.Segment) error {
					if len(segments) > batchSize {
						return fmt.Errorf("iterated segments (%d) larger than batch size (%d)", len(segments), batchSize)
					}
					rangeSegments = append(rangeSegments, segments...)
					return nil
				})
				require.NoError(t, err)
				actualRanges = append(actualRanges, rangeSegments)
			}
			require.Equal(t, tt.expectRanges, actualRanges)
		})
	}
}
