// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

// BucketTally contains information about aggregate data stored in a bucket
type BucketTally struct {
	BucketName []byte

	// TODO(jg): fix this so that it is uuid.UUID
	ProjectID []byte

	Objects int64 // TODO: rename to objects

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
	s.Objects += o.Objects

	s.Segments += o.Segments
	s.InlineSegments += o.InlineSegments
	s.RemoteSegments += o.RemoteSegments
	s.UnknownSegments += o.UnknownSegments

	s.Bytes += o.Bytes
	s.InlineBytes += o.InlineBytes
	s.RemoteBytes += o.RemoteBytes
}
