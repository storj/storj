// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
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

// Store for streams
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

type streamStore struct {
	segments    segments.Store
	segmentSize int64
}

// NewStreams stuff
func NewStreams(segments segments.Store, segmentSize int64) (Store, error) {
	if segmentSize <= 0 {
		return nil, errors.New("segment size must be larger than 0")
	}
	return &streamStore{segments: segments, segmentSize: segmentSize}, nil
}

func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (m Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: break up data as it comes in into s.segmentSize length pieces, then
	// store the first piece at s0/<path>, second piece at s1/<path>, and the
	// *last* piece at l/<path>. Store the given metadata, along with the number
	// of segments, in a new protobuf, in the metadata of l/<path>.

	identitySlice := make([]byte, 0)
	identityMeta := Meta{}
	var totalSegments int64
	var totalSize int64
	var lastSegmentSize int64

	awareLimitReader := EOFAwareReader(data)

	for !awareLimitReader.isEOF() {
		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		segmentData := io.LimitReader(awareLimitReader, s.segmentSize)
		segmentMetatdata := identitySlice
		putMeta, err := s.segments.Put(ctx, segmentPath, segmentData,
			segmentMetatdata, expiration)
		if err != nil {
			return identityMeta, err
		}
		lastSegmentSize = putMeta.Size
		totalSize = totalSize + putMeta.Size
		totalSegments = totalSegments + 1
	}

	identitySegmentData := data
	lastSegmentPath := path.Prepend("l")

	md := streamspb.MetaStreamInfo{
		NumberOfSegments: totalSegments,
		SegmentsSize:     s.segmentSize,
		LastSegmentSize:  lastSegmentSize,
		MetaData:         metadata,
	}
	lastSegmentMetadata, err := proto.Marshal(&md)
	if err != nil {
		return identityMeta, err
	}

	putMeta, err := s.segments.Put(ctx, lastSegmentPath, identitySegmentData,
		lastSegmentMetadata, expiration)
	if err != nil {
		return identityMeta, err
	}
	totalSize = totalSize + putMeta.Size

	resultMeta := Meta{
		Modified:   putMeta.Modified,
		Expiration: expiration,
		Size:       totalSize,
		Data:       lastSegmentMetadata,
	}

	return resultMeta, nil
}

// EOFAwareLimitReader holds reader and status of EOF
type EOFAwareLimitReader struct {
	reader io.Reader
	eof    bool
}

// EOFAwareReader keeps track of the state, has the internal reader reached EOF
func EOFAwareReader(r io.Reader) *EOFAwareLimitReader {
	return &EOFAwareLimitReader{reader: r, eof: false}
}

func (r *EOFAwareLimitReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if err == io.EOF {
		r.eof = true
	}
	return n, err
}

func (r *EOFAwareLimitReader) isEOF() bool {
	return r.eof
}

func sizeTuple(totalSegments int64, sizeOfSegments int64) (int64, error, int64, error) {
	sizePerSegment := float64(totalSegments) / float64(sizeOfSegments)
	stringSegmentsSize := fmt.Sprintf("%f", sizePerSegment)
	segmentSizeSlice := strings.Split(stringSegmentsSize, ".")
	perfectSizedSegments, pErr := strconv.ParseInt(segmentSizeSlice[0], 10, 64)
	lastSegmentSize, lErr := strconv.ParseInt(segmentSizeSlice[1], 10, 64)

	return perfectSizedSegments, pErr, lastSegmentSize, lErr
}

