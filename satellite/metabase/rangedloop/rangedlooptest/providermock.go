// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"math"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// RangeSplitterMock allows to iterate over segments from an in-memory source.
type RangeSplitterMock struct {
	Segments []segmentloop.Segment
}

// SegmentProviderMock allows to iterate over segments from an in-memory source.
type SegmentProviderMock struct {
	Segments []segmentloop.Segment

	batchSize int
}

// CreateRanges splits the segments into equal ranges.
func (m *RangeSplitterMock) CreateRanges(nRanges int, batchSize int) ([]rangedloop.SegmentProvider, error) {
	rangeSize := int(math.Ceil(float64(len(m.Segments)) / float64(nRanges)))

	rangeProviders := []rangedloop.SegmentProvider{}
	for i := 0; i < nRanges; i++ {
		offset := min(i*rangeSize, len(m.Segments))
		end := min(offset+rangeSize, len(m.Segments))

		segments := m.Segments[offset:end]

		rangeProviders = append(rangeProviders, &SegmentProviderMock{
			Segments:  segments,
			batchSize: batchSize,
		})
	}

	return rangeProviders, nil
}

// Iterate allows to loop over the segments stored in the provider.
func (m *SegmentProviderMock) Iterate(ctx context.Context, fn func([]segmentloop.Segment) error) error {
	for offset := 0; offset < len(m.Segments); offset += m.batchSize {
		end := min(offset+m.batchSize, len(m.Segments))
		err := fn(m.Segments[offset:end])
		if err != nil {
			return err
		}
	}

	return nil
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
