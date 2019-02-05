// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/encryption"
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

	info, err = objectStreamFromMeta(bucketInfo, path, lastSegmentMeta, streamInfo, streamMeta, redundancyScheme)
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

func objectStreamFromMeta(bucket storj.Bucket, path storj.Path, lastSegment segments.Meta, stream pb.StreamInfo, streamMeta pb.StreamMeta, redundancyScheme *pb.RedundancyScheme) (storj.Object, error) {
	var nonce storj.Nonce
	copy(nonce[:], streamMeta.LastSegmentMeta.KeyNonce)

	serMetaInfo := pb.SerializableMeta{}
	err := proto.Unmarshal(stream.Metadata, &serMetaInfo)
	if err != nil {
		return storj.Object{}, err
	}

	return storj.Object{
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
	}, nil
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