func (s *streamStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: return a ranger that knows what the overall size is (from l/<path>)
	// and then returns the appropriate data from segments s0/<path>, s1/<path>,
	// ..., l/<path>.

	lastRangerCloser, lastSegmentMeta, err := s.segments.Get(ctx, path.Prepend("l"))
	if err != nil {
		return nil, Meta{}, err
	}
	totalSize := lastSegmentMeta.Size

	newMeta := Meta{
		Modified:   lastSegmentMeta.Modified,
		Expiration: lastSegmentMeta.Expiration,
		Size:       lastSegmentMeta.Size,
		Data:       lastSegmentMeta.Data,
	}

	perfectSizedSegments, pErr, lastSegmentSize, lErr := sizeTuple(totalSize, s.segmentSize)
	if pErr == nil {
		return nil, Meta{}, pErr
	}
	if lErr == nil {
		return nil, Meta{}, lErr
	}

	if perfectSizedSegments == 0 {
		return ranger.NopCloser(lastRangerCloser), newMeta, nil
	}

	var resRanger ranger.Ranger

	for i := 0; i < int(perfectSizedSegments); i++ {
		currentPath := fmt.Sprintf("s%d", i)
		rangeCloser, _, err := s.segments.Get(ctx, path.Prepend(currentPath))
		if err != nil {
			return nil, Meta{}, err
		}

		resRanger = ranger.Concat(resRanger, rangeCloser)
	}

	if lastSegmentSize == 0 {
		return ranger.NopCloser(resRanger), newMeta, nil
	}
	currentPath := fmt.Sprintf("s%d", perfectSizedSegments+1)
	lastRangeCloser, _, err := s.segments.Get(ctx, path.Prepend(currentPath))
	if err != nil {
		return nil, Meta{}, err
	}
	resRanger = ranger.Concat(resRanger, lastRangeCloser)

	return ranger.NopCloser(resRanger), newMeta, nil

}

func (s *streamStore) Meta(ctx context.Context, path paths.Path) (Meta, error) {
	segmentMeta, err := s.segments.Meta(ctx, path)
	if err != nil {
		return Meta{}, err
	}
	meta := Meta{
		Modified:   segmentMeta.Modified,
		Expiration: segmentMeta.Expiration,
		Size:       segmentMeta.Size,
		Data:       segmentMeta.Data,
	}

	return meta, nil
}

func (s *streamStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: delete all the segments, with the last one last

	lastSegmentMeta, err := s.segments.Meta(ctx, path.Prepend("l"))
	if err != nil {
		return err
	}
	totalSize := lastSegmentMeta.Size

	perfectSizedSegments, pErr, lastSegmentSize, lErr := sizeTuple(totalSize, s.segmentSize)
	if pErr == nil {
		return pErr
	}
	if lErr == nil {
		return lErr
	}

	for i := 0; i < int(perfectSizedSegments); i++ {
		currentPath := fmt.Sprintf("s%d", i)
		worked := s.segments.Delete(ctx, path.Prepend(currentPath))
		if worked != nil {
			return worked
		}
	}

	if lastSegmentSize > 0 {
		currentPath := fmt.Sprintf("s%d", perfectSizedSegments+1)
		worked := s.segments.Delete(ctx, path.Prepend(currentPath))
		if worked != nil {
			return worked
		}
	}

	return s.segments.Delete(ctx, path.Prepend("l"))
}

// ListItem is a single item in a listing
type ListItem struct {
	Path paths.Path
	Meta Meta
}

func (s *streamStore) List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
	recursive bool, limit int, metaFlags uint32) (items []ListItem,
	more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: list all the paths inside l/, stripping off the l/ prefix

	lastSegmentMeta, err := s.segments.Meta(ctx, prefix.Prepend("l"))
	if err != nil {
		return nil, false, err
	}
	totalSize := lastSegmentMeta.Size

	perfectSizedSegments, pErr, lastSegmentSize, lErr := sizeTuple(totalSize, s.segmentSize)
	if pErr == nil {
		return nil, false, pErr
	}
	if lErr == nil {
		return nil, false, lErr
	}

	var resItems []ListItem
	var resMoore bool

	for i := 0; i < int(perfectSizedSegments); i++ {
		items, more, err := s.segments.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
		if err != nil {
			return nil, false, err
		}
		for _, item := range items {
			newPath := strings.Split(item.Path.String(), fmt.Sprintf("s%d", i))
			newMeta := Meta{
				Modified:   item.Meta.Modified,
				Expiration: item.Meta.Expiration,
				Size:       item.Meta.Size,
				Data:       item.Meta.Data,
			}
			resItems = append(resItems, ListItem{Path: newPath, Meta: newMeta})
			resMoore = more
		}
	}

	if lastSegmentSize > 0 {
		items, more, err := s.segments.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
		if err != nil {
			return nil, false, err
		}
		for _, item := range items {
			newPath := strings.Split(item.Path.String(), fmt.Sprintf("s%d", perfectSizedSegments+1))
			newMeta := Meta{
				Modified:   item.Meta.Modified,
				Expiration: item.Meta.Expiration,
				Size:       item.Meta.Size,
				Data:       item.Meta.Data,
			}
			resItems = append(resItems, ListItem{Path: newPath, Meta: newMeta})
		}
		return resItems, more, nil
	}

	return resItems, resMoore, nil
}
