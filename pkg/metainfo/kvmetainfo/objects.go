// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// Objects implements storj.Metainfo bucket handling
type Objects struct {
	objects  objects.Store
	streams  streams.Store
	segments segments.Store

	rootKey *storj.Key
}

const (
	locationCommitted = "l/"
	locationPending   = "m/"
)

// NewObjects creates Objects
func NewObjects(objects objects.Store, streams streams.Store, segments segments.Store, rootKey *storj.Key) *Objects {
	return &Objects{
		objects:  objects,
		streams:  streams,
		segments: segments,
		rootKey:  rootKey,
	}
}

// GetObject returns information about an object
func (db *Objects) GetObject(ctx context.Context, bucket string, path storj.Path) (storj.Object, error) {
	_, info, err := db.getInfo(ctx, locationCommitted, bucket, path)
	return info, err
}

// GetObjectStream returns interface for reading the object stream
func (db *Objects) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (storj.ReadOnlyStream, error) {
	meta, info, err := db.getInfo(ctx, locationCommitted, bucket, path)

	streamKey, err := encryption.DeriveContentKey(meta.fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	return &readonlyStream{
		db:            db,
		info:          info,
		encryptedPath: meta.encryptedPath,
		streamKey:     streamKey,

		lastSegment:     meta.streamMeta,
		lastSegmentSize: meta.streamInfo.LastSegmentSize,
	}, nil
}

// CreateObject creates an uploading object and returns an interface for uploading Object information
func (db *Objects) CreateObject(ctx context.Context, bucket string, path storj.Path, createInfo *storj.CreateObject) (storj.MutableObject, error) {
	if createInfo == nil {
		// TODO: get bucket defaults
	}

	err := db.setInfo(ctx, locationPending, bucket, path, createInfo.Object(bucket, path))
	if err != nil {
		return nil, err
	}

	meta, info, err := db.getInfo(ctx, locationPending, bucket, path)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(meta.fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	return &mutableObject{
		db:            db,
		info:          info,
		encryptedPath: meta.encryptedPath,
		streamKey:     streamKey,
	}, nil
}

// ContinueObject continues a pending object
func (db *Objects) ContinueObject(ctx context.Context, bucket string, path storj.Path) (storj.MutableObject, error) {
	meta, info, err := db.getInfo(ctx, locationPending, bucket, path)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(meta.fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	return &mutableObject{
		db:            db,
		info:          info,
		encryptedPath: meta.encryptedPath,
		streamKey:     streamKey,
	}, nil
}

// ModifyObject continues a pending object
func (db *Objects) ModifyObject(ctx context.Context, bucket string, path storj.Path) (storj.MutableObject, error) {
	meta, info, err := db.getInfo(ctx, locationCommitted, bucket, path)
	if err != nil {
		return nil, err
	}

	streamKey, err := encryption.DeriveContentKey(meta.fullpath, db.rootKey)
	if err != nil {
		return nil, err
	}

	return &mutableObject{
		db:            db,
		info:          info,
		encryptedPath: meta.encryptedPath,
		streamKey:     streamKey,
	}, nil
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

type object struct {
	fullpath        string
	encryptedPath   string
	lastSegmentMeta segments.Meta
	streamInfo      pb.StreamInfo
	streamMeta      pb.StreamMeta
}

func (db *Objects) getInfo(ctx context.Context, location string, bucket string, path storj.Path) (object, storj.Object, error) {
	fullpath := bucket + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, db.rootKey)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	_, lastSegmentMeta, err := db.segments.Get(ctx, location+encryptedPath)
	if err != nil {
		return object{}, storj.Object{}, err
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

	info := objectStreamFromMeta(bucket, path, lastSegmentMeta, streamInfo, streamMeta)

	return object{
		fullpath:        fullpath,
		encryptedPath:   encryptedPath,
		lastSegmentMeta: lastSegmentMeta,
		streamInfo:      streamInfo,
		streamMeta:      streamMeta,
	}, info, nil
}

func (db *Objects) deleteStreamInfo(ctx context.Context, location string, bucket string, path storj.Path) error {
	fullpath := bucket + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, db.rootKey)
	if err != nil {
		return err
	}

	return db.segments.Delete(ctx, location+encryptedPath)
}

func (db *Objects) setInfo(ctx context.Context, location string, bucket string, path storj.Path, obj storj.Object) error {
	fullpath := bucket + "/" + path

	derivedKey, err := encryption.DeriveContentKey(fullpath, db.rootKey)
	if err != nil {
		return err
	}

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, db.rootKey)
	if err != nil {
		return err
	}

	streamInfoData, err := proto.Marshal(&pb.StreamInfo{
		NumberOfSegments: obj.Stream.SegmentCount - 1,
		SegmentsSize:     obj.Stream.FixedSegmentSize,
		LastSegmentSize:  -1,  // TODO: add to stream for now
		Metadata:         nil, // TODO
	})

	encryptedInfo, err := encryption.Encrypt(streamInfoData, obj.Stream.EncryptionScheme.Cipher)
	/*
		return storj.Object{
			Version:  0, // TODO: add to info
			Bucket:   bucket,
			Path:     path,
			IsPrefix: false,

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
	*/
	_, lastSegmentMeta, err := db.segments.Get(ctx, location+encryptedPath)
	if err != nil {
		return err
	}

	streamInfoData, err := streams.DecryptStreamInfo(ctx, lastSegmentMeta, fullpath, db.rootKey)
	if err != nil {
		return err
	}

	streamInfo := pb.StreamInfo{}
	err = proto.Unmarshal(streamInfoData, &streamInfo)
	if err != nil {
		return err
	}

	streamMeta := pb.StreamMeta{}
	err = proto.Unmarshal(lastSegmentMeta.Data, &streamMeta)
	if err != nil {
		return err
	}

	// info := objectStreamFromMeta(bucket, path, lastSegmentMeta, streamInfo, streamMeta)
	return nil
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

func objectStreamFromMeta(bucket string, path storj.Path, lastSegment segments.Meta, stream pb.StreamInfo, streamMeta pb.StreamMeta) storj.Object {
	return storj.Object{
		Version:  0, // TODO: add to info
		Bucket:   bucket,
		Path:     path,
		IsPrefix: false,

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
