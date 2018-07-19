// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/segment"
	"storj.io/storj/pkg/storage"
	streamspb "storj.io/storj/protos/streams"
)

var mon = monkit.Package()

// Store for streams
type Store interface {
	Put(ctx context.Context, path paths.Path, data io.Reader,
		metadata []byte, expiration time.Time) (storage.Meta, error)
	// Get(ctx context.Context, path paths.Path) (ranger.RangeCloser,
	// 	storage.Meta, error)
	// Delete(ctx context.Context, path paths.Path) error
	// List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
	// 	recursive bool, limit int, metaFlags uint64) (items []storage.ListItem,
	// 	more bool, err error)
	// Meta(ctx context.Context, path paths.Path) (storage.Meta, error)
}

type streamStore struct {
	segments    segment.Store
	segmentSize int64
}

// NewStreams stuff
func NewStreams(segments segment.Store, segmentSize int64) (Store, error) {
	if segmentSize < 0 {
		return nil, errors.New("Segment size must be larger than 0")
	}
	return &streamStore{segments: segments, segmentSize: segmentSize}, nil
}

func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (m storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: break up data as it comes in into s.segmentSize length pieces, then
	// store the first piece at s0/<path>, second piece at s1/<path>, and the
	// *last* piece at l/<path>. Store the given metadata, along with the number
	// of segments, in a new protobuf, in the metadata of l/<path>.

	identitySlice := make([]byte, 0)
	identityMeta := storage.Meta{}
	var totalSegments int64
	totalSegments = 0
	stopLoop := false

	for !stopLoop {
		lr := io.LimitReader(data, s.segmentSize)

		_, err := lr.Read(identitySlice)
		if err != nil {
			stopLoop = true
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				totalSegments = totalSegments + 1
				identitySegmentData := data
				lastSegmentPath := path.Prepend("l")

				md := streamspb.MetaStreamInfo{NumberOfSegments: totalSegments, MetaData: metadata}
				lastSegmentMetadata, err := proto.Marshal(&md)

				err = s.segments.Put(ctx, lastSegmentPath, identitySegmentData, lastSegmentMetadata, expiration)
				if err != nil {
					return identityMeta, err
				}
			}

			return identityMeta, err
		}

		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		segmentData := lr
		segmentMetatdata := identitySlice
		err = s.segments.Put(ctx, segmentPath, segmentData, segmentMetatdata, expiration)
		if err != nil {
			return identityMeta, err
		}
	}

	res := storage.Meta{
		Modified:   time.Now(),
		Expiration: expiration,
		Size:       totalSegments * s.segmentSize,
		Checksum:   "",
	}

	return res, nil
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
