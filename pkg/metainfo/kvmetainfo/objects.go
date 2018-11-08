// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.uber.org/zap"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
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
	pointers pdbclient.Client

	rootKey *storj.Key
}

const (
	// commitedPrefix is prefix where completed object info is stored
	committedPrefix = "l/"
)

// NewObjects creates Objects
func NewObjects(objects objects.Store, streams streams.Store, segments segments.Store, pointers pdbclient.Client, rootKey *storj.Key) *Objects {
	return &Objects{
		objects:  objects,
		streams:  streams,
		segments: segments,
		pointers: pointers,
		rootKey:  rootKey,
	}
}

// GetObject returns information about an object
func (db *Objects) GetObject(ctx context.Context, bucket string, path storj.Path) (storj.Object, error) {
	_, info, err := db.getInfo(ctx, committedPrefix, bucket, path)
	return info, err
}

// GetObjectStream returns interface for reading the object stream
func (db *Objects) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (storj.ReadOnlyStream, error) {
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
func (db *Objects) CreateObject(ctx context.Context, bucket string, path storj.Path, createInfo *storj.CreateObject) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// ModifyObject modifies a committed object
func (db *Objects) ModifyObject(ctx context.Context, bucket string, path storj.Path) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// DeleteObject deletes an object from database
func (db *Objects) DeleteObject(ctx context.Context, bucket string, path storj.Path) error {
	return db.objects.Delete(ctx, bucket+"/"+path)
}

// ModifyPendingObject creates an interface for updating a partially uploaded object
func (db *Objects) ModifyPendingObject(ctx context.Context, bucket string, path storj.Path) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// ListPendingObjects lists pending objects in bucket based on the ListOptions
func (db *Objects) ListPendingObjects(ctx context.Context, bucket string, options storj.ListOptions) (storj.ObjectList, error) {
	return storj.ObjectList{}, errors.New("not implemented")
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

func (db *Objects) getInfo(ctx context.Context, prefix string, bucket string, path storj.Path) (object, storj.Object, error) {
	fullpath := bucket + "/" + path

	encryptedPath, err := streams.EncryptAfterBucket(fullpath, db.rootKey)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	pointer, _, err := db.pointers.Get(ctx, prefix+encryptedPath)
	if err != nil {
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
		Size:       pointer.GetSize(),
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

	info := objectStreamFromMeta(bucket, path, lastSegmentMeta, streamInfo, streamMeta, redundancyScheme)

	return object{
		fullpath:        fullpath,
		encryptedPath:   encryptedPath,
		lastSegmentMeta: lastSegmentMeta,
		streamInfo:      streamInfo,
		streamMeta:      streamMeta,
	}, info, nil
}

func objectFromMeta(bucket string, path storj.Path, isPrefix bool, meta objects.Meta) storj.Object {
	return storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: isPrefix,

		Metadata: nil,

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

func objectStreamFromMeta(bucket string, path storj.Path, lastSegment segments.Meta, stream pb.StreamInfo, streamMeta pb.StreamMeta, redundancyScheme *pb.RedundancyScheme) storj.Object {
	var nonce storj.Nonce
	copy(nonce[:], streamMeta.LastSegmentMeta.KeyNonce)
	return storj.Object{
		Version:  0, // TODO:
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

			SegmentCount:     stream.NumberOfSegments,
			FixedSegmentSize: stream.SegmentsSize,

			RedundancyScheme: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      int64(redundancyScheme.GetErasureShareSize()),
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
