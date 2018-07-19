// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/segment"
	streamspb "storj.io/storj/protos/streams"
)

var mon = monkit.Package()

// Store for streams
type Store interface {
	// Meta(ctx context.Context, path paths.Path) (storage.Meta, error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) error
	// Get(ctx context.Context, path paths.Path) (ranger.Ranger, paths.Meta, error)
	// Delete(ctx context.Context, path paths.Path) error
	// List(ctx context.Context, startingPath, endingPath paths.Path) (paths []paths.Path, truncated bool, err error)
	// List​(ctx context.Context, root, startAfter, endBefore paths.Path, recursive ​bool​, limit ​int​) (result []paths.Path, more ​bool​, err ​error​)
}

type streamStore struct {
	segments    segment.segmentStore
	segmentSize int64
}

// NewStreams stuff
func NewStreams(segments segments.SegmentStore, segmentSize int64) (StreamStore, error) {
	if segmentSize < 0 {
		return &streamStore{segments: segments, segmentSize: segmentSize}, nil
	}

	return nil, errors.New("Segment size must be larger than 0")
}

func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: break up data as it comes in into s.segmentSize length pieces, then
	// store the first piece at s0/<path>, second piece at s1/<path>, and the
	// *last* piece at l/<path>. Store the given metadata, along with the number
	// of segments, in a new protobuf, in the metadata of l/<path>.

	identitySlice := make([]byte, 0)
	totalSegmentsSize := 0
	totalSegments := 0
	stopLoop := false

	for !stopLoop {
		lr := io.LimitReader(data, s.segmentSize)

		_, err := lr.Read(identitySlice)
		if err != nil {
			stopLoop = true
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				totalSegments = totalSegments + 1
				identitySegmentData := bytes.NewReader(identitySlice)
				lastSegmentPath := path.Prepend("l")

				md := streamspb.Meta{NumberOfSegments: totalSegments, MetaData: metadata}
				lastSegmentMetadata, err := proto.Marshal(&md)

				s.segments.Put(ctx, lastSegmentPath, identitySegmentData, lastSegmentMetadata, expiration)
			}

			return err
		}

		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		segmentData := lr
		segmentMetatdata := identitySlice
		s.segments.Put(ctx, segmentPath, segmentData, segmentMetatdata, expiration)
	}

	return nil
}

/*
func (s *streamStore) Meta(ctx context.Context, path paths.Path) (storage.Meta, error) {

}

func (s *streamStore) Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error) {
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
