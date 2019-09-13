// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"github.com/skyrings/skyring-common/tools/uuid"
)

// BucketTally contains information about aggregate data stored in a bucket
type BucketTally struct {
	ProjectID  uuid.UUID
	BucketName []byte

	ObjectCount int64

	Segments        int64
	InlineSegments  int64
	RemoteSegments  int64
	UnknownSegments int64

	Bytes       int64
	InlineBytes int64
	RemoteBytes int64

	MetadataSize int64
}

// Combine aggregates all the tallies
func (s *BucketTally) Combine(o *BucketTally) {
	s.ObjectCount += o.ObjectCount

	s.Segments += o.Segments
	s.InlineSegments += o.InlineSegments
	s.RemoteSegments += o.RemoteSegments
	s.UnknownSegments += o.UnknownSegments

	s.Bytes += o.Bytes
	s.InlineBytes += o.InlineBytes
	s.RemoteBytes += o.RemoteBytes
}
