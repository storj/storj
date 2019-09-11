// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/segments"
)

var mon = monkit.Package()

// Meta info about a stream
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Data       []byte
}

func numberOfSegments(stream *pb.StreamInfo, streamMeta *pb.StreamMeta) int64 {
	if streamMeta.NumberOfSegments > 0 {
		return streamMeta.NumberOfSegments
	}
	return stream.DeprecatedNumberOfSegments
}

// convertMeta converts segment metadata to stream metadata
func convertMeta(modified, expiration time.Time, stream pb.StreamInfo, streamMeta pb.StreamMeta) Meta {
	return Meta{
		Modified:   modified,
		Expiration: expiration,
		Size:       ((numberOfSegments(&stream, &streamMeta) - 1) * stream.SegmentsSize) + stream.LastSegmentSize,
		Data:       stream.Metadata,
	}
}

// Store interface methods for streams to satisfy to be a store
type typedStore interface {
	Meta(ctx context.Context, path Path, pathCipher storj.CipherSuite) (Meta, error)
	Get(ctx context.Context, path Path, pathCipher storj.CipherSuite) (ranger.Ranger, Meta, error)
	Put(ctx context.Context, path Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path Path, pathCipher storj.CipherSuite) error
	List(ctx context.Context, prefix Path, startAfter, endBefore string, pathCipher storj.CipherSuite, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

// streamStore is a store for streams. It implements typedStore as part of an ongoing migration
// to use typed paths. See the shim for the store that the rest of the world interacts with.
type streamStore struct {
	metainfo        *metainfo.Client
	segments        segments.Store
	segmentSize     int64
	encStore        *encryption.Store
	encBlockSize    int
	cipher          storj.CipherSuite
	inlineThreshold int
}

// newTypedStreamStore constructs a typedStore backed by a streamStore.
func newTypedStreamStore(metainfo *metainfo.Client, segments segments.Store, segmentSize int64, encStore *encryption.Store, encBlockSize int, cipher storj.CipherSuite, inlineThreshold int) (typedStore, error) {
	if segmentSize <= 0 {
		return nil, errs.New("segment size must be larger than 0")
	}
	if encBlockSize <= 0 {
		return nil, errs.New("encryption block size must be larger than 0")
	}

	return &streamStore{
		metainfo:        metainfo,
		segments:        segments,
		segmentSize:     segmentSize,
		encStore:        encStore,
		encBlockSize:    encBlockSize,
		cipher:          cipher,
		inlineThreshold: inlineThreshold,
	}, nil
}

// Put breaks up data as it comes in into s.segmentSize length pieces, then
// store the first piece at s0/<path>, second piece at s1/<path>, and the
// *last* piece at l/<path>. Store the given metadata, along with the number
// of segments, in a new protobuf, in the metadata of l/<path>.
func (s *streamStore) Put(ctx context.Context, path Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (m Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// previously file uploaded?
	err = s.Delete(ctx, path, pathCipher)
	if err != nil && !storj.ErrObjectNotFound.Has(err) {
		// something wrong happened checking for an existing
		// file with the same name
		return Meta{}, err
	}

	m, lastSegment, streamID, err := s.upload(ctx, path, pathCipher, data, metadata, expiration)
	if err != nil {
		s.cancelHandler(context.Background(), streamID, lastSegment, path, pathCipher)
	}

	return m, err
}

func (s *streamStore) upload(ctx context.Context, path Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (m Meta, lastSegment int64, streamID storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	var currentSegment int64
	var streamSize int64
	var putMeta segments.Meta
	var objectMetadata []byte

	derivedKey, err := encryption.DeriveContentKey(path.Bucket(), path.UnencryptedPath(), s.encStore)
	if err != nil {
		return Meta{}, currentSegment, streamID, err
	}
	encPath, err := encryption.EncryptPath(path.Bucket(), path.UnencryptedPath(), pathCipher, s.encStore)
	if err != nil {
		return Meta{}, currentSegment, streamID, err
	}

	streamID, err = s.metainfo.BeginObject(ctx, metainfo.BeginObjectParams{
		Bucket:        []byte(path.Bucket()),
		EncryptedPath: []byte(encPath.Raw()),
		ExpiresAt:     expiration,
	})
	if err != nil {
		return Meta{}, currentSegment, streamID, err
	}

	defer func() {
		select {
		case <-ctx.Done():
			s.cancelHandler(context.Background(), streamID, currentSegment, path, pathCipher)
		default:
		}
	}()

	eofReader := NewEOFReader(data)

	for !eofReader.isEOF() && !eofReader.hasError() {
		// generate random key for encrypting the segment's content
		var contentKey storj.Key
		_, err = rand.Read(contentKey[:])
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		// Initialize the content nonce with the segment's index incremented by 1.
		// The increment by 1 is to avoid nonce reuse with the metadata encryption,
		// which is encrypted with the zero nonce.
		var contentNonce storj.Nonce
		_, err := encryption.Increment(&contentNonce, currentSegment+1)
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		encrypter, err := encryption.NewEncrypter(s.cipher, &contentKey, &contentNonce, s.encBlockSize)
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		// generate random nonce for encrypting the content key
		var keyNonce storj.Nonce
		_, err = rand.Read(keyNonce[:])
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		encryptedKey, err := encryption.EncryptKey(&contentKey, s.cipher, derivedKey, &keyNonce)
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		sizeReader := NewSizeReader(eofReader)
		segmentReader := io.LimitReader(sizeReader, s.segmentSize)
		peekReader := segments.NewPeekThresholdReader(segmentReader)
		// If the data is larger than the inline threshold size, then it will be a remote segment
		isRemote, err := peekReader.IsLargerThan(s.inlineThreshold)
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}
		var transformedReader io.Reader
		if isRemote {
			paddedReader := eestream.PadReader(ioutil.NopCloser(peekReader), encrypter.InBlockSize())
			transformedReader = encryption.TransformReader(paddedReader, encrypter, 0)
		} else {
			data, err := ioutil.ReadAll(peekReader)
			if err != nil {
				return Meta{}, currentSegment, streamID, err
			}
			cipherData, err := encryption.Encrypt(data, s.cipher, &contentKey, &contentNonce)
			if err != nil {
				return Meta{}, currentSegment, streamID, err
			}
			transformedReader = bytes.NewReader(cipherData)
		}

		putMeta, err = s.segments.Put(ctx, streamID, transformedReader, expiration, func() (_ int64, segmentEncryption storj.SegmentEncryption, err error) {
			if s.cipher != storj.EncNull {
				segmentEncryption = storj.SegmentEncryption{
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: keyNonce,
				}
			}
			return currentSegment, segmentEncryption, nil
		})
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		streamInfo, err := proto.Marshal(&pb.StreamInfo{
			DeprecatedNumberOfSegments: currentSegment + 1,
			SegmentsSize:               s.segmentSize,
			LastSegmentSize:            sizeReader.Size(),
			Metadata:                   metadata,
		})
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		// encrypt metadata with the content encryption key and zero nonce
		encryptedStreamInfo, err := encryption.Encrypt(streamInfo, s.cipher, &contentKey, &storj.Nonce{})
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		streamMeta := pb.StreamMeta{
			NumberOfSegments:    currentSegment + 1,
			EncryptedStreamInfo: encryptedStreamInfo,
			EncryptionType:      int32(s.cipher),
			EncryptionBlockSize: int32(s.encBlockSize),
		}

		if s.cipher != storj.EncNull {
			streamMeta.LastSegmentMeta = &pb.SegmentMeta{
				EncryptedKey: encryptedKey,
				KeyNonce:     keyNonce[:],
			}

		}

		objectMetadata, err = proto.Marshal(&streamMeta)
		if err != nil {
			return Meta{}, currentSegment, streamID, err
		}

		currentSegment++
		streamSize += sizeReader.Size()
	}

	if eofReader.hasError() {
		return Meta{}, currentSegment, streamID, eofReader.err
	}

	err = s.metainfo.CommitObject(ctx, metainfo.CommitObjectParams{
		StreamID:          streamID,
		EncryptedMetadata: objectMetadata,
	})
	if err != nil {
		return Meta{}, currentSegment, streamID, err
	}

	resultMeta := Meta{
		Modified:   putMeta.Modified,
		Expiration: expiration,
		Size:       streamSize,
		Data:       metadata,
	}

	return resultMeta, currentSegment, streamID, nil
}

// Get returns a ranger that knows what the overall size is (from l/<path>)
// and then returns the appropriate data from segments s0/<path>, s1/<path>,
// ..., l/<path>.
func (s *streamStore) Get(ctx context.Context, path Path, pathCipher storj.CipherSuite) (rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryption.EncryptPath(path.Bucket(), path.UnencryptedPath(), pathCipher, s.encStore)
	if err != nil {
		return nil, Meta{}, err
	}

	object, err := s.metainfo.GetObject(ctx, metainfo.GetObjectParams{
		Bucket:        []byte(path.Bucket()),
		EncryptedPath: []byte(encPath.Raw()),
	})
	if err != nil {
		return nil, Meta{}, err
	}

	lastSegmentRanger, _, err := s.segments.Get(ctx, object.StreamID, -1, object.RedundancyScheme)
	if err != nil {
		return nil, Meta{}, err
	}

	streamInfo, streamMeta, err := TypedDecryptStreamInfo(ctx, object.Metadata, path, s.encStore)
	if err != nil {
		return nil, Meta{}, err
	}

	stream := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfo, &stream)
	if err != nil {
		return nil, Meta{}, err
	}

	derivedKey, err := encryption.DeriveContentKey(path.Bucket(), path.UnencryptedPath(), s.encStore)
	if err != nil {
		return nil, Meta{}, err
	}

	var rangers []ranger.Ranger
	for i := int64(0); i < numberOfSegments(&stream, &streamMeta)-1; i++ {
		var contentNonce storj.Nonce
		_, err = encryption.Increment(&contentNonce, i+1)
		if err != nil {
			return nil, Meta{}, err
		}

		rangers = append(rangers, &lazySegmentRanger{
			segments:      s.segments,
			streamID:      object.StreamID,
			segmentIndex:  int32(i),
			rs:            object.RedundancyScheme,
			m:             streamMeta.LastSegmentMeta,
			size:          stream.SegmentsSize,
			derivedKey:    derivedKey,
			startingNonce: &contentNonce,
			encBlockSize:  int(streamMeta.EncryptionBlockSize),
			cipher:        storj.CipherSuite(streamMeta.EncryptionType),
		})
	}

	var contentNonce storj.Nonce
	_, err = encryption.Increment(&contentNonce, numberOfSegments(&stream, &streamMeta))
	if err != nil {
		return nil, Meta{}, err
	}

	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	decryptedLastSegmentRanger, err := decryptRanger(
		ctx,
		lastSegmentRanger,
		stream.LastSegmentSize,
		storj.CipherSuite(streamMeta.EncryptionType),
		derivedKey,
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
	meta = convertMeta(object.Modified, object.Expires, stream, streamMeta)
	return catRangers, meta, nil
}

// Meta implements Store.Meta
func (s *streamStore) Meta(ctx context.Context, path Path, pathCipher storj.CipherSuite) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryption.EncryptPath(path.Bucket(), path.UnencryptedPath(), pathCipher, s.encStore)
	if err != nil {
		return Meta{}, err
	}

	object, err := s.metainfo.GetObject(ctx, metainfo.GetObjectParams{
		Bucket:        []byte(path.Bucket()),
		EncryptedPath: []byte(encPath.Raw()),
	})

	streamInfo, streamMeta, err := TypedDecryptStreamInfo(ctx, object.Metadata, path, s.encStore)
	if err != nil {
		return Meta{}, err
	}

	var stream pb.StreamInfo
	if err := proto.Unmarshal(streamInfo, &stream); err != nil {
		return Meta{}, err
	}

	return convertMeta(object.Modified, object.Expires, stream, streamMeta), nil
}

// Delete all the segments, with the last one last
func (s *streamStore) Delete(ctx context.Context, path Path, pathCipher storj.CipherSuite) (err error) {
	defer mon.Task()(&ctx)(&err)

	encPath, err := encryption.EncryptPath(path.Bucket(), path.UnencryptedPath(), pathCipher, s.encStore)
	if err != nil {
		return err
	}

	// TODO do it in batch
	streamID, err := s.metainfo.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
		Bucket:        []byte(path.Bucket()),
		EncryptedPath: []byte(encPath.Raw()),
	})
	if err != nil {
		return err
	}

	// TODO handle `more`
	items, _, err := s.metainfo.ListSegmentsNew(ctx, metainfo.ListSegmentsParams{
		StreamID: streamID,
		CursorPosition: storj.SegmentPosition{
			Index: 0,
		},
	})
	if err != nil {
		return err
	}

	var errlist errs.Group
	for _, item := range items {
		err = s.segments.Delete(ctx, streamID, item.Position.Index)
		if err != nil {
			errlist.Add(err)
			continue
		}
	}

	return errlist.Err()
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     string
	Meta     Meta
	IsPrefix bool
}

