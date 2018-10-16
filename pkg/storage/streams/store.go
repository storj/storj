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
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	logger.Debug("entering STREAMSTORE META -1\n\n\n\n\n")

	// streamMeta := pb.StreamMeta{}
	// err := proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	// if err != nil {
	// 	logger.Debug("entering STREAMSTORE META -2\n\n\n\n\n")
	// 	return Meta{}, err
	// }

	// logger.Debug("entering STREAMSTORE META -3\n\n\n\n\n")
	// // TODO decrypt before unmarshalling
	// stream := pb.StreamInfo{}
	// err = proto.Unmarshal(streamMeta.EncryptedStreamInfo, &stream)
	// if err != nil {
	// 	logger.Debug("entering STREAMSTORE META -4\n\n\n\n\n")
	// 	return Meta{}, err
	// }


	stream := pb.StreamInfo{}
	err := proto.Unmarshal(lastSegmentMeta.Data, &stream)
	if err != nil {
		logger.Debug("entering STREAMSTORE META -4\n\n\n\n\n")
		return Meta{}, err
	}

	logger.Debug("entering STREAMSTORE META -5\n\n\n\n\n")
	fmt.Println("this is stream in convertMeta: ", stream)
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
func (s *streamStore) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
 	logger.Debug("entering put streamstore\n\n\n\n\n")

	// previously file uploaded?
	err = s.Delete(ctx, path)
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		//something wrong happened checking for an existing
		//file with the same name
		return Meta{}, err
	}

	logger.Debug("streamstore put-2")
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

	logger.Debug("streamstore put-3")
	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		logger.Debug("streamstore put-4")
		return Meta{}, err
	}

	logger.Debug("streamstore put-5")
	cipher := s.encType

	eofReader := NewEOFReader(data)

	for !eofReader.isEOF() && !eofReader.hasError() {
		var encKey eestream.Key
		_, err = rand.Read(encKey[:])
		if err != nil {
			logger.Debug("streamstore put-6")
			return Meta{}, err
		}

		logger.Debug("streamstore put-7")
		var nonce eestream.Nonce
		_, err := nonce.Increment(currentSegment)
		if err != nil {
			logger.Debug("streamstore put-8")
			return Meta{}, err
		}

		logger.Debug("streamstore put-9")
		encrypter, err := cipher.NewEncrypter(&encKey, &nonce, s.encBlockSize)
		if err != nil {
			logger.Debug("streamstore put-10")
			return Meta{}, err
		}

		logger.Debug("streamstore put-11")
		// generate random nonce for encrypting the encryption key
		var keyNonce eestream.Nonce
		_, err = rand.Read(keyNonce[:])
		if err != nil {
			logger.Debug("streamstore put-12")
			return Meta{}, err
		}

		logger.Debug("streamstore put-13")
		encryptedEncKey, err := cipher.Encrypt(encKey[:], (*eestream.Key)(derivedKey), &keyNonce)
		if err != nil {
			logger.Debug("streamstore put-14")
			return Meta{}, err
		}	

		logger.Debug("streamstore put-15")

		sizeReader := NewSizeReader(eofReader)
		segmentReader := io.LimitReader(sizeReader, s.segmentSize)
		peekReader := segments.NewPeekThresholdReader(segmentReader)
		largeData, err := peekReader.IsLargerThan(encrypter.InBlockSize())
		if err != nil {
			logger.Debug("streamstore put-16")
			return Meta{}, err
		}
		logger.Debug("streamstore put-17")
		var transformedReader io.Reader
		if largeData {
			logger.Debug("streamstore put-18")
			paddedReader := eestream.PadReader(ioutil.NopCloser(peekReader), encrypter.InBlockSize())
			transformedReader = eestream.TransformReader(paddedReader, encrypter, 0)
		} else {
			logger.Debug("streamstore put-19")
			data, err := ioutil.ReadAll(peekReader)
			if err != nil {
				logger.Debug("streamstore put-20")
				return Meta{}, err
			}
			logger.Debug("streamstore put-21")
			cipherData, err := cipher.Encrypt(data, &encKey, &nonce)
			if err != nil {
				logger.Debug("streamstore put 22")
				return Meta{}, err
			}
			logger.Debug("streamstore put-23")
			transformedReader = bytes.NewReader(cipherData)
		}

		logger.Debug("streamstore put-24")
		putMeta, err = s.segments.Put(ctx, transformedReader, expiration, func() (paths.Path, []byte, error) {
			encPath, err := encryptAfterBucket(path, s.rootKey)
			if err != nil {
				logger.Debug("streamstore put-25")
				return nil, nil, err
			}

			logger.Debug("streamstore put-26")
			if !eofReader.isEOF() {
				logger.Debug("streamstore put-27")
				segmentPath := getSegmentPath(encPath, currentSegment)

				if cipher == eestream.None {
					logger.Debug("streamstore put-28")
					return segmentPath, nil, nil
				}
				logger.Debug("streamstore put-29")
				segmentMeta, err := proto.Marshal(&pb.SegmentMeta{
					EncryptedKey:      encryptedEncKey,
					EncryptedKeyNonce: keyNonce[:],
				})
				if err != nil {
					logger.Debug("streamstore put-30")
					return nil, nil, err
				}
				logger.Debug("streamstore put-31")
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
				logger.Debug("streamstore put-32")
				return nil, nil, err
			}



			logger.Debug("stream info-put-1- before encrypting")


			// encrypt streaminfo -- do i need a new nonce?
			encryptedStreamInfo, err := cipher.Encrypt(streamInfo, (*eestream.Key)(derivedKey), &nonce)
			if err != nil {
				logger.Debug("stream info-put-aaaa- after encrypting")
				return nil, nil, err
			}

			logger.Debug("stream info-put-2- after encrypting")



			streamMeta := pb.StreamMeta{
				EncryptedStreamInfo: encryptedStreamInfo, //encrypted streamInfo
				EncryptionType:      int32(s.encType),
				EncryptionBlockSize: int32(s.encBlockSize),
			}

			if cipher != eestream.None {
				streamMeta.LastSegmentMeta = &pb.SegmentMeta{
					EncryptedKey:      encryptedEncKey,
					EncryptedKeyNonce: keyNonce[:],
				}
				logger.Debug("stream info-put-34")
			}

			lastSegmentMeta, err := proto.Marshal(&streamMeta)
			if err != nil {
				logger.Debug("stream info-put-35")
				return nil, nil, err
			}
			logger.Debug("stream info-put-36")
			return lastSegmentPath, lastSegmentMeta, nil
		}) //end seg put
		logger.Debug("stream info-put-37")
		if err != nil {
			logger.Debug("stream info-put-38")
			return Meta{}, err
		}
		logger.Debug("stream info-put-39")
		currentSegment++
		streamSize += sizeReader.Size()
	}
	logger.Debug("stream info-put-40")
	if eofReader.hasError() {
		logger.Debug("stream info-put-41")
		return Meta{}, eofReader.err
	}
	logger.Debug("stream info-put-42")
	resultMeta := Meta{
		Modified:   putMeta.Modified,
		Expiration: expiration,
		Size:       streamSize,
		Data:       metadata,
	}

	logger.Debug("stream info-put-43")
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

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Debug("in stream get -1")

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		logger.Debug("in stream get -2")
		return nil, Meta{}, err
	}
	logger.Debug("in stream get -3")
	lastSegmentRanger, lastSegmentMeta, err := s.segments.Get(ctx, encPath.Prepend("l"))
	if err != nil {
		logger.Debug("in stream get -4")
		return nil, Meta{}, err
	}

	logger.Debug("in stream get -5")
	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		logger.Debug("in stream get -6")
		return nil, Meta{}, err
	}




	fmt.Println("last segmentmeta.data: ", lastSegmentMeta.Data)

	logger.Debug("in stream get -7")
	cipher := eestream.Cipher(streamMeta.EncryptionType)
	var nonce eestream.Nonce

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		logger.Debug("get streamstore err in derived key")
		return nil,Meta{}, err
	}



	fmt.Println("last segment: ", lastSegmentMeta, "\n\n\n\n\n\n")
	fmt.Println("this is the cipher: ", cipher, "\n\n\n\n\n")
	
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	e, err := cipher.Decrypt(encryptedKey, (*eestream.Key)(derivedKey), keyNonce)
	if err != nil {
		logger.Debug("err in derypted enc key streamstore get")
		return nil, Meta{}, err
	}

	var encKey eestream.Key
	copy(encKey[:], e)

	fmt.Println("enckey: ", encKey, "\n\n\n\n\n\n\n\n")
	fmt.Println("enctype: ", streamMeta.EncryptionType, "\n\n\n\n\n\n")
	fmt.Println("nonce: ", &nonce, "\n\n\n\n\n\n\n\n")
	fmt.Println("stream meta encrypted streamninfo: ", streamMeta.EncryptedStreamInfo, "\n\n\n\n\n")

	decryptedStreamInfo, err := cipher.Decrypt(streamMeta.EncryptedStreamInfo, (*eestream.Key)(derivedKey), &nonce)
	if err != nil {
		fmt.Println("error in decrypting the stream info")
		return nil, Meta{}, err
	}
	// TODO decrypt before umarshalling
	// get encryption key and decrypt it
	// get enc type from pb.StreamMeta
	// decrypt stramMeata.EncryptedStreamInfo 


	fmt.Println("decryptedStreamInfo: ", decryptedStreamInfo, "\n\n\n\n\n\n\n\n")

	stream := pb.StreamInfo{}
	err = proto.Unmarshal(decryptedStreamInfo, &stream) //streamMeta.EncryptedStreamInfo
	if err != nil {
		logger.Debug("in stream get -8")
		return nil, Meta{}, err
	}

	//fmt.Println("streams are: ", stream)

	//fmt.Println("stream segments: ", stream.NumberOfSegments, "\n\n\n\n\n\n\n")
	//logger.Debug("in stream get -9")
	//derivedKey, err := path.DeriveContentKey(s.rootKey)
	// if err != nil {
	// 	logger.Debug("in stream get -10")
	// 	return nil, Meta{}, err
	// }


	decryptedLastSegmentMetaData := segments.Meta{
		Modified: lastSegmentMeta.Modified,
		Expiration: lastSegmentMeta.Expiration,
		Size: lastSegmentMeta.Size,
		Data: decryptedStreamInfo,
	}

	logger.Debug("in stream get -11")
	var rangers []ranger.Ranger
	for i := int64(0); i < stream.NumberOfSegments-1; i++ {
		currentPath := getSegmentPath(encPath, i)
		size := stream.SegmentsSize
		var nonce eestream.Nonce
		_, err := nonce.Increment(i)
		if err != nil {
			logger.Debug("in stream get -12")
			return nil, Meta{}, err
		}
		rr := &lazySegmentRanger{
			segments:      s.segments,
			path:          currentPath,
			size:          size,
			derivedKey:    (*eestream.Key)(derivedKey),
			startingNonce: &nonce,
			encBlockSize:  int(streamMeta.EncryptionBlockSize),
			encType:       eestream.Cipher(streamMeta.EncryptionType),
		}
		logger.Debug("in stream get -13")
		rangers = append(rangers, rr)
	}

	//var nonce eestream.Nonce
	i, err := nonce.Increment(stream.NumberOfSegments - 1)
	if err != nil {
		logger.Debug("in stream get -14")
		fmt.Println("this is i: ", i)
		return nil, Meta{}, err
	}
	//encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	decryptedLastSegmentRanger, err := decryptRanger(
		ctx,
		lastSegmentRanger,
		stream.LastSegmentSize,
		eestream.Cipher(streamMeta.EncryptionType),
		(*eestream.Key)(derivedKey),
		encryptedKey,
		keyNonce,
		&nonce,
		int(streamMeta.EncryptionBlockSize),
	)
	if err != nil {
		logger.Debug("in stream get -15")
		return nil, Meta{}, err
	}
	logger.Debug("in stream get -16")
	rangers = append(rangers, decryptedLastSegmentRanger)

	catRangers := ranger.Concat(rangers...)









	meta, err = convertMeta(decryptedLastSegmentMetaData) //lastSegmentMeta
	if err != nil {
		logger.Debug("in stream get -17")
		return nil, Meta{}, err
	}

	logger.Debug("in stream get -18")
	return catRangers, meta, nil
}

