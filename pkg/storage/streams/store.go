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
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/storage"
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
func convertMeta(lastSegmentMeta segments.Meta) (Meta, error) {
	stream := pb.StreamInfo{}
	err := proto.Unmarshal(lastSegmentMeta.Data, &stream)
	if err != nil {
		return Meta{}, err
	}

	return Meta{
		Modified:   lastSegmentMeta.Modified,
		Expiration: lastSegmentMeta.Expiration,
		Size:       ((stream.NumberOfSegments - 1) * stream.SegmentsSize) + stream.LastSegmentSize,
		Data:       stream.Metadata,
	}, nil
}

// Store interface methods for streams to satisfy to be a store
type Store interface {
	Meta(ctx context.Context, path paths.Path) (Meta, error)
	Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

// streamStore is a store for streams
type streamStore struct {
	segments     segments.Store
	segmentSize  int64
	rootKey      []byte
	encBlockSize int
	encType      encryption.Cipher
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
		encType:      encryption.Cipher(encType),
	}, nil
}

// Put breaks up data as it comes in into s.segmentSize length pieces, then
// store the first piece at s0/<path>, second piece at s1/<path>, and the
// *last* piece at l/<path>. Store the given metadata, along with the number
// of segments, in a new protobuf, in the metadata of l/<path>.
func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	// previously file uploaded?
	err = s.Delete(ctx, path)
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		//something wrong happened checking for an existing
		//file with the same name
		return Meta{}, err
	}

	var currentSegment int64
	var streamSize int64
	var putMeta segments.Meta

	defer func() {
		select {
		case <-ctx.Done():
			s.cancelHandler(context.Background(), currentSegment, path)
		default:
		}
	}()

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return Meta{}, err
	}
	cipher := s.encType

	eofReader := NewEOFReader(data)

	for !eofReader.isEOF() && !eofReader.hasError() {
		// generate random key for encrypting the segment's content
		var contentKey encryption.Key
		_, err = rand.Read(contentKey[:])
		if err != nil {
			return Meta{}, err
		}

		// Initialize the content nonce with the segment's index incremented by 1.
		// The increment by 1 is to avoid nonce reuse with the metadata encryption,
		// which is encrypted with the zero nonce.
		var contentNonce encryption.Nonce
		_, err := contentNonce.Increment(currentSegment + 1)
		if err != nil {
			return Meta{}, err
		}

		encrypter, err := cipher.NewEncrypter(&contentKey, &contentNonce, s.encBlockSize)
		if err != nil {
			return Meta{}, err
		}

		// generate random nonce for encrypting the content key
		var keyNonce encryption.Nonce
		_, err = rand.Read(keyNonce[:])
		if err != nil {
			return Meta{}, err
		}

		encryptedKey, err := cipher.Encrypt(contentKey[:], (*encryption.Key)(derivedKey), &keyNonce)
		if err != nil {
			return Meta{}, err
		}

		sizeReader := NewSizeReader(eofReader)
		segmentReader := io.LimitReader(sizeReader, s.segmentSize)
		peekReader := segments.NewPeekThresholdReader(segmentReader)
		largeData, err := peekReader.IsLargerThan(encrypter.InBlockSize())
		if err != nil {
			return Meta{}, err
		}
		var transformedReader io.Reader
		if largeData {
			paddedReader := eestream.PadReader(ioutil.NopCloser(peekReader), encrypter.InBlockSize())
			transformedReader = encryption.TransformReader(paddedReader, encrypter, 0)
		} else {
			data, err := ioutil.ReadAll(peekReader)
			if err != nil {
				return Meta{}, err
			}
			cipherData, err := cipher.Encrypt(data, &contentKey, &contentNonce)
			if err != nil {
				return Meta{}, err
			}
			transformedReader = bytes.NewReader(cipherData)
		}

		putMeta, err = s.segments.Put(ctx, transformedReader, expiration, func() (paths.Path, []byte, error) {
			encPath, err := encryptAfterBucket(path, s.rootKey)
			if err != nil {
				return nil, nil, err
			}

			if !eofReader.isEOF() {
				segmentPath := getSegmentPath(encPath, currentSegment)

				if cipher == encryption.None {
					return segmentPath, nil, nil
				}

				segmentMeta, err := proto.Marshal(&pb.SegmentMeta{
					EncryptedKey: encryptedKey,
					KeyNonce:     keyNonce[:],
				})
				if err != nil {
					return nil, nil, err
				}

				return segmentPath, segmentMeta, nil
			}

			lastSegmentPath := encPath.Prepend("l")

			streamInfo, err := proto.Marshal(&pb.StreamInfo{
				NumberOfSegments: currentSegment + 1,
				SegmentsSize:     s.segmentSize,
				LastSegmentSize:  sizeReader.Size(),
				Metadata:         metadata,
			})
			if err != nil {
				return nil, nil, err
			}

			// encrypt metadata with the content encryption key and zero nonce
			encryptedStreamInfo, err := cipher.Encrypt(streamInfo, &contentKey, &encryption.Nonce{})
			if err != nil {
				return nil, nil, err
			}

			streamMeta := pb.StreamMeta{
				EncryptedStreamInfo: encryptedStreamInfo,
				EncryptionType:      int32(s.encType),
				EncryptionBlockSize: int32(s.encBlockSize),
			}

			if cipher != encryption.None {
				streamMeta.LastSegmentMeta = &pb.SegmentMeta{
					EncryptedKey: encryptedKey,
					KeyNonce:     keyNonce[:],
				}
			}

			lastSegmentMeta, err := proto.Marshal(&streamMeta)
			if err != nil {
				return nil, nil, err
			}

			return lastSegmentPath, lastSegmentMeta, nil
		})
		if err != nil {
			return Meta{}, err
		}

		currentSegment++
		streamSize += sizeReader.Size()
	}

	if eofReader.hasError() {
		return Meta{}, eofReader.err
	}

	resultMeta := Meta{
		Modified:   putMeta.Modified,
		Expiration: expiration,
		Size:       streamSize,
		Data:       metadata,
	}

	return resultMeta, nil
}

