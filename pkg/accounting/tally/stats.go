// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"storj.io/storj/pkg/pb"
)

// stats all stats fields
type stats struct {
	Segments        int64
	InlineSegments  int64
	RemoteSegments  int64
	UnknownSegments int64

	Files       int64
	InlineFiles int64
	RemoteFiles int64

	Bytes       int64
	InlineBytes int64
	RemoteBytes int64
}

// Combine aggregates all the stats
func (s *stats) Combine(o *stats) {
	s.Segments += o.Segments
	s.InlineSegments += o.InlineSegments
	s.RemoteSegments += o.RemoteSegments
	s.UnknownSegments += o.UnknownSegments

	s.Files += o.Files
	s.InlineFiles += o.InlineFiles
	s.RemoteFiles += o.RemoteFiles

	s.Bytes += o.Bytes
	s.InlineBytes += o.InlineBytes
	s.RemoteBytes += o.RemoteBytes
}

// AddSegment groups all the data based the passed pointer
func (s *stats) AddSegment(pointer *pb.Pointer, last bool) {
	s.Segments++
	switch pointer.GetType() {
	case pb.Pointer_INLINE:
		s.InlineSegments++
		s.InlineBytes += int64(len(pointer.InlineSegment))
		s.Bytes += int64(len(pointer.InlineSegment))

	case pb.Pointer_REMOTE:
		s.RemoteSegments++
		s.RemoteBytes += pointer.GetSegmentSize()
		s.Bytes += pointer.GetSegmentSize()
	default:
		s.UnknownSegments++
	}

	if last {
		s.Files++
		switch pointer.GetType() {
		case pb.Pointer_INLINE:
			s.InlineFiles++
		case pb.Pointer_REMOTE:
			s.RemoteFiles++
		}
	}
}

// Report reports the stats thru monkit
func (s *stats) Report(prefix string) {
	mon.IntVal(prefix + ".segments").Observe(s.Segments)
	mon.IntVal(prefix + ".inline_segments").Observe(s.InlineSegments)
	mon.IntVal(prefix + ".remote_segments").Observe(s.RemoteSegments)
	mon.IntVal(prefix + ".unknown_segments").Observe(s.UnknownSegments)

	mon.IntVal(prefix + ".files").Observe(s.Files)
	mon.IntVal(prefix + ".inline_files").Observe(s.InlineFiles)
	mon.IntVal(prefix + ".remote_files").Observe(s.RemoteFiles)

	mon.IntVal(prefix + ".bytes").Observe(s.Bytes)
	mon.IntVal(prefix + ".inline_bytes").Observe(s.InlineBytes)
	mon.IntVal(prefix + ".remote_bytes").Observe(s.RemoteBytes)
}