// Meta implements Store.Meta
func (s *streamStore) Meta(ctx context.Context, path paths.Path) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Debug("entering streamstore s.Meta")

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		return Meta{}, err
	}






	lastSegmentMeta, err := s.segments.Meta(ctx, encPath.Prepend("l"))
	if err != nil {
		return Meta{}, err
	}

	logger.Debug("streamstore s.meta - 1")
	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		logger.Debug("entering s.STREAMSTORE META -2\n\n\n\n\n")
		return Meta{}, err
	}


	logger.Debug("in stream s.META -XXXXXX\n\n\n\n\n\n")
	cipher := eestream.Cipher(streamMeta.EncryptionType)
	var nonce eestream.Nonce

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return Meta{}, err
	}

	fmt.Println("last segment s.m : ", lastSegmentMeta, "\n\n\n\n\n\n")
	fmt.Println("this is the cipher: ", cipher, "\n\n\n\n\n")
	
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	e, err := cipher.Decrypt(encryptedKey, (*eestream.Key)(derivedKey), keyNonce)
	if err != nil {
		return Meta{}, err
	}

	var encKey eestream.Key
	copy(encKey[:], e)

	decryptedStreamInfo, err := cipher.Decrypt(streamMeta.EncryptedStreamInfo, (*eestream.Key)(derivedKey), &nonce)
	if err != nil {
		logger.Debug("error in decrypting data")
		return Meta{}, err
	}

	logger.Debug("s.meta- 9")
	decryptedLastSegmentMetaData := segments.Meta{
		Modified: lastSegmentMeta.Modified,
		Expiration: lastSegmentMeta.Expiration,
		Size: lastSegmentMeta.Size,
		Data: decryptedStreamInfo,
	}




	logger.Debug("s.meta- 10")
	streamMetaFinal, err := convertMeta(decryptedLastSegmentMetaData) // lastSegmentMeta
	if err != nil {
		logger.Debug("s.meta- 11")
		return Meta{}, err
	}

	logger.Debug("s.meta- 12")
	return streamMetaFinal, nil
}

