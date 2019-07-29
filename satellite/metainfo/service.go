// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/storage/meta"
)

// Service structure
type Service struct {
	logger    *zap.Logger
	DB        storage.KeyValueStore
	bucketsDB BucketsDB
}

// NewService creates new metainfo service
func NewService(logger *zap.Logger, db storage.KeyValueStore, bucketsDB BucketsDB) *Service {
	return &Service{logger: logger, DB: db, bucketsDB: bucketsDB}
}

// Put puts pointer to db under specific path
func (s *Service) Put(ctx context.Context, path string, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Update the pointer with the creation date
	pointer.CreationDate = time.Now()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		return Error.Wrap(err)
	}

	// CompareAndSwap is used instead of Put to avoid overwriting existing pointers
	err = s.DB.CompareAndSwap(ctx, []byte(path), nil, pointerBytes)
	return Error.Wrap(err)
}

// UpdatePieces atomically adds toAdd pieces and removes toRemove pieces from
// the pointer under path. ref is the pointer that caller received via Get
// prior to calling this method.
//
// It will first check if the pointer has been deleted or replaced. Then it
// will remove the toRemove pieces and then it will add the toAdd pieces.
// Replacing the node ID and the hash of a piece can be done by adding the
// piece to both toAdd and toRemove.
func (s *Service) UpdatePieces(ctx context.Context, path string, ref *pb.Pointer, toAdd, toRemove []*pb.RemotePiece) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		// read the pointer
		oldPointerBytes, err := s.DB.Get(ctx, []byte(path))
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// unmarshal the pointer
		pointer = &pb.Pointer{}
		err = proto.Unmarshal(oldPointerBytes, pointer)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// check if pointer has been replaced
		if !pointer.GetCreationDate().Equal(ref.GetCreationDate()) {
			return nil, Error.New("pointer has been replaced")
		}

		// put all existing pieces to a map
		pieceMap := make(map[int32]*pb.RemotePiece)
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			pieceMap[piece.PieceNum] = piece
		}

		// remove the toRemove pieces from the map
		// only if all piece number, node id and hash match
		for _, piece := range toRemove {
			if piece == nil {
				continue
			}
			existing := pieceMap[piece.PieceNum]
			if existing != nil &&
				existing.NodeId == piece.NodeId &&
				existing.Hash == piece.Hash {
				delete(pieceMap, piece.PieceNum)
			}
		}

		// add the toAdd pieces to the map
		for _, piece := range toAdd {
			if piece == nil {
				continue
			}
			_, exists := pieceMap[piece.PieceNum]
			if exists {
				return nil, Error.New("piece to add already exists (piece no: %d)", piece.PieceNum)
			}
			pieceMap[piece.PieceNum] = piece
		}

		// copy the pieces from the map back to the pointer
		var pieces []*pb.RemotePiece
		for _, piece := range pieceMap {
			pieces = append(pieces, piece)
		}
		pointer.GetRemote().RemotePieces = pieces

		// marshal the pointer
		newPointerBytes, err := proto.Marshal(pointer)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// write the pointer using compare-and-swap
		err = s.DB.CompareAndSwap(ctx, []byte(path), oldPointerBytes, newPointerBytes)
		if storage.ErrValueChanged.Has(err) {
			continue
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}
		return pointer, nil
	}
}

// Get gets pointer from db
func (s *Service) Get(ctx context.Context, path string) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)
	pointerBytes, err := s.DB.Get(ctx, []byte(path))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pointer = &pb.Pointer{}
	err = proto.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return pointer, nil
}

// List returns all Path keys in the pointers bucket
func (s *Service) List(ctx context.Context, prefix string, startAfter string, endBefore string, recursive bool, limit int32,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var prefixKey storage.Key
	if prefix != "" {
		prefixKey = storage.Key(prefix)
		if prefix[len(prefix)-1] != storage.Delimiter {
			prefixKey = append(prefixKey, storage.Delimiter)
		}
	}

	rawItems, more, err := storage.ListV2(ctx, s.DB, storage.ListOptions{
		Prefix:       prefixKey,
		StartAfter:   storage.Key(startAfter),
		EndBefore:    storage.Key(endBefore),
		Recursive:    recursive,
		Limit:        int(limit),
		IncludeValue: metaFlags != meta.None,
	})
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	for _, rawItem := range rawItems {
		items = append(items, s.createListItem(ctx, rawItem, metaFlags))
	}
	return items, more, nil
}

// createListItem creates a new list item with the given path. It also adds
// the metadata according to the given metaFlags.
func (s *Service) createListItem(ctx context.Context, rawItem storage.ListItem, metaFlags uint32) *pb.ListResponse_Item {
	defer mon.Task()(&ctx)(nil)
	item := &pb.ListResponse_Item{
		Path:     rawItem.Key.String(),
		IsPrefix: rawItem.IsPrefix,
	}
	if item.IsPrefix {
		return item
	}

	err := s.setMetadata(item, rawItem.Value, metaFlags)
	if err != nil {
		s.logger.Warn("err retrieving metadata", zap.Error(err))
	}
	return item
}

// getMetadata adds the metadata to the given item pointer according to the
// given metaFlags
func (s *Service) setMetadata(item *pb.ListResponse_Item, data []byte, metaFlags uint32) (err error) {
	if metaFlags == meta.None || len(data) == 0 {
		return nil
	}

	pr := &pb.Pointer{}
	err = proto.Unmarshal(data, pr)
	if err != nil {
		return Error.Wrap(err)
	}

	// Start with an empty pointer to and add only what's requested in
	// metaFlags to safe to transfer payload
	item.Pointer = &pb.Pointer{}
	if metaFlags&meta.Modified != 0 {
		item.Pointer.CreationDate = pr.GetCreationDate()
	}
	if metaFlags&meta.Expiration != 0 {
		item.Pointer.ExpirationDate = pr.GetExpirationDate()
	}
	if metaFlags&meta.Size != 0 {
		item.Pointer.SegmentSize = pr.GetSegmentSize()
	}
	if metaFlags&meta.UserDefined != 0 {
		item.Pointer.Metadata = pr.GetMetadata()
	}

	return nil
}

// Delete deletes from item from db
func (s *Service) Delete(ctx context.Context, path string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return s.DB.Delete(ctx, []byte(path))
}

// Iterate iterates over items in db
func (s *Service) Iterate(ctx context.Context, prefix string, first string, recurse bool, reverse bool, f func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	opts := storage.IterateOptions{
		Prefix:  storage.Key(prefix),
		First:   storage.Key(first),
		Recurse: recurse,
		Reverse: reverse,
	}
	return s.DB.Iterate(ctx, opts, f)
}

// CreateBucket creates a new bucket in the buckets db
func (s *Service) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.CreateBucket(ctx, bucket)
}

// GetBucket returns an existing bucket in the buckets db
func (s *Service) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.GetBucket(ctx, bucketName, projectID)
}

// UpdateBucket returns an updated bucket in the buckets db
func (s *Service) UpdateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.UpdateBucket(ctx, bucket)
}

// DeleteBucket deletes a bucket from the bucekts db
func (s *Service) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.DeleteBucket(ctx, bucketName, projectID)
}

// ListBuckets returns a list of buckets for a project
func (s *Service) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.ListBuckets(ctx, projectID, listOpts, allowedBuckets)
}
