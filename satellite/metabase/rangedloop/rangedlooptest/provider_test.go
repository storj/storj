// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
)

func TestSplitter(t *testing.T) {
	ctx := testcontext.New(t)

	mkseg := func(streamID byte, pos uint64) rangedloop.Segment {
		return rangedloop.Segment{
			StreamID: uuid.UUID{0: streamID},
			Position: metabase.SegmentPositionFromEncoded(pos),
		}
	}

	mkstream := func(streamID byte, numSegments int) []rangedloop.Segment {
		var stream []rangedloop.Segment
		for i := 0; i < numSegments; i++ {
			stream = append(stream, mkseg(streamID, uint64(numSegments)))
		}
		return stream
	}

	intermix := func(segments []rangedloop.Segment) []rangedloop.Segment {
		segments = append([]rangedloop.Segment(nil), segments...)
		r.Shuffle(len(segments), func(i, j int) {
			segments[i], segments[j] = segments[j], segments[i]
		})
		return segments
	}

	combine := func(streams ...[]rangedloop.Segment) []rangedloop.Segment {
		return segmentsFromStreams(streams)
	}

	stream1 := mkstream(1, 3)
	stream2 := mkstream(2, 5)
	stream3 := mkstream(3, 1)
	stream4 := mkstream(4, 2)
	stream5 := mkstream(5, 4)

	for _, tt := range []struct {
		desc         string
		segments     []rangedloop.Segment
		numRanges    int
		expectRanges [][]rangedloop.Segment
	}{
		{
			desc:      "no segments",
			segments:  nil,
			numRanges: 2,
			expectRanges: [][]rangedloop.Segment{
				{},
				{},
			},
		},
		{
			desc:      "one stream over two ranges",
			segments:  stream1,
			numRanges: 2,
			expectRanges: [][]rangedloop.Segment{
				stream1,
				{},
			},
		},
		{
			desc:      "two streams over two ranges",
			segments:  combine(stream1, stream2),
			numRanges: 2,
			expectRanges: [][]rangedloop.Segment{
				stream1,
				stream2,
			},
		},
		{
			desc:      "three streams over two ranges",
			segments:  combine(stream1, stream2, stream3),
			numRanges: 2,
			expectRanges: [][]rangedloop.Segment{
				combine(stream1, stream2),
				stream3,
			},
		},
		{
			desc:      "three streams intermixed over two ranges",
			segments:  intermix(combine(stream1, stream2, stream3)),
			numRanges: 2,
			expectRanges: [][]rangedloop.Segment{
				combine(stream1, stream2),
				stream3,
			},
		},
		{
			desc:      "five streams intermixed over three ranges",
			segments:  intermix(combine(stream1, stream2, stream3, stream4, stream5)),
			numRanges: 3,
			expectRanges: [][]rangedloop.Segment{
				combine(stream1, stream2),
				combine(stream3, stream4),
				stream5,
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			const batchSize = 3

			splitter := RangeSplitter{Segments: tt.segments}

			providers, err := splitter.CreateRanges(ctx, tt.numRanges, batchSize)
			require.NoError(t, err)

			var actualRanges [][]rangedloop.Segment
			for _, provider := range providers {
				rangeSegments := []rangedloop.Segment{}
				err := provider.Iterate(t.Context(), func(segments []rangedloop.Segment) error {
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
