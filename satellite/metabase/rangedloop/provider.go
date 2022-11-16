// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"

	"storj.io/storj/satellite/metabase/segmentloop"
)

// Provider gives a way to get non-overlapping ranges of segments concurrently.
// It usually abstracts over a database.
// It is a subcomponent of the ranged segment loop.
type Provider interface {
	CreateRanges(nRanges int, batchSize int) ([]RangeProvider, error)
}

// RangeProvider iterates through a range of segments.
type RangeProvider interface {
	Iterate(ctx context.Context, fn func([]segmentloop.Segment) error) error
}
