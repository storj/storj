// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
)

var mon = monkit.Package()

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
	s.Segments += o.Segments
	s.InlineSegments += o.InlineSegments
	s.RemoteSegments += o.RemoteSegments
	s.UnknownSegments += o.UnknownSegments

	s.ObjectCount += o.ObjectCount

	s.Bytes += o.Bytes
	s.InlineBytes += o.InlineBytes
	s.RemoteBytes += o.RemoteBytes
}

// AddSegment groups all the data based the passed pointer
func (s *BucketTally) AddSegment(pointer *pb.Pointer, last bool) {
	s.Segments++
	switch pointer.GetType() {
	case pb.Pointer_INLINE:
		s.InlineSegments++
		s.InlineBytes += int64(len(pointer.InlineSegment))
		s.Bytes += int64(len(pointer.InlineSegment))
		s.MetadataSize += int64(len(pointer.Metadata))

	case pb.Pointer_REMOTE:
		s.RemoteSegments++
		s.RemoteBytes += pointer.GetSegmentSize()
		s.Bytes += pointer.GetSegmentSize()
		s.MetadataSize += int64(len(pointer.Metadata))
	default:
		s.UnknownSegments++
	}

	if last {
		s.ObjectCount++
	}
}

// Report reports the stats thru monkit
func (s *BucketTally) Report(prefix string) {
	mon.IntVal(prefix + ".objects").Observe(s.ObjectCount)

	mon.IntVal(prefix + ".segments").Observe(s.Segments)
	mon.IntVal(prefix + ".inline_segments").Observe(s.InlineSegments)
	mon.IntVal(prefix + ".remote_segments").Observe(s.RemoteSegments)
	mon.IntVal(prefix + ".unknown_segments").Observe(s.UnknownSegments)

	mon.IntVal(prefix + ".bytes").Observe(s.Bytes)
	mon.IntVal(prefix + ".inline_bytes").Observe(s.InlineBytes)
	mon.IntVal(prefix + ".remote_bytes").Observe(s.RemoteBytes)
}
