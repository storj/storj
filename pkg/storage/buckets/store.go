// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/metainfo"
)

var mon = monkit.Package()

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket string, inMeta Meta) (meta Meta, err error)
	Delete(ctx context.Context, bucket string) (err error)
	List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error)
	GetObjectStore(ctx context.Context, bucketName string) (store objects.Store, err error)
}

// ListItem is a single item in a listing
type ListItem struct {
	Bucket string
	Meta   Meta
}

// BucketStore contains objects store
type BucketStore struct {
	metainfo metainfo.Client
}

// Meta is the bucket metadata struct
type Meta struct {
	Created            time.Time
	PathEncryptionType storj.Cipher
	SegmentsSize       int64
	RedundancyScheme   storj.RedundancyScheme
	EncryptionScheme   storj.EncryptionScheme
}

// NewStore instantiates BucketStore
func NewStore(metainfo metainfo.Client) Store {
	return &BucketStore{metainfo: metainfo}
}

// GetObjectStore returns an implementation of objects.Store
func (b *BucketStore) GetObjectStore(ctx context.Context, bucket string) (_ objects.Store, err error) {
	defer mon.Task()(&ctx)(&err)
	if bucket == "" {
		return nil, storj.ErrNoBucket.New("")
	}

	m, err := b.Get(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return nil, err
	}
	//--- UPDATE
	prefixed := prefixedObjStore{
		store:  objects.NewStore(b.stream, m.PathEncryptionType),
		prefix: bucket,
	}
	return &prefixed, nil
	//---
}

// Get calls objects store Get
func (b *BucketStore) Get(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	bb, objectPath, segmentIndex, err := segments.SplitPathFragments(bucket)
	if err != nil {
		return Meta{}, err
	}

	pointer, err := b.metainfo.SegmentInfo(ctx, bb, objectPath, segmentIndex)
	if err != nil {
		return Meta{}, err
	}

	// metadata conversion function call: pointer -> segment -> stream -> object -> bucket
	segment := segments.ConvertMeta(pointer)

	streamInfo := pb.StreamInfo{
		NumberOfSegments: 1,
		SegmentsSize:     segment.Size,
		LastSegmentSize:  segment.Size,
		Metadata:         segment.Data,
	}
	streamMeta := pb.StreamMeta{}

	object := objects.ConvertMeta(streams.ConvertMeta(segment, streamInfo, streamMeta))
	return convertMeta(object)
}

// Put calls objects store Put and fills in some specific metadata to be used
// in the bucket's object Pointer. Note that the Meta.Created field is ignored.
func (b *BucketStore) Put(ctx context.Context, bucketName string, inMeta Meta) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	pathCipher := inMeta.PathEncryptionType
	if pathCipher < storj.Unencrypted || pathCipher > storj.SecretBox {
		return Meta{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", pathCipher)
	}

	// r := bytes.NewReader(nil)
	userMeta := map[string]string{
		"path-enc-type":     strconv.Itoa(int(pathCipher)),
		"default-seg-size":  strconv.FormatInt(inMeta.SegmentsSize, 10),
		"default-enc-type":  strconv.Itoa(int(inMeta.EncryptionScheme.Cipher)),
		"default-enc-blksz": strconv.FormatInt(int64(inMeta.EncryptionScheme.BlockSize), 10),
		"default-rs-algo":   strconv.Itoa(int(inMeta.RedundancyScheme.Algorithm)),
		"default-rs-sharsz": strconv.FormatInt(int64(inMeta.RedundancyScheme.ShareSize), 10),
		"default-rs-reqd":   strconv.Itoa(int(inMeta.RedundancyScheme.RequiredShares)),
		"default-rs-repair": strconv.Itoa(int(inMeta.RedundancyScheme.RepairShares)),
		"default-rs-optim":  strconv.Itoa(int(inMeta.RedundancyScheme.OptimalShares)),
		"default-rs-total":  strconv.Itoa(int(inMeta.RedundancyScheme.TotalShares)),
	}
	var exp time.Time
	//--- UPDATE
	// m, err := b.store.Put(ctx, bucketName, r, pb.SerializableMeta{UserDefined: userMeta}, exp)
	m, err := b.Get(ctx, bucketName)
	if err == nil {
		//bucket exists, add meta to existing entry
	}


	//*** - prefix the bucket path with `l/` before storing in pointerdb
	// lastSegmentPath := storj.JoinPaths("l", encPath)

	//***- populate and marshal a pb.StreamInfo protobuf

	// streamInfo, err := proto.Marshal(&pb.StreamInfo{
	// 	NumberOfSegments: currentSegment + 1,
	// 	SegmentsSize:     s.segmentSize,
	// 	LastSegmentSize:  sizeReader.Size(),
	// 	Metadata:         metadata,
	// })

	// *** - populate and marshal a pb.StreamMeta protobuf (the marshaled pb.StreamInfo goes in encrypted_stream_info)
	// streamMeta := pb.StreamMeta{
	// 	EncryptedStreamInfo: encryptedStreamInfo,
	// 	EncryptionType:      int32(s.cipher),
	// 	EncryptionBlockSize: int32(s.encBlockSize),
	// }

	// ***- populate and marshal a pb.Pointer protobuf
	// pointer = &pb.Pointer{
	// 	Type:           pb.Pointer_INLINE,
	// 	InlineSegment:  peekReader.thresholdBuf,
	// 	SegmentSize:    int64(len(peekReader.thresholdBuf)),
	// 	ExpirationDate: exp,
	// 	Metadata:       metadata,
	// }

	// ***- call metainfoClient.CommitSegment()

	// ***- fill in a new storj.Bucket instance (the Created member should come from the CreationDate attribute in the protobuf resulting from that CommitSegment() call; all the other fields should be handy)
	// type Bucket struct {
	// 	Name                 string
	// 	Created              time.Time
	// 	PathCipher           Cipher
	// 	SegmentsSize         int64
	// 	RedundancyScheme     RedundancyScheme
	// 	EncryptionParameters EncryptionParameters
	// }

	//---
	if err != nil {
		return Meta{}, err
	}
	// we could use convertMeta() here, but that's a lot of int-parsing
	// just to get back to what should be the same contents we already
	// have. the only change ought to be the modified time.
	inMeta.Created = m.Modified
	return inMeta, nil
}