// getSegmentPath returns the unique path for a particular segment
func getSegmentPath(p paths.Path, segNum int64) paths.Path {
	return p.Prepend(fmt.Sprintf("s%d", segNum))
}

// Get returns a ranger that knows what the overall size is (from l/<path>)
// and then returns the appropriate data from segments s0/<path>, s1/<path>,
// ..., l/<path>.
func (s *streamStore) Get(ctx context.Context, path paths.Path) (rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		return nil, Meta{}, err
	}

	lastSegmentRanger, lastSegmentMeta, err := s.segments.Get(ctx, encPath.Prepend("l"))
	if err != nil {
		return nil, Meta{}, err
	}

	streamInfo, err := decryptStreamInfo(ctx, lastSegmentMeta, path, s.rootKey)
	if err != nil {
		return nil, Meta{}, err
	}

	stream := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfo, &stream)
	if err != nil {
		return nil, Meta{}, err
	}

	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		return nil, Meta{}, err
	}

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return nil, Meta{}, err
	}

	var rangers []ranger.Ranger
	for i := int64(0); i < stream.NumberOfSegments-1; i++ {
		currentPath := getSegmentPath(encPath, i)
		size := stream.SegmentsSize
		var contentNonce encryption.Nonce
		_, err := contentNonce.Increment(i + 1)
		if err != nil {
			return nil, Meta{}, err
		}
		rr := &lazySegmentRanger{
			segments:      s.segments,
			path:          currentPath,
			size:          size,
			derivedKey:    (*encryption.Key)(derivedKey),
			startingNonce: &contentNonce,
			encBlockSize:  int(streamMeta.EncryptionBlockSize),
			encType:       encryption.Cipher(streamMeta.EncryptionType),
		}
		rangers = append(rangers, rr)
	}

	var contentNonce encryption.Nonce
	_, err = contentNonce.Increment(stream.NumberOfSegments)
	if err != nil {
		return nil, Meta{}, err
	}
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	decryptedLastSegmentRanger, err := decryptRanger(
		ctx,
		lastSegmentRanger,
		stream.LastSegmentSize,
		encryption.Cipher(streamMeta.EncryptionType),
		(*encryption.Key)(derivedKey),
		encryptedKey,
		keyNonce,
		&contentNonce,
		int(streamMeta.EncryptionBlockSize),
	)
	if err != nil {
		return nil, Meta{}, err
	}
	rangers = append(rangers, decryptedLastSegmentRanger)

	catRangers := ranger.Concat(rangers...)

	lastSegmentMeta.Data = streamInfo
	meta, err = convertMeta(lastSegmentMeta)
	if err != nil {
		return nil, Meta{}, err
	}

	return catRangers, meta, nil
}

