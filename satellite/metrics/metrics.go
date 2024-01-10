// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

// Metrics represents the metrics that are tracked by this package.
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

// Reset resets the invidual metrics back to zero.
func (metrics *Metrics) Reset() {
	*metrics = Metrics{}
}

// Aggregate aggregates the given metrics into the receiver.
func (metrics *Metrics) Aggregate(partial Metrics) {
	metrics.RemoteObjects += partial.RemoteObjects
	metrics.InlineObjects += partial.InlineObjects
	metrics.TotalInlineBytes += partial.TotalInlineBytes
	metrics.TotalRemoteBytes += partial.TotalRemoteBytes
	metrics.TotalInlineSegments += partial.TotalInlineSegments
	metrics.TotalRemoteSegments += partial.TotalRemoteSegments
	metrics.TotalSegmentsWithExpiresAt += partial.TotalSegmentsWithExpiresAt
}
