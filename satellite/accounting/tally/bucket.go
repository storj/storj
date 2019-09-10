// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/accounting"
)

// bucketAddSegment groups all the data based the passed pointer
func bucketAddSegment(tally *accounting.BucketTally, pointer *pb.Pointer, last bool) {
	tally.Segments++
	switch pointer.GetType() {
	case pb.Pointer_INLINE:
		tally.InlineSegments++
		tally.InlineBytes += int64(len(pointer.InlineSegment))
		tally.Bytes += int64(len(pointer.InlineSegment))
		tally.MetadataSize += int64(len(pointer.Metadata))

	case pb.Pointer_REMOTE:
		tally.RemoteSegments++
		tally.RemoteBytes += pointer.GetSegmentSize()
		tally.Bytes += pointer.GetSegmentSize()
		tally.MetadataSize += int64(len(pointer.Metadata))
	default:
		tally.UnknownSegments++
	}

	if last {
		tally.Files++
		switch pointer.GetType() {
		case pb.Pointer_INLINE:
			tally.InlineFiles++
		case pb.Pointer_REMOTE:
			tally.RemoteFiles++
		}
	}
}

// bucketReport reports the stats thru monkit
func bucketReport(tally *accounting.BucketTally, prefix string) {
	mon.IntVal(prefix + ".segments").Observe(tally.Segments)
	mon.IntVal(prefix + ".inline_segments").Observe(tally.InlineSegments)
	mon.IntVal(prefix + ".remote_segments").Observe(tally.RemoteSegments)
	mon.IntVal(prefix + ".unknown_segments").Observe(tally.UnknownSegments)

	mon.IntVal(prefix + ".files").Observe(tally.Files)
	mon.IntVal(prefix + ".inline_files").Observe(tally.InlineFiles)
	mon.IntVal(prefix + ".remote_files").Observe(tally.RemoteFiles)

	mon.IntVal(prefix + ".bytes").Observe(tally.Bytes)
	mon.IntVal(prefix + ".inline_bytes").Observe(tally.InlineBytes)
	mon.IntVal(prefix + ".remote_bytes").Observe(tally.RemoteBytes)
}