// Delete calls objects store Delete
func (b *BucketStore) Delete(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.ErrNoBucket.New("")
	}

	bb, objectPath, segmentIndex, err := segments.SplitPathFragments(bucket)
	if err != nil {
		return err
	}

	limits, err := b.metainfo.DeleteSegment(ctx, bb, objectPath, segmentIndex)
	if err != nil {
		return err
	}

	if len(limits) != 0 {
		return errs.New("bucket segment should be inline, something went wrong")
	}

	return nil
}

// List calls objects store List
func (b *BucketStore) List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	// objItems, more, err := b.store.List(ctx, "", startAfter, endBefore, false, limit, meta.Modified)

	bb, strippedPrefix, _, err := segments.SplitPathFragments("")
	if err != nil {
		return nil, false, err
	}

	list, more, err := b.metainfo.ListSegments(ctx, bb, strippedPrefix, startAfter, endBefore, false, int32(limit), meta.Modified)
	if err != nil {
		return nil, false, err
	}

	objItems := make([]segments.ListItem, len(list))
	for i, itm := range list {
		objItems[i] = segments.ListItem{
			Path:     itm.Path,
			Meta:     segments.ConvertMeta(itm.Pointer),
			IsPrefix: itm.IsPrefix,
		}
	}

	items = make([]ListItem, 0, len(objItems))
	for _, objItem := range objItems {
		if objItem.IsPrefix {
			continue
		}
		streamInfo := pb.StreamInfo{
			NumberOfSegments: 1,
			SegmentsSize:     objItem.Meta.Size,
			LastSegmentSize:  objItem.Meta.Size,
			Metadata:         objItem.Meta.Data,
		}

		object := objects.ConvertMeta(streams.ConvertMeta(objItem.Meta, streamInfo, pb.StreamMeta{}))

		m, err := convertMeta(object)

		items = append(items, ListItem{
			Bucket: objItem.Path,
			Meta:   m,
		})
	}
	return items, more, nil
}

// convertMeta converts stream metadata to object metadata
func convertMeta(ctx context.Context, m objects.Meta) (out Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	out.Created = m.Modified
	// backwards compatibility for old buckets
	out.PathEncryptionType = storj.AESGCM
	out.EncryptionScheme.Cipher = storj.Invalid

	applySetting := func(nameInMap string, bits int, storeFunc func(val int64)) {
		if err != nil {
			return
		}
		if stringVal := m.UserDefined[nameInMap]; stringVal != "" {
			var intVal int64
			intVal, err = strconv.ParseInt(stringVal, 10, bits)
			if err != nil {
				err = errs.New("invalid metadata field for %s: %v", nameInMap, err)
				return
			}
			storeFunc(intVal)
		}
	}

	es := &out.EncryptionScheme
	rs := &out.RedundancyScheme

	applySetting("path-enc-type", 16, func(v int64) { out.PathEncryptionType = storj.Cipher(v) })
	applySetting("default-seg-size", 64, func(v int64) { out.SegmentsSize = v })
	applySetting("default-enc-type", 32, func(v int64) { es.Cipher = storj.Cipher(v) })
	applySetting("default-enc-blksz", 32, func(v int64) { es.BlockSize = int32(v) })
	applySetting("default-rs-algo", 32, func(v int64) { rs.Algorithm = storj.RedundancyAlgorithm(v) })
	applySetting("default-rs-sharsz", 32, func(v int64) { rs.ShareSize = int32(v) })
	applySetting("default-rs-reqd", 16, func(v int64) { rs.RequiredShares = int16(v) })
	applySetting("default-rs-repair", 16, func(v int64) { rs.RepairShares = int16(v) })
	applySetting("default-rs-optim", 16, func(v int64) { rs.OptimalShares = int16(v) })
	applySetting("default-rs-total", 16, func(v int64) { rs.TotalShares = int16(v) })

	return out, err
}
