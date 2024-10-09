// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"storj.io/common/storj"
)

// PlacementsMetrics tracks metrics related to object and segments by placement.
//
// storj.PlacmentConstraints are the indexes of the slice.
type PlacementsMetrics []Metrics

// Reset resets all the metrics to zero.
func (metrics *PlacementsMetrics) Reset() {
	// Reuse the same allocated slice.
	(*metrics) = (*metrics)[:0]
}

// Aggregate aggregates the given metrics into the receiver.
func (metrics *PlacementsMetrics) Aggregate(partial PlacementsMetrics) {
	mlen := len(*metrics)
	// Resize the metrics slice if it has less registered placements than partial.
	if len(partial) > mlen {
		*metrics = append(*metrics, partial[mlen:]...)
	}

	// Adjust the iterations to aggregate partials, except the placements that weren't accounted
	// before.
	if mlen > len(partial) {
		mlen = len(partial)
	}

	for i := 0; i < mlen; i++ {
		(*metrics)[i].Aggregate(partial[i])
	}
}

// Read reads the metrics for all the placements and calls cb for each placement and its metrics.
func (metrics *PlacementsMetrics) Read(cb func(_ storj.PlacementConstraint, _ Metrics)) {
	for i, pm := range *metrics {
		cb(storj.PlacementConstraint(i), pm)
	}
}

// Metrics tracks metrics related to objects and segments.
type Metrics struct {
	// RemoteObjects is the count of objects with at least one remote segment.
	RemoteObjects int64

	// InlineObjects is the count of objects with only inline segments.
	InlineObjects int64

	// TotalInlineBytes is the amount of bytes across all inline segments.
	TotalInlineBytes int64

	// TotalRemoteBytes is the amount of bytes across all remote segments.
	TotalRemoteBytes int64

	// TotalInlineSegments is the count of inline segments across all objects.
	TotalInlineSegments int64

	// TotalRemoteSegments is the count of remote segments across all objects.
	TotalRemoteSegments int64

	// TotalSegmentsWithExpiresAt is the count of segments that will expire automatically.
	TotalSegmentsWithExpiresAt int64
}

// Aggregate aggregates the partial metrics into the receiver.
func (pm *Metrics) Aggregate(partial Metrics) {
	pm.RemoteObjects += partial.RemoteObjects
	pm.InlineObjects += partial.InlineObjects
	pm.TotalInlineBytes += partial.TotalInlineBytes
	pm.TotalRemoteBytes += partial.TotalRemoteBytes
	pm.TotalInlineSegments += partial.TotalInlineSegments
	pm.TotalRemoteSegments += partial.TotalRemoteSegments
	pm.TotalSegmentsWithExpiresAt += partial.TotalSegmentsWithExpiresAt
}
