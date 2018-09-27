// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
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
	msi := pb.MetaStreamInfo{}
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
	Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error)
	Put(ctx context.Context, path paths.Path, data io.Reader,
		metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint32) (items []ListItem,
		more bool, err error)
}

// streamStore is a store for streams
type streamStore struct {
	segments     segments.Store
	segmentSize  int64
	rootKey      []byte
	encBlockSize int
	encType      eestream.Cipher
}

// NewStreamStore stuff
func NewStreamStore(segments segments.Store, segmentSize int64, rootKey string, encBlockSize int, encType int) (Store, error) {
	if segmentSize <= 0 {
		return nil, errs.New("segment size must be larger than 0")
	}
	if rootKey == "" {
		return nil, errs.New("encryption key must not be empty")
	}
	if encBlockSize <= 0 {
		return nil, errs.New("encryption block size must be larger than 0")
	}

	return &streamStore{
		segments:     segments,
		segmentSize:  segmentSize,
		rootKey:      []byte(rootKey),
		encBlockSize: encBlockSize,
		encType:      eestream.Cipher(encType),
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

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return Meta{}, err
	}

	cipher := s.encType

	eofReader := NewEOFReader(data)

	for !eofReader.isEOF() && !eofReader.hasError() {
		var encKey eestream.Key
		_, err = rand.Read(encKey[:])
		if err != nil {
			return Meta{}, err
		}

		var nonce eestream.Nonce
		_, err := nonce.Increment(totalSegments)
		if err != nil {
			return Meta{}, err
		}

		encrypter, err := cipher.NewEncrypter(&encKey, &nonce, s.encBlockSize)
		if err != nil {
			return Meta{}, err
		}

		encryptedEncKey, err := cipher.Encrypt(encKey[:], (*eestream.Key)(derivedKey), &nonce)
		if err != nil {
			return Meta{}, err
		}

		sizeReader := NewSizeReader(eofReader)
		segmentPath := getSegmentPath(path, totalSegments)
		segmentReader := io.LimitReader(sizeReader, s.segmentSize)
		peekReader := segments.NewPeekThresholdReader(segmentReader)
		largeData, err := peekReader.IsLargerThan(encrypter.InBlockSize())
		if err != nil {
			return Meta{}, err
		}
		var transformedReader io.Reader
		if largeData {
			paddedReader := eestream.PadReader(ioutil.NopCloser(peekReader), encrypter.InBlockSize())
			transformedReader = eestream.TransformReader(paddedReader, encrypter, 0)
		} else {
			data, err := ioutil.ReadAll(peekReader)
			if err != nil {
				return Meta{}, err
			}
			cipherData, err := cipher.Encrypt(data, &encKey, &nonce)
			if err != nil {
				return Meta{}, err
			}
			transformedReader = bytes.NewReader(cipherData)
		}

		_, err = s.segments.Put(ctx, segmentPath, transformedReader, encryptedEncKey, expiration)
		if err != nil {
			return Meta{}, err
		}

		lastSegmentSize = sizeReader.Size()
		totalSize = totalSize + lastSegmentSize
		totalSegments = totalSegments + 1
	}
	if eofReader.hasError() {
		return Meta{}, eofReader.err
	}

	lastSegmentPath := path.Prepend("l")

	md := pb.MetaStreamInfo{
		NumberOfSegments:    totalSegments,
		SegmentsSize:        s.segmentSize,
		LastSegmentSize:     lastSegmentSize,
		Metadata:            metadata,
		EncryptionType:      int32(s.encType),
		EncryptionBlockSize: int32(s.encBlockSize),
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

// GetSegmentPath returns the unique path for a particular segment
func getSegmentPath(p paths.Path, segNum int64) paths.Path {
	return p.Prepend(fmt.Sprintf("s%d", segNum))
}

// Get returns a ranger that knows what the overall size is (from l/<path>)
// and then returns the appropriate data from segments s0/<path>, s1/<path>,
// ..., l/<path>.
func (s *streamStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	lastSegmentMeta, err := s.segments.Meta(ctx, path.Prepend("l"))
	if err != nil {
		return nil, Meta{}, err
	}

	msi := pb.MetaStreamInfo{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &msi)
	if err != nil {
		return nil, Meta{}, err
	}

	newMeta, err := convertMeta(lastSegmentMeta)
	if err != nil {
		return nil, Meta{}, err
	}

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return nil, Meta{}, err
	}

	var rangers []ranger.Ranger
	for i := int64(0); i < msi.NumberOfSegments; i++ {
		currentPath := getSegmentPath(path, i)
		size := msi.SegmentsSize
		if i == msi.NumberOfSegments-1 {
			size = msi.LastSegmentSize
		}
		var nonce eestream.Nonce
		_, err := nonce.Increment(i)
		if err != nil {
			return nil, Meta{}, err
		}
		rr := &lazySegmentRanger{
			segments:      s.segments,
			path:          currentPath,
			size:          size,
			derivedKey:    (*eestream.Key)(derivedKey),
			startingNonce: &nonce,
			encBlockSize:  int(msi.EncryptionBlockSize),
			encType:       eestream.Cipher(msi.EncryptionType),
		}
		rangers = append(rangers, rr)
	}

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

	msi := pb.MetaStreamInfo{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &msi)
	if err != nil {
		return err
	}

	for i := 0; i < int(msi.NumberOfSegments); i++ {
		currentPath := getSegmentPath(path, int64(i))
		err := s.segments.Delete(ctx, currentPath)
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
	ranger        ranger.Ranger
	segments      segments.Store
	path          paths.Path
	size          int64
	derivedKey    *eestream.Key
	startingNonce *eestream.Nonce
	encBlockSize  int
	encType       eestream.Cipher
}

// Size implements Ranger.Size
func (lr *lazySegmentRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazySegmentRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	cipher := lr.encType

	if lr.ranger == nil {
		rr, m, err := lr.segments.Get(ctx, lr.path)
		if err != nil {
			return nil, err
		}
		encryptedEncKey := m.Data
		e, err := cipher.Decrypt(encryptedEncKey, lr.derivedKey, lr.startingNonce)
		if err != nil {
			return nil, err
		}
		var encKey eestream.Key
		copy(encKey[:], e)
		decrypter, err := cipher.NewDecrypter(&encKey, lr.startingNonce, lr.encBlockSize)
		if err != nil {
			return nil, err
		}

		var rd ranger.Ranger
		if rr.Size()%int64(decrypter.InBlockSize()) != 0 {
			reader, err := rr.Range(ctx, 0, rr.Size())
			if err != nil {
				return nil, err
			}
			cipherData, err := ioutil.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			data, err := cipher.Decrypt(cipherData, &encKey, lr.startingNonce)
			if err != nil {
				return nil, err
			}
			rd = ranger.ByteRanger(data)
			lr.ranger = rd
		} else {
			rd, err = eestream.Transform(rr, decrypter)
			if err != nil {
				return nil, err
			}

			paddedSize := rd.Size()
			rc, err := eestream.Unpad(rd, int(paddedSize-lr.Size()))
			if err != nil {
				return nil, err
			}
			lr.ranger = rc
		}
	}
	return lr.ranger.Range(ctx, offset, length)
}
