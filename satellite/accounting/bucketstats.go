// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/satellite/metabase"
)

// BucketTally contains information about aggregate data stored in a bucket.
type BucketTally struct {
	metabase.BucketLocation

	ObjectCount        int64
	PendingObjectCount int64
	TotalSegments      int64
	TotalBytes         int64

	MetadataSize int64
}

// Combine aggregates all the tallies.
func (s *BucketTally) Combine(o *BucketTally) {
	s.ObjectCount += o.ObjectCount
	s.PendingObjectCount += o.PendingObjectCount
	s.TotalSegments += o.TotalSegments
	s.TotalBytes += o.TotalBytes
	s.MetadataSize += o.MetadataSize
}

// Segments returns total number of segments.
func (s *BucketTally) Segments() int64 {
	return s.TotalSegments
}

// Bytes returns total bytes.
func (s *BucketTally) Bytes() int64 {
	return s.TotalBytes
}
