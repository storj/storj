// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/metainfo"
)

var mon = monkit.Package()

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket string, inMeta Meta) (meta Meta, err error)
	Delete(ctx context.Context, bucket string) (err error)
	List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error)
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
	// pointer -> segment -> stream -> object -> bucket
	return convertMeta(ctx, convertPointer(pointer))
}

// Put calls objects store Put and fills in some specific metadata to be used
// in the bucket's object Pointer. Note that the Meta.Created field is ignored.
func (b *BucketStore) Put(ctx context.Context, bucketName string, inMeta Meta) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return meta, storj.ErrNoBucket.New("")
	}

	_, err = b.Get(ctx, bucketName)
	if err == nil {
		return meta, errs.New("Bucket already exists, cannot create bucket")
	}

	pathCipher := inMeta.PathEncryptionType
	if pathCipher < storj.Unencrypted || pathCipher > storj.SecretBox {
		return meta, encryption.ErrInvalidConfig.New("encryption type %d is not supported", pathCipher)
	}

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

	pointer, err := userMetaToPointer(userMeta)
	if err != nil {
		return meta, err
	}
	path := storj.JoinPaths("l", bucketName)
	p, err := b.metainfo.CommitSegment(ctx, bucketName, path, 0, pointer, nil)

	if err != nil {
		return Meta{}, err
	}

	meta = Meta{
		Created:            convertTime(p.CreationDate),
		PathEncryptionType: storj.Unencrypted,
		SegmentsSize:       p.SegmentSize,
		RedundancyScheme:   storj.RedundancyScheme{},
		EncryptionScheme:   storj.EncryptionScheme{Cipher: storj.Unencrypted},
	}
	return meta, nil
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
			NumberOfSegments: 1, // is this the correct value?
			SegmentsSize:     objItem.Meta.Size,
			LastSegmentSize:  objItem.Meta.Size,
			Metadata:         objItem.Meta.Data,
		}

		object := objects.ConvertMeta(streams.ConvertMeta(objItem.Meta, streamInfo, pb.StreamMeta{}))

		m, err := convertMeta(ctx, object)
		if err != nil {
			return nil, false, err
		}
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

// convertTime converts gRPC timestamp to Go time
func convertTime(ts *timestamp.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	t, err := ptypes.Timestamp(ts)
	if err != nil {
		zap.S().Warnf("Failed converting timestamp %v: %v", ts, err)
	}
	return t
}

// helper method for Get function
// TODO: This conversion from pointer to bucket Meta object would be simplified if there were a metainfo call available like b.metainfo.BucketInfo(), but currently that doesn't exist.
// However, it does seem like that change is captured in the metainfo refactor.
// When the refactor is done, we can replace this method.
func convertPointer(pointer *pb.Pointer) objects.Meta {
	segment := segments.ConvertMeta(pointer)

	streamInfo := pb.StreamInfo{
		NumberOfSegments: 0,
		SegmentsSize:     segment.Size,
		LastSegmentSize:  segment.Size,
		Metadata:         segment.Data,
	}

	streamMeta := pb.StreamMeta{}

	return objects.ConvertMeta(streams.ConvertMeta(segment, streamInfo, streamMeta))
}

// helper method for Put function
func userMetaToPointer(userMeta map[string]string) (pointer *pb.Pointer, err error) {

	metadata, err := proto.Marshal(&pb.SerializableMeta{UserDefined: userMeta})
	if err != nil {
		return &pb.Pointer{}, err
	}

	streamInfo, err := proto.Marshal(&pb.StreamInfo{
		NumberOfSegments: 1,
		SegmentsSize:     0,
		LastSegmentSize:  0,
		Metadata:         metadata,
	})
	if err != nil {
		return &pb.Pointer{}, err
	}

	streamMeta, err := proto.Marshal(&pb.StreamMeta{
		EncryptedStreamInfo: streamInfo,
		EncryptionType:      int32(storj.Unencrypted),
		EncryptionBlockSize: 0,
	})

	var exp timestamp.Timestamp
	pointer = &pb.Pointer{
		Type:           pb.Pointer_INLINE,
		InlineSegment:  nil,
		SegmentSize:    0,
		ExpirationDate: &exp,
		Metadata:       streamMeta,
	}
	return pointer, nil
}
