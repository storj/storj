// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/vivint/infectious"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	// commitedPrefix is prefix where completed object info is stored
	committedPrefix = "l/"
)

// DefaultRS default values for RedundancyScheme
var DefaultRS = storj.RedundancyScheme{
	Algorithm:      storj.ReedSolomon,
	RequiredShares: 20,
	RepairShares:   30,
	OptimalShares:  40,
	TotalShares:    50,
	ShareSize:      1 * memory.KB.Int32(),
}

// DefaultES default values for EncryptionScheme
var DefaultES = storj.EncryptionScheme{
	Cipher:    storj.AESGCM,
	BlockSize: 1 * memory.KB.Int32(),
}

// GetObject returns information about an object
func (db *DB) GetObject(ctx context.Context, bucket string, path storj.Path) (info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	_, info, err = db.getInfo(ctx, committedPrefix, bucket, path)

	return info, err
}

// GetObjectStream returns interface for reading the object stream
func (db *DB) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (stream storj.ReadOnlyStream, err error) {
	defer mon.Task()(&ctx)(&err)

	meta, info, err := db.getInfo(ctx, committedPrefix, bucket, path)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(meta.fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	return &readonlyStream{
		db:            db,
		info:          info,
		encryptedPath: meta.encryptedPath,
		streamKey:     streamKey,
	}, nil
}

// CreateObject creates an uploading object and returns an interface for uploading Object information
func (db *DB) CreateObject(ctx context.Context, bucket string, path storj.Path, createInfo *storj.CreateObject) (object storj.MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, err := db.GetBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if path == "" {
		return nil, storj.ErrNoPath.New("")
	}

	info := storj.Object{
		Bucket: bucketInfo,
		Path:   path,
	}

	if createInfo != nil {
		info.Metadata = createInfo.Metadata
		info.ContentType = createInfo.ContentType
		info.Expires = createInfo.Expires
		info.RedundancyScheme = createInfo.RedundancyScheme
		info.EncryptionScheme = createInfo.EncryptionScheme
	}

	// TODO: autodetect content type from the path extension
	// if info.ContentType == "" {}

	if info.RedundancyScheme.IsZero() {
		info.RedundancyScheme = DefaultRS
	}

	if info.EncryptionScheme.IsZero() {
		info.EncryptionScheme = storj.EncryptionScheme{
			Cipher:    DefaultES.Cipher,
			BlockSize: info.RedundancyScheme.ShareSize,
		}
	}

	return &mutableObject{
		db:   db,
		info: info,
	}, nil
}

// ModifyObject modifies a committed object
func (db *DB) ModifyObject(ctx context.Context, bucket string, path storj.Path) (object storj.MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

// DeleteObject deletes an object from database
func (db *DB) DeleteObject(ctx context.Context, bucket string, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	store, err := db.buckets.GetObjectStore(ctx, bucket)
	if err != nil {
		return err
	}

	return store.Delete(ctx, path)
}

// ModifyPendingObject creates an interface for updating a partially uploaded object
func (db *DB) ModifyPendingObject(ctx context.Context, bucket string, path storj.Path) (object storj.MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

// ListPendingObjects lists pending objects in bucket based on the ListOptions
func (db *DB) ListPendingObjects(ctx context.Context, bucket string, options storj.ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)
	return storj.ObjectList{}, errors.New("not implemented")
}

// ListObjects lists objects in bucket based on the ListOptions
func (db *DB) ListObjects(ctx context.Context, bucket string, options storj.ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, err := db.GetBucket(ctx, bucket)
	if err != nil {
		return storj.ObjectList{}, err
	}

	objects, err := db.buckets.GetObjectStore(ctx, bucket)
	if err != nil {
		return storj.ObjectList{}, err
	}

	var startAfter, endBefore string
	switch options.Direction {
	case storj.Before:
		// before lists backwards from cursor, without cursor
		endBefore = options.Cursor
	case storj.Backward:
		// backward lists backwards from cursor, including cursor
		endBefore = keyAfter(options.Cursor)
	case storj.Forward:
		// forward lists forwards from cursor, including cursor
		startAfter = keyBefore(options.Cursor)
	case storj.After:
		// after lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.ObjectList{}, errClass.New("invalid direction %d", options.Direction)
	}

	// TODO: remove this hack-fix of specifying the last key
	if options.Cursor == "" && (options.Direction == storj.Before || options.Direction == storj.Backward) {
		endBefore = "\x7f\x7f\x7f\x7f\x7f\x7f\x7f"
	}

	items, more, err := objects.List(ctx, options.Prefix, startAfter, endBefore, options.Recursive, options.Limit, meta.All)
	if err != nil {
		return storj.ObjectList{}, err
	}

	list = storj.ObjectList{
		Bucket: bucket,
		Prefix: options.Prefix,
		More:   more,
		Items:  make([]storj.Object, 0, len(items)),
	}

	for _, item := range items {
		list.Items = append(list.Items, objectFromMeta(bucketInfo, item.Path, item.IsPrefix, item.Meta))
	}

	return list, nil
}

type object struct {
	fullpath        string
	encryptedPath   string
	lastSegmentMeta segments.Meta
	streamInfo      pb.StreamInfo
	streamMeta      pb.StreamMeta
}

func (db *DB) getInfo(ctx context.Context, prefix string, bucket string, path storj.Path) (obj object, info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, err := db.GetBucket(ctx, bucket)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	if path == "" {
		return object{}, storj.Object{}, storj.ErrNoPath.New("")
	}

	fullpath := bucket + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, bucketInfo.PathCipher, db.rootKey)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	pointer, _, _, err := db.pointers.Get(ctx, prefix+encryptedPath)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrObjectNotFound.Wrap(err)
		}
		return object{}, storj.Object{}, err
	}

	var redundancyScheme *pb.RedundancyScheme
	if pointer.GetType() == pb.Pointer_REMOTE {
		redundancyScheme = pointer.GetRemote().GetRedundancy()
	} else {
		// TODO: handle better
		redundancyScheme = &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_RS,
			MinReq:           -1,
			Total:            -1,
			RepairThreshold:  -1,
			SuccessThreshold: -1,
			ErasureShareSize: -1,
		}
	}

	lastSegmentMeta := segments.Meta{
		Modified:   convertTime(pointer.GetCreationDate()),
		Expiration: convertTime(pointer.GetExpirationDate()),
		Size:       pointer.GetSegmentSize(),
		Data:       pointer.GetMetadata(),
	}

	streamInfoData, err := streams.DecryptStreamInfo(ctx, lastSegmentMeta, fullpath, db.rootKey)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	streamInfo := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfoData, &streamInfo)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	info, err = db.objectStreamFromMeta(ctx, bucketInfo, path, lastSegmentMeta, streamInfo, streamMeta, redundancyScheme)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	return object{
		fullpath:        fullpath,
		encryptedPath:   encryptedPath,
		lastSegmentMeta: lastSegmentMeta,
		streamInfo:      streamInfo,
		streamMeta:      streamMeta,
	}, info, nil
}