// Meta implements Store.Meta
func (s *streamStore) Meta(ctx context.Context, path paths.Path) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		return Meta{}, err
	}

	lastSegmentMeta, err := s.segments.Meta(ctx, encPath.Prepend("l"))
	if err != nil {
		return Meta{}, err
	}

	streamInfo, err := decryptStreamInfo(ctx, lastSegmentMeta, path, s.rootKey)
	if err != nil {
		return Meta{}, err
	}

	lastSegmentMeta.Data = streamInfo
	newStreamMeta, err := convertMeta(lastSegmentMeta)
	if err != nil {
		return Meta{}, err
	}

	return newStreamMeta, nil
}

// Delete all the segments, with the last one last
func (s *streamStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		return err
	}
	lastSegmentMeta, err := s.segments.Meta(ctx, encPath.Prepend("l"))
	if err != nil {
		return err
	}

	streamInfo, err := decryptStreamInfo(ctx, lastSegmentMeta, path, s.rootKey)
	if err != nil {
		return err
	}

	stream := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfo, &stream)
	if err != nil {
		return err
	}

	for i := 0; i < int(stream.NumberOfSegments-1); i++ {
		encPath, err = encryptAfterBucket(path, s.rootKey)
		if err != nil {
			return err
		}
		currentPath := getSegmentPath(encPath, int64(i))
		err := s.segments.Delete(ctx, currentPath)
		if err != nil {
			return err
		}
	}

	return s.segments.Delete(ctx, encPath.Prepend("l"))
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     paths.Path
	Meta     Meta
	IsPrefix bool
}

// List all the paths inside l/, stripping off the l/ prefix
func (s *streamStore) List(ctx context.Context, prefix, startAfter, endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if metaFlags&meta.Size != 0 {
		// Calculating the stream's size require also the user-defined metadata,
		// where stream store keeps info about the number of segments and their size.
		metaFlags |= meta.UserDefined
	}

	encPrefix, err := encryptAfterBucket(prefix, s.rootKey)
	if err != nil {
		return nil, false, err
	}

	prefixKey, err := prefix.DeriveKey(s.rootKey, len(prefix))
	if err != nil {
		return nil, false, err
	}

	encStartAfter, err := s.encryptMarker(startAfter, prefixKey)
	if err != nil {
		return nil, false, err
	}

	encEndBefore, err := s.encryptMarker(endBefore, prefixKey)
	if err != nil {
		return nil, false, err
	}

	segments, more, err := s.segments.List(ctx, encPrefix.Prepend("l"), encStartAfter, encEndBefore, recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(segments))
	for i, item := range segments {
		path, err := s.decryptMarker(item.Path, prefixKey)
		if err != nil {
			return nil, false, err
		}

		streamInfo, err := decryptStreamInfo(ctx, item.Meta, path.Prepend(prefix...), s.rootKey)
		if err != nil {
			return nil, false, err
		}

		item.Meta.Data = streamInfo
		newMeta, err := convertMeta(item.Meta)
		if err != nil {
			return nil, false, err
		}

		items[i] = ListItem{Path: path, Meta: newMeta, IsPrefix: item.IsPrefix}
	}

	return items, more, nil
}

// encryptMarker is a helper method for encrypting startAfter and endBefore markers
func (s *streamStore) encryptMarker(marker paths.Path, prefixKey []byte) (paths.Path, error) {
	if bytes.Equal(s.rootKey, prefixKey) { // empty prefix
		return encryptAfterBucket(marker, s.rootKey)
	}
	return marker.Encrypt(prefixKey)
}

// decryptMarker is a helper method for decrypting listed path markers
func (s *streamStore) decryptMarker(marker paths.Path, prefixKey []byte) (paths.Path, error) {
	if bytes.Equal(s.rootKey, prefixKey) { // empty prefix
		return decryptAfterBucket(marker, s.rootKey)
	}
	return marker.Decrypt(prefixKey)
}

type lazySegmentRanger struct {
	ranger        ranger.Ranger
	segments      segments.Store
	path          paths.Path
	size          int64
	derivedKey    *encryption.Key
	startingNonce *encryption.Nonce
	encBlockSize  int
	encType       encryption.Cipher
}

