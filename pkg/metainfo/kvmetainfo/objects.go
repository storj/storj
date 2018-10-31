// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

const defaultSegmentLimit = 8 // TODO

// Objects implements storj.Metainfo bucket handling
type Objects struct {
	objects  objects.Store
	streams  streams.Store
	segments segments.Store

	rootKey *storj.Key
}

// NewObjects creates Objects
func NewObjects(objects objects.Store, streams streams.Store, segments segments.Store) *Objects {
	return &Objects{}
}

// GetObject returns information about an object
func (db *Objects) GetObject(ctx context.Context, bucket string, path storj.Path) (storj.Object, error) {
	meta, err := db.objects.Meta(ctx, bucket+"/"+path)
	if err != nil {
		return storj.Object{}, err
	}

	return objectFromMeta(bucket, path, false, meta), nil
}

// GetObjectStream returns interface for reading the object stream
func (db *Objects) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (storj.ReadOnlyStream, error) {
	fullpath := bucket + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	_, lastSegmentMeta, err := db.segments.Get(ctx, "l/"+fullpath)
	if err != nil {
		return nil, err
	}

	streamInfo, err := streams.DecryptStreamInfo(ctx, lastSegmentMeta, fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	info := objectStreamFromMeta(
		bucket, path, false,
		lastSegmentMeta, streamInfo, streamMeta,
	)

	return readonlyStream{
		db:            db,
		info:          info,
		encryptedPath: encryptedPath,
		streamKey:     streamKey,
		
		lastSegment: streamMeta,
		lastSegmentSize: streamInfo.LastSegmentSize,
	}, nil
}

type readonlyStream struct {
	db *Objects

	info          storj.Object
	encryptedPath storj.Path
	streamKey     *storj.Key // lazySegmentReader derivedKey

	lastSegment   pb.StreamMeta
	lastSegmentSize int64
}

func (stream *readonlyStream) Info() storj.Object { return stream.info }

func (stream *readonlyStream) SegmentsAt(ctx context.Context, byteOffset int64, limit int64) (infos []storj.Segment, more bool, err error) {
	if stream.info.FixedSegmentSize <= 0 {
		return nil, false, errors.New("not implemented")
	}

	index := byteOffset / stream.info.FixedSegmentSize
	return stream.Segments(ctx, index, limit)
}

func (stream *readonlyStream) segment(ctx context.Context, index int64) (storj.Segment, err error) {
	segment := storj.Segment{
		Index: index,
	}

	isLastSegment := segment.Index + 1 == stream.info.SegmentCount
	if !isLastSegment {
		segmentPath := streams.GetSegmentPath(stream.encryptedPath, index+1)
		_, meta, err := stream.db.segments.Get(ctx, segmentPath)
		if err != nil {
			return segment, err
		}

		segmentMeta := pb.SegmentMeta{}
		err = proto.Unmarshal(meta.Data, &segmentMeta)
		if err != nil {
			return segment, err
		}
		encryptedKey, encryptedKeyNonce := streams.GetEncryptedKeyAndNonce(&segmentMeta)

		segment.Size = stream.info.FixedSegmentSize
		segment.EncryptedKeyNonce = *encryptedKeyNonce
		segment.EncryptedKey = encryptedKey
	} else {
		lastEncryptedKey, lastEncryptedKeyNonce := streams.GetEncryptedKeyAndNonce(stream.lastSegment.LastSegmentMeta)

		segment.Size = stream.lastSegmentSize
		segment.EncryptedKeyNonce = stream.lastEncryptedKeyNonce
		segment.EncryptedKey = stream.lastEncryptedKey
	}

	var nonce storj.Nonce
	_, err = encryption.Increment(&nonce, index+1)
	if err != nil {
		return segment, err
	}

	return segment, nil
}

func (stream *readonlyStream) Segments(ctx context.Context, index int64, limit int64) (infos []storj.Segment, more bool, err error) {
	if index < 0 {
		return nil, false, errors.New("invalid argument")
	}
	if limit <= 0 {
		limit = defaultSegmentLimit
	}
	if index >= stream.info.SegmentCount {
		return nil, false, nil
	}

	infos = make([]storj.Segment, 0, limit)
	for ; index < stream.info.SegmentCount && limit > 0; index++ {
		limit--
		segment, err := stream.segment(ctx, index)
		if err != nil {
			return nil, false, err
		}
		infos = append(infos, segment)
	}

	more = index+limit >= stream.info.SegmentCount
	return infos, more, nil
}

// CreateObject creates an uploading object and returns an interface for uploading Object information
func (db *Objects) CreateObject(ctx context.Context, bucket string, path storj.Path, info *storj.CreateObject) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// ModifyObject creates an interface for modifying an existing object
func (db *Objects) ModifyObject(ctx context.Context, bucket string, path storj.Path, info storj.Object) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// DeleteObject deletes an object from database
func (db *Objects) DeleteObject(ctx context.Context, bucket string, path storj.Path) error {
	return db.objects.Delete(ctx, bucket+"/"+path)
}

// ListObjects lists objects in bucket based on the ListOptions
func (db *Objects) ListObjects(ctx context.Context, bucket string, options storj.ListOptions) (storj.ObjectList, error) {
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

	items, more, err := db.objects.List(ctx, bucket+"/"+options.Prefix, startAfter, endBefore, options.Recursive, options.Limit, meta.All)
	if err != nil {
		return storj.ObjectList{}, err
	}

	list := storj.ObjectList{
		Bucket: bucket,
		Prefix: options.Prefix,
		More:   more,
		Items:  make([]storj.Object, 0, len(items)),
	}

	for _, item := range items {
		list.Items = append(list.Items, objectFromMeta("", item.Path, item.IsPrefix, item.Meta))
	}

	return list, nil
}

func objectFromMeta(bucket string, path storj.Path, isPrefix bool, meta objects.Meta) storj.Object {
	return storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: isPrefix,

		Metadata: nil,

		ContentType: meta.ContentType,
		// Created:     meta.Created,
		Modified: meta.Modified,
		Expires:  meta.Expiration,

		Stream: storj.Stream{
			Size:     meta.Size,
			Checksum: []byte(meta.Checksum),
		},
	}
}

func objectStreamFromMeta(
	bucket string, path storj.Path, isPrefix bool,
	lastSegment segments.Meta, stream pb.StreamInfo, streamMeta pb.StreamMeta,
) storj.Object {

	return storj.Object{
		Version:  0, // TODO: add to info
		Bucket:   bucket,
		Path:     path,
		IsPrefix: isPrefix,

		Metadata: nil, // TODO:

		// ContentType: object.ContentType,
		Created:  lastSegment.Modified,   // TODO: use correct field
		Modified: lastSegment.Modified,   // TODO: use correct field
		Expires:  lastSegment.Expiration, // TODO: use correct field

		Stream: storj.Stream{
			Size: stream.SegmentsSize*(stream.NumberOfSegments-1) + stream.LastSegmentSize,
			// Checksum: []byte(object.Checksum),

			SegmentCount:     stream.NumberOfSegments + 1,
			FixedSegmentSize: stream.SegmentsSize,

			RedundancyScheme: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      0, // TODO:
				RequiredShares: 0, // TODO:
				RepairShares:   0, // TODO:
				OptimalShares:  0, // TODO:
				TotalShares:    0, // TODO:
			},
			EncryptionScheme: storj.EncryptionScheme{
				Cipher:    storj.Cipher(streamMeta.EncryptionType),
				BlockSize: int32(streamMeta.EncryptionBlockSize),
			},
		},
	}
}