// pathForKey removes the trailing `/` from the raw path, which is required so
// the derived key matches the final list path (which also has the trailing
// encrypted `/` part of the path removed)
func pathForKey(raw string) paths.Unencrypted {
	return paths.NewUnencrypted(strings.TrimSuffix(raw, "/"))
}

// List all the paths inside l/, stripping off the l/ prefix
func (s *streamStore) List(ctx context.Context, prefix Path, startAfter, endBefore string, pathCipher storj.CipherSuite, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO use flags with listing
	// if metaFlags&meta.Size != 0 {
	// Calculating the stream's size require also the user-defined metadata,
	// where stream store keeps info about the number of segments and their size.
	// metaFlags |= meta.UserDefined
	// }

	prefixKey, err := encryption.DerivePathKey(prefix.Bucket(), pathForKey(prefix.UnencryptedPath().Raw()), s.encStore)
	if err != nil {
		return nil, false, err
	}

	encPrefix, err := encryption.EncryptPath(prefix.Bucket(), prefix.UnencryptedPath(), pathCipher, s.encStore)
	if err != nil {
		return nil, false, err
	}

	// If the raw unencrypted path ends in a `/` we need to remove the final
	// section of the encrypted path. For example, if we are listing the path
	// `/bob/`, the encrypted path results in `enc("")/enc("bob")/enc("")`. This
	// is an incorrect list prefix, what we really want is `enc("")/enc("bob")`
	if strings.HasSuffix(prefix.UnencryptedPath().Raw(), "/") {
		lastSlashIdx := strings.LastIndex(encPrefix.Raw(), "/")
		encPrefix = paths.NewEncrypted(encPrefix.Raw()[:lastSlashIdx])
	}

	// We have to encrypt startAfter and endBefore but only if they don't contain a bucket.
	// They contain a bucket if and only if the prefix has no bucket. This is why they are raw
	// strings instead of a typed string: it's either a bucket or an unencrypted path component
	// and that isn't known at compile time.
	needsEncryption := prefix.Bucket() != ""
	if needsEncryption {
		startAfter, err = encryption.EncryptPathRaw(startAfter, pathCipher, prefixKey)
		if err != nil {
			return nil, false, err
		}
	}

	objects, more, err := s.metainfo.ListObjects(ctx, metainfo.ListObjectsParams{
		Bucket:          []byte(prefix.Bucket()),
		EncryptedPrefix: []byte(encPrefix.Raw()),
		EncryptedCursor: []byte(startAfter),
		Limit:           int32(limit),
		Recursive:       recursive,
	})
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(objects))
	for i, item := range objects {
		var path Path
		var itemPath string

		if needsEncryption {
			itemPath, err = encryption.DecryptPathRaw(string(item.EncryptedPath), pathCipher, prefixKey)
			if err != nil {
				return nil, false, err
			}

			// TODO(jeff): this shouldn't be necessary if we handled trailing slashes
			// appropriately. there's some issues with list.
			fullPath := prefix.UnencryptedPath().Raw()
			if len(fullPath) > 0 && fullPath[len(fullPath)-1] != '/' {
				fullPath += "/"
			}
			fullPath += itemPath

			path = CreatePath(prefix.Bucket(), paths.NewUnencrypted(fullPath))
		} else {
			itemPath = string(item.EncryptedPath)
			path = CreatePath(string(item.EncryptedPath), paths.Unencrypted{})
		}

		streamInfo, streamMeta, err := TypedDecryptStreamInfo(ctx, item.EncryptedMetadata, path, s.encStore)
		if err != nil {
			return nil, false, err
		}

		var stream pb.StreamInfo
		if err := proto.Unmarshal(streamInfo, &stream); err != nil {
			return nil, false, err
		}

		newMeta := convertMeta(item.CreatedAt, item.ExpiresAt, stream, streamMeta)
		items[i] = ListItem{
			Path:     itemPath,
			Meta:     newMeta,
			IsPrefix: item.IsPrefix,
		}
	}

	return items, more, nil
}

