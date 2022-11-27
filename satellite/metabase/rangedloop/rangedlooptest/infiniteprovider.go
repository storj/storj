// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var _ rangedloop.RangeSplitter = (*InfiniteSegmentProvider)(nil)
var _ rangedloop.SegmentProvider = (*InfiniteSegmentProvider)(nil)

// InfiniteSegmentProvider allow to iterate indefinitely to test service cancellation.
type InfiniteSegmentProvider struct {
}

// CreateRanges splits the segments into equal ranges.
func (m *InfiniteSegmentProvider) CreateRanges(nRanges int, batchSize int) (segmentsProviders []rangedloop.SegmentProvider, err error) {
	for i := 0; i < nRanges; i++ {
		segmentsProviders = append(segmentsProviders, &InfiniteSegmentProvider{})
	}
	return segmentsProviders, nil
}

// Iterate allows to loop over the segments stored in the provider.
func (m *InfiniteSegmentProvider) Iterate(ctx context.Context, fn func([]segmentloop.Segment) error) error {
	for {
		err := fn(make([]segmentloop.Segment, 3))
		if err != nil {
			return err
		}
	}
}
