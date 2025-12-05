// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"math"
	"sort"

	"storj.io/storj/satellite/metabase/rangedloop"
)

var _ rangedloop.RangeSplitter = (*RangeSplitter)(nil)

// RangeSplitter allows to iterate over segments from an in-memory source.
type RangeSplitter struct {
	Segments []rangedloop.Segment
}

var _ rangedloop.SegmentProvider = (*SegmentProvider)(nil)

// SegmentProvider allows to iterate over segments from an in-memory source.
type SegmentProvider struct {
	Segments []rangedloop.Segment

	batchSize int
}

// CreateRanges splits the segments into equal ranges.
func (m *RangeSplitter) CreateRanges(ctx context.Context, nRanges int, batchSize int) ([]rangedloop.SegmentProvider, error) {
	// The segments for a given stream must be handled by a single segment
	// provider. Split the segments into streams.
	streams := streamsFromSegments(m.Segments)

	// Break up the streams into ranges
	rangeSize := int(math.Ceil(float64(len(streams)) / float64(nRanges)))

	rangeProviders := []rangedloop.SegmentProvider{}
	for i := 0; i < nRanges; i++ {
		offset := min(i*rangeSize, len(streams))
		end := min(offset+rangeSize, len(streams))
		rangeProviders = append(rangeProviders, &SegmentProvider{
			Segments:  segmentsFromStreams(streams[offset:end]),
			batchSize: batchSize,
		})
	}

	return rangeProviders, nil
}

// Range returns range which is processed by this provider.
func (m *SegmentProvider) Range() rangedloop.UUIDRange {
	return rangedloop.UUIDRange{}
}

// Iterate allows to loop over the segments stored in the provider.
func (m *SegmentProvider) Iterate(ctx context.Context, fn func([]rangedloop.Segment) error) error {
	for offset := 0; offset < len(m.Segments); offset += m.batchSize {
		end := min(offset+m.batchSize, len(m.Segments))
		err := fn(m.Segments[offset:end])
		if err != nil {
			return err
		}
	}

	return nil
}

func streamsFromSegments(segments []rangedloop.Segment) [][]rangedloop.Segment {
	// Duplicate and sort the segments by stream ID
	segments = append([]rangedloop.Segment(nil), segments...)
	sort.Slice(segments, func(i int, j int) bool {
		idcmp := segments[i].StreamID.Compare(segments[j].StreamID)
		switch {
		case idcmp < 0:
			return true
		case idcmp > 0:
			return false
		default:
			return segments[i].Position.Less(segments[j].Position)
		}
	})
	// Break up the sorted segments into streams
	var streams [][]rangedloop.Segment
	var stream []rangedloop.Segment
	for _, segment := range segments {
		if len(stream) > 0 && stream[0].StreamID != segment.StreamID {
			// Stream ID changed; push and reset stream
			streams = append(streams, stream)
			stream = nil
		}
		stream = append(stream, segment)
	}

	// Append the last stream (will be empty if there were no segments)
	if len(stream) > 0 {
		streams = append(streams, stream)
	}
	return streams
}

func segmentsFromStreams(streams [][]rangedloop.Segment) []rangedloop.Segment {
	var segments []rangedloop.Segment
	for _, stream := range streams {
		segments = append(segments, stream...)
	}
	return segments
}
