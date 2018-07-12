// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/segment"
)

var mon = monkit.Package()

type streamStore struct {
	segments    segment.segmentStore
	segmentSize int64
}

// NewStreams stuff
func NewStreams(segments segments.SegmentStore, segmentSize int64) StreamStore {
	return &streamStore{segments: segments, segmentSize: segmentSize}
}

func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: break up data as it comes in into s.segmentSize length pieces, then
	// store the first piece at s0/<path>, second piece at s1/<path>, and the
	// *last* piece at l/<path>. Store the given metadata, along with the number
	// of segments, in a new protobuf, in the metadata of l/<path>.

	type meta struct {
		segmentNumber int64
		segmentSize   int64
	}

	segmentByteSlice := make([]byte, s.segmentSize)
	totalSegmentsSize := 0
	totalSegments := 0
	stopLoop := false

	for stopLoop {
		numBytesRead, err := data.Read(segmentByteSlice)
		if err != nil {
			stopLoop = true
			if err == io.EOF {
				var lastSegmentMetadata bytes.Buffer

				totalSegments = totalSegments + 1
				totalSegmentsSize = totalSegmentsSize + numBytesRead
				segmentData := NewReader(segmentByteSlice)
				lastSegmentPath := path.Prepend("l")
				m := meta{segmentNumber: totalSegments, segmentSize: totalSegmentsSize}
				binary.Write(&lastSegmentMetadata, binary.BigEndian, metadata)
				binary.Write(&lastSegmentMetadata, binary.BigEndian, m)
				s.segments.Put(ctx, lastSegmentPath, segmentData, lastSegmentMetadata, expiration)
			}

			return err
		}

		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		totalSegments = totalSegments + 1
		totalSegmentsSize = totalSegmentsSize + numBytesRead
		segmentData := NewReader(segmentByteSlice)
		segmentMetatdata := metadata
		s.segments.Put(ctx, segmentPath, segmentData, segmentMetatdata, expiration)
	}

	return nil
}

/*
func (s *streamStore) Get(ctx context.Context, path dtypes.Path) (rv ranger.Ranger, m dtypes.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: return a ranger that knows what the overall size is (from l/<path>)
	// and then returns the appropriate data from segments s0/<path>, s1/<path>,
	// ..., l/<path>.

	rv, meta, err := s.store.Get(ctx, path)
	if err != nil {
		return nil, m, err
	}
	return rv, meta.Meta, nil
}

func (s *streamStore) Delete(ctx context.Context, path dtypes.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: delete all the segments, with the last one last

	return s.store.Delete(ctx, path)
}

func (s *streamStore) List(ctx context.Context, startingPath, endingPath dtypes.Path) (paths []dtypes.Path, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: list all the paths inside l/, stripping off the l/ prefix

	return s.store.List(ctx, startingPath, endingPath)
}
*/
