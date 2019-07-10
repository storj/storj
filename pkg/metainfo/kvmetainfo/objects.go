// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// DefaultRS default values for RedundancyScheme
var DefaultRS = storj.RedundancyScheme{
	Algorithm:      storj.ReedSolomon,
	RequiredShares: 20,
	RepairShares:   30,
	OptimalShares:  40,
	TotalShares:    50,
	ShareSize:      1 * memory.KiB.Int32(),
}

// DefaultES default values for EncryptionParameters
// BlockSize should default to the size of a stripe
var DefaultES = storj.EncryptionParameters{
	CipherSuite: storj.EncAESGCM,
	BlockSize:   DefaultRS.StripeSize(),
}

// GetObject returns information about an object
func (db *DB) GetObject(ctx context.Context, bucket string, path storj.Path) (info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	_, info, err = db.getInfo(ctx, bucket, path)

	return info, err
}

// GetObjectStream returns interface for reading the object stream
func (db *DB) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (stream storj.ReadOnlyStream, err error) {
	defer mon.Task()(&ctx)(&err)

	meta, info, err := db.getInfo(ctx, bucket, path)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(bucket, meta.fullpath.UnencryptedPath(), db.encStore)
	if err != nil {
		return nil, err
	}

	return &readonlyStream{
		db:        db,
		info:      info,
		bucket:    meta.bucket,
		encPath:   meta.encPath.Raw(),
		streamKey: streamKey,
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
		info.EncryptionParameters = createInfo.EncryptionParameters
	}

	// TODO: autodetect content type from the path extension
	// if info.ContentType == "" {}

	if info.EncryptionParameters.IsZero() {
		info.EncryptionParameters = storj.EncryptionParameters{
			CipherSuite: DefaultES.CipherSuite,
			BlockSize:   DefaultES.BlockSize,
		}
	}

	if info.RedundancyScheme.IsZero() {
		info.RedundancyScheme = DefaultRS

		// If the provided EncryptionParameters.BlockSize isn't a multiple of the
		// DefaultRS stripeSize, then overwrite the EncryptionParameters with the DefaultES values
		if err := validateBlockSize(DefaultRS, info.EncryptionParameters.BlockSize); err != nil {
			info.EncryptionParameters.BlockSize = DefaultES.BlockSize
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

	bucketInfo, err := db.GetBucket(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return err
	}
	prefixed := prefixedObjStore{
		store:  objects.NewStore(db.streams, bucketInfo.PathCipher),
		prefix: bucket,
	}
	return prefixed.Delete(ctx, path)
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

	objects := prefixedObjStore{
		store:  objects.NewStore(db.streams, bucketInfo.PathCipher),
		prefix: bucket,
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
	fullpath        streams.Path
	bucket          string
	encPath         paths.Encrypted
	lastSegmentMeta segments.Meta
	streamInfo      pb.StreamInfo
	streamMeta      pb.StreamMeta
}

func (db *DB) getInfo(ctx context.Context, bucket string, path storj.Path) (obj object, info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: we shouldn't need to go load the bucket metadata every time we get object info
	bucketInfo, err := db.GetBucket(ctx, bucket)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	if path == "" {
		return object{}, storj.Object{}, storj.ErrNoPath.New("")
	}

	fullpath := streams.CreatePath(bucket, paths.NewUnencrypted(path))

	encPath, err := encryption.EncryptPath(bucket, paths.NewUnencrypted(path), bucketInfo.PathCipher, db.encStore)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	pointer, err := db.metainfo.SegmentInfo(ctx, bucket, encPath.Raw(), -1)
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
		Modified:   pointer.CreationDate,
		Expiration: pointer.GetExpirationDate(),
		Size:       pointer.GetSegmentSize(),
		Data:       pointer.GetMetadata(),
	}

	streamInfoData, streamMeta, err := streams.TypedDecryptStreamInfo(ctx, lastSegmentMeta.Data, fullpath, db.encStore)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	streamInfo := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfoData, &streamInfo)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	info, err = objectStreamFromMeta(bucketInfo, path, lastSegmentMeta, streamInfo, streamMeta, redundancyScheme)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	return object{
		fullpath:        fullpath,
		bucket:          bucket,
		encPath:         encPath,
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
	var encryptedKey storj.EncryptedPrivateKey
	if streamMeta.LastSegmentMeta != nil {
		copy(nonce[:], streamMeta.LastSegmentMeta.KeyNonce)
		encryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
	}

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
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(streamMeta.EncryptionType),
				BlockSize:   streamMeta.EncryptionBlockSize,
			},
			LastSegment: storj.LastSegment{
				Size:              stream.LastSegmentSize,
				EncryptedKeyNonce: nonce,
				EncryptedKey:      encryptedKey,
			},
		},
	}, nil
}

type mutableObject struct {
	db   *DB
	info storj.Object
}

func (object *mutableObject) Info() storj.Object { return object.info }

func (object *mutableObject) CreateStream(ctx context.Context) (_ storj.MutableStream, err error) {
	defer mon.Task()(&ctx)(&err)
	return &mutableStream{
		db:   object.db,
		info: object.info,
	}, nil
}

func (object *mutableObject) ContinueStream(ctx context.Context) (_ storj.MutableStream, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

func (object *mutableObject) DeleteStream(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errors.New("not implemented")
}

func (object *mutableObject) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, info, err := object.db.getInfo(ctx, object.info.Bucket.Name, object.info.Path)
	object.info = info
	return err
}
