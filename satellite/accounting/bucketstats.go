// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/satellite/metainfo/metabase"
)

// BucketTally contains information about aggregate data stored in a bucket.
type BucketTally struct {
	metabase.BucketLocation

	ObjectCount int64

	InlineSegments int64
	RemoteSegments int64

	InlineBytes int64
	RemoteBytes int64

	MetadataSize int64
}

// Combine aggregates all the tallies.
func (s *BucketTally) Combine(o *BucketTally) {
	s.ObjectCount += o.ObjectCount

	s.InlineSegments += o.InlineSegments
	s.RemoteSegments += o.RemoteSegments

	s.InlineBytes += o.InlineBytes
	s.RemoteBytes += o.RemoteBytes
}

// Segments returns total number of segments.
func (s *BucketTally) Segments() int64 {
	return s.InlineSegments + s.RemoteSegments
}

// Bytes returns total bytes.
func (s *BucketTally) Bytes() int64 {
	return s.InlineBytes + s.RemoteBytes
}
