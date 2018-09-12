// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"fmt"
	"io"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/utils"
	streamspb "storj.io/storj/protos/streams"
)

var mon = monkit.Package()

// Meta info about a segment
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Data       []byte
}

// convertMeta converts segment metadata to stream metadata
func convertMeta(segmentMeta segments.Meta) (Meta, error) {
	msi := streamspb.MetaStreamInfo{}
	err := proto.Unmarshal(segmentMeta.Data, &msi)
	if err != nil {
		return Meta{}, err
	}

	return Meta{
		Modified:   segmentMeta.Modified,
		Expiration: segmentMeta.Expiration,
		Size:       ((msi.NumberOfSegments - 1) * msi.SegmentsSize) + msi.LastSegmentSize,
		Data:       msi.Metadata,
	}, nil
}

// Store interface methods for streams to satisfy to be a store
type Store interface {
	Meta(ctx context.Context, path paths.Path) (Meta, error)
	Get(ctx context.Context, path paths.Path) (ranger.RangeCloser, Meta, error)
	Put(ctx context.Context, path paths.Path, data io.Reader,
		metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint32) (items []ListItem,
		more bool, err error)
}

// streamStore is a store for streams
type streamStore struct {
	segments    segments.Store
	segmentSize int64
}

// NewStreamStore stuff
func NewStreamStore(segments segments.Store, segmentSize int64) (Store, error) {
	if segmentSize <= 0 {
		return nil, errs.New("segment size must be larger than 0")
	}
	return &streamStore{segments: segments, segmentSize: segmentSize}, nil
}

// Put breaks up data as it comes in into s.segmentSize length pieces, then
// store the first piece at s0/<path>, second piece at s1/<path>, and the
// *last* piece at l/<path>. Store the given metadata, along with the number
// of segments, in a new protobuf, in the metadata of l/<path>.
func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (m Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	var totalSegments int64
	var totalSize int64
	var lastSegmentSize int64

	awareLimitReader := EOFAwareReader(data)

	for !awareLimitReader.isEOF() && !awareLimitReader.hasError() {
		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		segmentData := io.LimitReader(awareLimitReader, s.segmentSize)

		putMeta, err := s.segments.Put(ctx, segmentPath, segmentData, nil, expiration)
		if err != nil {
			return Meta{}, err
		}
		lastSegmentSize = putMeta.Size
		totalSize = totalSize + putMeta.Size
		totalSegments = totalSegments + 1
	}
	if awareLimitReader.hasError() {
		return Meta{}, awareLimitReader.err
	}

	lastSegmentPath := path.Prepend("l")

	md := streamspb.MetaStreamInfo{
		NumberOfSegments: totalSegments,
		SegmentsSize:     s.segmentSize,
		LastSegmentSize:  lastSegmentSize,
		Metadata:         metadata,
	}
	lastSegmentMetadata, err := proto.Marshal(&md)
	if err != nil {
		return Meta{}, err
	}

	putMeta, err := s.segments.Put(ctx, lastSegmentPath, data,
		lastSegmentMetadata, expiration)
	if err != nil {
		return Meta{}, err
	}
	totalSize = totalSize + putMeta.Size

	resultMeta := Meta{
		Modified:   putMeta.Modified,
		Expiration: expiration,
		Size:       totalSize,
		Data:       metadata,
	}

	return resultMeta, nil
}

// Get returns a ranger that knows what the overall size is (from l/<path>)
// and then returns the appropriate data from segments s0/<path>, s1/<path>,
// ..., l/<path>.
func (s *streamStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	lastRangerCloser, lastSegmentMeta, err := s.segments.Get(ctx, path.Prepend("l"))
	if err != nil {
		return nil, Meta{}, err
	}

	msi := streamspb.MetaStreamInfo{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &msi)
	if err != nil {
		utils.LogClose(lastRangerCloser)
		return nil, Meta{}, err
	}

	newMeta, err := convertMeta(lastSegmentMeta)
	if err != nil {
		utils.LogClose(lastRangerCloser)
		return nil, Meta{}, err
	}

	var rangers []ranger.RangeCloser

	for i := int64(0); i < msi.NumberOfSegments; i++ {
		currentPath := fmt.Sprintf("s%d", i)
		size := msi.SegmentsSize
		if i == msi.NumberOfSegments-1 {
			size = msi.LastSegmentSize
		}
		rr := &lazySegmentRanger{
			segments: s.segments,
			path:     path.Prepend(currentPath),
			size:     size,
		}
		rangers = append(rangers, rr)
	}

	rangers = append(rangers, lastRangerCloser)

	catRangers := ranger.Concat(rangers...)

	return catRangers, newMeta, nil
}

// Meta implements Store.Meta
func (s *streamStore) Meta(ctx context.Context, path paths.Path) (Meta, error) {
	segmentMeta, err := s.segments.Meta(ctx, path.Prepend("l"))
	if err != nil {
		return Meta{}, err
	}

	meta, err := convertMeta(segmentMeta)
	if err != nil {
		return Meta{}, err
	}

	return meta, nil
}

// Delete all the segments, with the last one last
func (s *streamStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	lastSegmentMeta, err := s.segments.Meta(ctx, path.Prepend("l"))
	if err != nil {
		return err
	}

	msi := streamspb.MetaStreamInfo{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &msi)
	if err != nil {
		return err
	}

	for i := 0; i < int(msi.NumberOfSegments); i++ {
		currentPath := fmt.Sprintf("s%d", i)
		err := s.segments.Delete(ctx, path.Prepend(currentPath))
		if err != nil {
			return err
		}
	}

	return s.segments.Delete(ctx, path.Prepend("l"))
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     paths.Path
	Meta     Meta
	IsPrefix bool
}

// List all the paths inside l/, stripping off the l/ prefix
func (s *streamStore) List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
	recursive bool, limit int, metaFlags uint32) (items []ListItem,
	more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if metaFlags&meta.Size != 0 {
		// Calculating the stream's size require also the user-defined metadata,
		// where stream store keeps info about the number of segments and their size.
		metaFlags |= meta.UserDefined
	}

	segments, more, err := s.segments.List(ctx, prefix.Prepend("l"), startAfter, endBefore, recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(segments))
	for i, item := range segments {
		newMeta, err := convertMeta(item.Meta)
		if err != nil {
			return nil, false, err
		}
		items[i] = ListItem{Path: item.Path, Meta: newMeta, IsPrefix: item.IsPrefix}
	}

	return items, more, nil
}

type lazySegmentRanger struct {
	ranger   ranger.RangeCloser
	segments segments.Store
	path     paths.Path
	size     int64
}

// Size implements Ranger.Size
func (lr *lazySegmentRanger) Size() int64 {
	return lr.size
}

// Size implements Ranger.Close
func (lr *lazySegmentRanger) Close() error {
	if lr.ranger == nil {
		return nil
	}
	return lr.ranger.Close()
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazySegmentRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if lr.ranger == nil {
		rr, _, err := lr.segments.Get(ctx, lr.path)
		if err != nil {
			return nil, err
		}
		lr.ranger = rr
	}
	return lr.ranger.Range(ctx, offset, length)
}