func objectFromMeta(bucket storj.Bucket, path storj.Path, isPrefix bool, meta objects.Meta) storj.Object {
	return storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: isPrefix,

		Metadata: meta.UserDefined,

		ContentType: meta.ContentType,
		Created:     meta.Modified, // TODO: use correct field
		Modified:    meta.Modified, // TODO: use correct field
		Expires:     meta.Expiration,

		Stream: storj.Stream{
			Size:     meta.Size,
			Checksum: []byte(meta.Checksum),
		},
	}
}

func (db *DB) objectStreamFromMeta(ctx context.Context, bucket storj.Bucket, path storj.Path, lastSegment segments.Meta, stream pb.StreamInfo, streamMeta pb.StreamMeta, redundancyScheme *pb.RedundancyScheme) (storj.Object, error) {
	var nonce storj.Nonce
	copy(nonce[:], streamMeta.LastSegmentMeta.KeyNonce)

	serMetaInfo := pb.SerializableMeta{}
	err := proto.Unmarshal(stream.Metadata, &serMetaInfo)
	if err != nil {
		return storj.Object{}, err
	}

	objInfo := storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: false,

		Metadata: serMetaInfo.UserDefined,

		ContentType: serMetaInfo.ContentType,
		Created:     lastSegment.Modified,   // TODO: use correct field
		Modified:    lastSegment.Modified,   // TODO: use correct field
		Expires:     lastSegment.Expiration, // TODO: use correct field

		Stream: storj.Stream{
			Size: stream.SegmentsSize*(stream.NumberOfSegments-1) + stream.LastSegmentSize,
			// Checksum: []byte(object.Checksum),

			SegmentCount:     stream.NumberOfSegments,
			FixedSegmentSize: stream.SegmentsSize,

			RedundancyScheme: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      redundancyScheme.GetErasureShareSize(),
				RequiredShares: int16(redundancyScheme.GetMinReq()),
				RepairShares:   int16(redundancyScheme.GetRepairThreshold()),
				OptimalShares:  int16(redundancyScheme.GetSuccessThreshold()),
				TotalShares:    int16(redundancyScheme.GetTotal()),
			},
			EncryptionScheme: storj.EncryptionScheme{
				Cipher:    storj.Cipher(streamMeta.EncryptionType),
				BlockSize: streamMeta.EncryptionBlockSize,
			},
			LastSegment: storj.LastSegment{
				Size:              stream.LastSegmentSize,
				EncryptedKeyNonce: nonce,
				EncryptedKey:      streamMeta.LastSegmentMeta.EncryptedKey,
			},
		},
	}

	/* iterate over all the segments */
	bucketInfo, err := db.GetBucket(ctx, bucket.Name)
	if err != nil {
		return storj.Object{}, err
	}

	if path == "" {
		return storj.Object{}, storj.ErrNoPath.New("")
	}

	fullpath := bucket.Name + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, bucketInfo.PathCipher, db.rootKey)
	if err != nil {
		return storj.Object{}, err
	}

	var segmentList []storj.Segment
	for i := int64(0); i < stream.NumberOfSegments-1; i++ {
		currentPath := storj.JoinPaths(fmt.Sprintf("s%d", i), encryptedPath)
		pointer, nodes, _, err := db.pointers.Get(ctx, currentPath)
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				err = storj.ErrObjectNotFound.Wrap(err)
			}
			return storj.Object{}, err
		}

		segInfo := storj.Segment{}
		if pointer.GetType() == pb.Pointer_REMOTE {
			seg := pointer.GetRemote()

			segInfo.Index = i
			segInfo.Size = pointer.GetSegmentSize()
			segInfo.PieceID = storj.PieceID(seg.GetPieceId())

			// minium need pieces
			segInfo.Needed = calcNeededNodes(pointer.GetRemote().GetRedundancy())

			nodes, err := lookupAndAlignNodes(ctx, db.overlay, nodes, seg)
			if err != nil {
				return storj.Object{}, err
			}

			// currently available nodes
			segInfo.Online = int32(len(nodes))

		} else {
			// TODO: handle better
			redundancyScheme = &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_RS,
				MinReq:           -1,
				Total:            -1,
				RepairThreshold:  -1,
				SuccessThreshold: -1,
				ErasureShareSize: -1,
			}
		}
		segmentList = append(segmentList, segInfo)
		// segmentMeta := segments.Meta{
		// 	Modified:   convertTime(pointer.GetCreationDate()),
		// 	Expiration: convertTime(pointer.GetExpirationDate()),
		// 	Size:       pointer.GetSegmentSize(),
		// 	Data:       pointer.GetMetadata(),
		// }

		// streamInfoData, err := streams.DecryptStreamInfo(ctx, segmentMeta, fullpath, db.rootKey)
		// if err != nil {
		// 	return storj.Object{}, err
		// }

		// streamInfo := pb.StreamInfo{}
		// err = proto.Unmarshal(streamInfoData, &streamInfo)
		// if err != nil {
		// 	return storj.Object{}, err
		// }

	}
	objInfo.SegmentList = segmentList
	return objInfo, nil
}

