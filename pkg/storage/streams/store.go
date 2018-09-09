// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
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
	segments            segments.Store
	segmentSize         int64
	key                 []byte
	encryptionBlockSize int
}

// NewStreamStore stuff
func NewStreamStore(segments segments.Store, segmentSize int64, key string, encryptionBlockSize int) (Store, error) {
	if segmentSize <= 0 {
		return nil, errs.New("segment size must be larger than 0")
	}
	if key == "" {
		return nil, errs.New("encryption key must not be empty")
	}
	if encryptionBlockSize <= 0 {
		return nil, errs.New("encryption block size must be larger than 0")
	}

	return &streamStore{
		segments:            segments,
		segmentSize:         segmentSize,
		key:                 []byte(key),
		encryptionBlockSize: encryptionBlockSize,
	}, nil
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

	encKey := sha256.Sum256(s.key)
	var firstNonce [12]byte
	encrypter, err := eestream.NewAESGCMEncrypter(&encKey, &firstNonce, s.encryptionBlockSize)
	if err != nil {
		return Meta{}, err
	}

	awareLimitReader := EOFAwareReader(data)

	for !awareLimitReader.isEOF() {
		segmentPath := path.Prepend(fmt.Sprintf("s%d", totalSegments))
		segmentData := io.LimitReader(awareLimitReader, s.segmentSize)
		paddedData := eestream.PadReader(ioutil.NopCloser(segmentData), encrypter.InBlockSize())
		transformedData := eestream.TransformReader(paddedData, encrypter, 0)

		_, err := s.segments.Put(ctx, segmentPath, transformedData, nil, expiration)
		if err != nil {
			return Meta{}, err
		}
		lastSegmentSize = putMeta.Size
		totalSize = totalSize + putMeta.Size
		totalSegments = totalSegments + 1
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
		_ = lastRangerCloser.Close()
		return nil, Meta{}, err
	}

	newMeta, err := convertMeta(lastSegmentMeta)
	if err != nil {
		_ = lastRangerCloser.Close()
		return nil, Meta{}, err
	}

	encKey := sha256.Sum256(s.key)
	var firstNonce [12]byte
	decrypter, err := eestream.NewAESGCMDecrypter(&encKey, &firstNonce, s.encryptionBlockSize)
	if err != nil {
		_ = lastRangerCloser.Close()
		return nil, Meta{}, err
	}

	var rangers []ranger.RangeCloser
	cleanupRangers := func() {
		for _, r := range rangers {
			_ = r.Close()
		}
		_ = lastRangerCloser.Close()
	}

	for i := int64(0); i < msi.NumberOfSegments; i++ {
		currentPath := fmt.Sprintf("s%d", i)
		rangeCloser, _, err := s.segments.Get(ctx, path.Prepend(currentPath))
		if err != nil {
			cleanupRangers()
			return nil, Meta{}, err
		}

		rd, err := eestream.Transform(rangeCloser, decrypter)
		if err != nil {
			cleanupRangers()
			return nil, Meta{}, err
		}

		paddedSize := rd.Size()
		size := msi.SegmentsSize
		if int64(i) == msi.NumberOfSegments-1 {
			size = msi.LastSegmentSize
		}
		rc, err := eestream.Unpad(rd, int(paddedSize-size)) // int64 -> int; is this a problem?
		if err != nil {
			cleanupRangers()
			return nil, Meta{}, err
		}

		rangers = append(rangers, rc)
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