// Size implements Ranger.Size
func (lr *lazySegmentRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazySegmentRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if lr.ranger == nil {
		rr, m, err := lr.segments.Get(ctx, lr.path)
		if err != nil {
			return nil, err
		}
		segmentMeta := pb.SegmentMeta{}
		err = proto.Unmarshal(m.Data, &segmentMeta)
		if err != nil {
			return nil, err
		}
		encryptedKey, keyNonce := getEncryptedKeyAndNonce(&segmentMeta)
		lr.ranger, err = decryptRanger(ctx, rr, lr.size, lr.encType, lr.derivedKey, encryptedKey, keyNonce, lr.startingNonce, lr.encBlockSize)
		if err != nil {
			return nil, err
		}
	}
	return lr.ranger.Range(ctx, offset, length)
}

// decryptRanger returns a decrypted ranger of the given rr ranger
func decryptRanger(ctx context.Context, rr ranger.Ranger, decryptedSize int64, cipher encryption.Cipher, derivedKey *encryption.Key, encryptedKey []byte, encryptedKeyNonce, startingNonce *encryption.Nonce, encBlockSize int) (ranger.Ranger, error) {
	e, err := cipher.Decrypt(encryptedKey, derivedKey, encryptedKeyNonce)
	if err != nil {
		return nil, err
	}
	var encKey encryption.Key
	copy(encKey[:], e)
	decrypter, err := cipher.NewDecrypter(&encKey, startingNonce, encBlockSize)
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
		data, err := cipher.Decrypt(cipherData, &encKey, startingNonce)
		if err != nil {
			return nil, err
		}
		return ranger.ByteRanger(data), nil
	}

	rd, err = encryption.Transform(rr, decrypter)
	if err != nil {
		return nil, err
	}
	return eestream.Unpad(rd, int(rd.Size()-decryptedSize))
}

// encryptAfterBucket encrypts a path without encrypting its first element
func encryptAfterBucket(p paths.Path, key []byte) (encrypted paths.Path, err error) {
	if len(p) <= 1 {
		return p, nil
	}
	bucket := p[0]
	toEncrypt := p[1:]

	// derive a key from the bucket so the same path in different buckets is encrypted differently
	bucketKey, err := p.DeriveKey(key, 1)
	if err != nil {
		return nil, err
	}
	encPath, err := toEncrypt.Encrypt(bucketKey)
	if err != nil {
		return nil, err
	}
	return encPath.Prepend(bucket), nil
}

// decryptAfterBucket decrypts a path without modifying its first element
func decryptAfterBucket(p paths.Path, key []byte) (decrypted paths.Path, err error) {
	if len(p) <= 1 {
		return p, nil
	}
	bucket := p[0]
	toDecrypt := p[1:]

	bucketKey, err := p.DeriveKey(key, 1)
	if err != nil {
		return nil, err
	}
	decPath, err := toDecrypt.Decrypt(bucketKey)
	if err != nil {
		return nil, err
	}
	return decPath.Prepend(bucket), nil
}

// CancelHandler handles clean up of segments on receiving CTRL+C
func (s *streamStore) cancelHandler(ctx context.Context, totalSegments int64, path paths.Path) {
	for i := int64(0); i < totalSegments; i++ {
		currentPath := getSegmentPath(path, i)
		err := s.segments.Delete(ctx, currentPath)
		if err != nil {
			zap.S().Warnf("Failed deleting a segment %v %v", currentPath, err)
		}
	}
}

func getEncryptedKeyAndNonce(m *pb.SegmentMeta) ([]byte, *encryption.Nonce) {
	if m == nil {
		return nil, nil
	}

	var nonce encryption.Nonce
	copy(nonce[:], m.KeyNonce)

	return m.EncryptedKey, &nonce
}

func decryptStreamInfo(ctx context.Context, item segments.Meta, path paths.Path, rootKey []byte) (streamInfo []byte, err error) {
	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(item.Data, &streamMeta)
	if err != nil {
		return nil, err
	}

	derivedKey, err := path.DeriveContentKey(rootKey)
	if err != nil {
		return nil, err
	}

	cipher := encryption.Cipher(streamMeta.EncryptionType)
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	e, err := cipher.Decrypt(encryptedKey, (*encryption.Key)(derivedKey), keyNonce)
	if err != nil {
		return nil, err
	}

	var contentKey encryption.Key
	copy(contentKey[:], e)

	// decrypt metadata with the content encryption key and zero nonce
	return cipher.Decrypt(streamMeta.EncryptedStreamInfo, &contentKey, &encryption.Nonce{})
}