func makeRedundancyStrategy(scheme *pb.RedundancyScheme) (eestream.RedundancyStrategy, error) {
	fc, err := infectious.NewFEC(int(scheme.GetMinReq()), int(scheme.GetTotal()))
	if err != nil {
		return eestream.RedundancyStrategy{}, err
	}
	es := eestream.NewRSScheme(fc, int(scheme.GetErasureShareSize()))
	return eestream.NewRedundancyStrategy(es, int(scheme.GetRepairThreshold()), int(scheme.GetSuccessThreshold()))
}

// calcNeededNodes calculate how many minimum nodes are needed for download,
// based on t = k + (n-o)k/o
func calcNeededNodes(rs *pb.RedundancyScheme) int32 {
	extra := int32(1)

	if rs.GetSuccessThreshold() > 0 {
		extra = ((rs.GetTotal() - rs.GetSuccessThreshold()) * rs.GetMinReq()) / rs.GetSuccessThreshold()
		if extra == 0 {
			// ensure there is at least one extra node, so we can have error detection/correction
			extra = 1
		}
	}

	needed := rs.GetMinReq() + extra

	if needed > rs.GetTotal() {
		needed = rs.GetTotal()
	}

	return needed
}

// lookupNodes, if necessary, calls Lookup to get node addresses from the overlay.
// It also realigns the nodes to an indexed list of nodes based on the piece number.
// Missing pieces are represented by a nil node.
func lookupAndAlignNodes(ctx context.Context, oc overlay.Client, nodes []*pb.Node, seg *pb.RemoteSegment) (result []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if nodes == nil {
		// Get list of all nodes IDs storing a piece from the segment
		var nodeIds storj.NodeIDList
		for _, p := range seg.RemotePieces {
			nodeIds = append(nodeIds, p.NodeId)
		}
		// Lookup the node info from node IDs
		nodes, err = oc.BulkLookup(ctx, nodeIds)
		if err != nil {
			return nil, err
		}
	}
	for _, v := range nodes {
		if v != nil {
			v.Type.DPanicOnInvalid("lookup and align nodes")
		}
	}

	// Realign the nodes
	result = make([]*pb.Node, seg.GetRedundancy().GetTotal())
	for i, p := range seg.GetRemotePieces() {
		result[p.PieceNum] = nodes[i]
	}

	return result, nil
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

type mutableObject struct {
	db   *DB
	info storj.Object
}

func (object *mutableObject) Info() storj.Object { return object.info }

func (object *mutableObject) CreateStream(ctx context.Context) (storj.MutableStream, error) {
	return &mutableStream{
		db:   object.db,
		info: object.info,
	}, nil
}

func (object *mutableObject) ContinueStream(ctx context.Context) (storj.MutableStream, error) {
	return nil, errors.New("not implemented")
}

func (object *mutableObject) DeleteStream(ctx context.Context) error {
	return errors.New("not implemented")
}

func (object *mutableObject) Commit(ctx context.Context) error {
	_, info, err := object.db.getInfo(ctx, committedPrefix, object.info.Bucket.Name, object.info.Path)
	object.info = info
	return err
}