// Delete all the segments, with the last one last
func (s *streamStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)


	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	encPath, err := encryptAfterBucket(path, s.rootKey)
	if err != nil {
		return err
	}
	lastSegmentMeta, err := s.segments.Meta(ctx, encPath.Prepend("l"))
	if err != nil {
		return err
	}

	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		return err
	}






	cipher := eestream.Cipher(streamMeta.EncryptionType)
	var nonce eestream.Nonce

	derivedKey, err := path.DeriveContentKey(s.rootKey)
	if err != nil {
		return err
	}

	fmt.Println("last segment delete seg  : ", lastSegmentMeta, "\n\n\n\n\n\n")
	fmt.Println("this is the cipher delete seg: ", cipher, "\n\n\n\n\n")
	
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	e, err := cipher.Decrypt(encryptedKey, (*eestream.Key)(derivedKey), keyNonce)
	if err != nil {
		return err
	}

	var encKey eestream.Key
	copy(encKey[:], e)

	decryptedStreamInfo, err := cipher.Decrypt(streamMeta.EncryptedStreamInfo, (*eestream.Key)(derivedKey), &nonce)
	if err != nil {
		logger.Debug("error in decrypting data")
		return err
	}

	logger.Debug("delete seg - 9 \n\n\n\n\n\n")
	fmt.Println("decrypted info in delete: ", decryptedStreamInfo, "\n\n\n\n\n\n")



	stream := pb.StreamInfo{}
	err = proto.Unmarshal(decryptedStreamInfo, &stream)
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

	allSegments, more, err := s.segments.List(ctx, encPrefix.Prepend("l"), encStartAfter, encEndBefore, recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(allSegments))
	for i, item := range allSegments {






		//added wholepath
		wholePath := prefix.Append([]string(item.Path)...)


		streamMeta := pb.StreamMeta{}
		err := proto.Unmarshal(item.Meta.Data, &streamMeta)
		if err != nil {
			fmt.Println("entering STREAMSTORE META -2\n\n\n\n\n")
			return nil, false, err
		}


	cipher := eestream.Cipher(streamMeta.EncryptionType)
	var nonce eestream.Nonce

	derivedKey, err := wholePath.DeriveContentKey(s.rootKey)
	if err != nil {
		return nil, false, err
	}

	//fmt.Println("last segment delete seg  : ", lastSegmentMeta, "\n\n\n\n\n\n")
	//fmt.Println("this is the cipher delete seg: ", cipher, "\n\n\n\n\n")
	
	encryptedKey, keyNonce := getEncryptedKeyAndNonce(streamMeta.LastSegmentMeta)
	e, err := cipher.Decrypt(encryptedKey, (*eestream.Key)(derivedKey), keyNonce)
	if err != nil {
		return nil, false, err
	}

	var encKey eestream.Key
	copy(encKey[:], e)

	decryptedStreamInfo, err := cipher.Decrypt(streamMeta.EncryptedStreamInfo, (*eestream.Key)(derivedKey), &nonce)
	if err != nil {
		//logger.Debug("error in decrypting data")
		return nil, false,  err
	}


	decryptedLastSegmentMetaData := segments.Meta{
		Modified: item.Meta.Modified,
		Expiration: item.Meta.Expiration,
		Size: item.Meta.Size,
		Data: decryptedStreamInfo,
	}











		newMeta, err := convertMeta(decryptedLastSegmentMetaData)
		if err != nil {
			return nil, false, err
		}
		decPath, err := s.decryptMarker(item.Path, prefixKey)
		if err != nil {
			return nil, false, err
		}
		items[i] = ListItem{Path: decPath, Meta: newMeta, IsPrefix: item.IsPrefix}
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
func decryptRanger(ctx context.Context, rr ranger.Ranger, decryptedSize int64, cipher eestream.Cipher, derivedKey *eestream.Key, encryptedKey []byte, encryptedKeyNonce, startingNonce *eestream.Nonce, encBlockSize int) (ranger.Ranger, error) {
	e, err := cipher.Decrypt(encryptedKey, derivedKey, encryptedKeyNonce)
	if err != nil {
		return nil, err
	}
	var encKey eestream.Key
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

	rd, err = eestream.Transform(rr, decrypter)
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

func getEncryptedKeyAndNonce(m *pb.SegmentMeta) ([]byte, *eestream.Nonce) {
	if m == nil {
		return nil, nil
	}

	var nonce eestream.Nonce
	copy(nonce[:], m.EncryptedKeyNonce)

	return m.EncryptedKey, &nonce
}