type lazySegmentRanger struct {
	ranger        ranger.Ranger
	segments      segments.Store
	streamID      storj.StreamID
	segmentIndex  int32
	rs            storj.RedundancyScheme
	m             *pb.SegmentMeta
	size          int64
	derivedKey    *storj.Key
	startingNonce *storj.Nonce
	encBlockSize  int
	cipher        storj.CipherSuite
}

// Size implements Ranger.Size
func (lr *lazySegmentRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected
func (lr *lazySegmentRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	if lr.ranger == nil {
		rr, encryption, err := lr.segments.Get(ctx, lr.streamID, lr.segmentIndex, lr.rs)
		if err != nil {
			return nil, err
		}

		encryptedKey, keyNonce := encryption.EncryptedKey, encryption.EncryptedKeyNonce
		lr.ranger, err = decryptRanger(ctx, rr, lr.size, lr.cipher, lr.derivedKey, encryptedKey, &keyNonce, lr.startingNonce, lr.encBlockSize)
		if err != nil {
			return nil, err
		}
	}
	return lr.ranger.Range(ctx, offset, length)
}

// decryptRanger returns a decrypted ranger of the given rr ranger
func decryptRanger(ctx context.Context, rr ranger.Ranger, decryptedSize int64, cipher storj.CipherSuite, derivedKey *storj.Key, encryptedKey storj.EncryptedPrivateKey, encryptedKeyNonce, startingNonce *storj.Nonce, encBlockSize int) (decrypted ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)
	contentKey, err := encryption.DecryptKey(encryptedKey, cipher, derivedKey, encryptedKeyNonce)
	if err != nil {
		return nil, err
	}

	decrypter, err := encryption.NewDecrypter(cipher, contentKey, startingNonce, encBlockSize)
	if err != nil {
		return nil, err
	}

	var rd ranger.Ranger
	if rr.Size()%int64(decrypter.InBlockSize()) != 0 {
		reader, err := rr.Range(ctx, 0, rr.Size())
		if err != nil {
			return nil, err
		}
		defer func() { err = errs.Combine(err, reader.Close()) }()
		cipherData, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		data, err := encryption.Decrypt(cipherData, cipher, contentKey, startingNonce)
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

// CancelHandler handles clean up of segments on receiving CTRL+C
func (s *streamStore) cancelHandler(ctx context.Context, streamID storj.StreamID, totalSegments int64, path Path, pathCipher storj.CipherSuite) {
	defer mon.Task()(&ctx)(nil)

	for i := int64(0); i < totalSegments; i++ {
		err := s.segments.Delete(ctx, streamID, int32(i))
		if err != nil {
			zap.L().Warn("Failed deleting segment", zap.String("path", path.String()), zap.Int64("segmentIndex", i), zap.Error(err))
			continue
		}
	}
}

func getEncryptedKeyAndNonce(m *pb.SegmentMeta) (storj.EncryptedPrivateKey, *storj.Nonce) {
	if m == nil {
		return nil, nil
	}

	var nonce storj.Nonce
	copy(nonce[:], m.KeyNonce)

	return m.EncryptedKey, &nonce
}

// TypedDecryptStreamInfo decrypts stream info
func TypedDecryptStreamInfo(ctx context.Context, streamMetaBytes []byte, path Path, encStore *encryption.Store) (
	streamInfo []byte, streamMeta pb.StreamMeta, err error) {
	defer mon.Task()(&ctx)(&err)

	err = proto.Unmarshal(streamMetaBytes, &streamMeta)
	if err != nil {
		return nil, pb.StreamMeta{}, err
	}

	derivedKey, err := encryption.DeriveContentKey(path.Bucket(), path.UnencryptedPath(), encStore)
	if err != nil {
		return nil, pb.StreamMeta{}, err
	}

	cipher := storj.CipherSuite(streamMeta.EncryptionType)
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	contentKey, err := encryption.DecryptKey(encryptedKey, cipher, derivedKey, keyNonce)
	if err != nil {
		return nil, pb.StreamMeta{}, err
	}

	// decrypt metadata with the content encryption key and zero nonce
	streamInfo, err = encryption.Decrypt(streamMeta.EncryptedStreamInfo, cipher, contentKey, &storj.Nonce{})
	return streamInfo, streamMeta, err
}
