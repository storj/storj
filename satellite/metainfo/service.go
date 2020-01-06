// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/storage/meta"
)

// Service structure
//
// architecture: Service
type Service struct {
	logger    *zap.Logger
	db        PointerDB
	bucketsDB BucketsDB
}

// NewService creates new metainfo service.
func NewService(logger *zap.Logger, db PointerDB, bucketsDB BucketsDB) *Service {
	return &Service{logger: logger, db: db, bucketsDB: bucketsDB}
}

// Put puts pointer to db under specific path.
func (s *Service) Put(ctx context.Context, path string, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Update the pointer with the creation date
	pointer.CreationDate = time.Now()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		return Error.Wrap(err)
	}

	// CompareAndSwap is used instead of Put to avoid overwriting existing pointers
	err = s.db.CompareAndSwap(ctx, []byte(path), nil, pointerBytes)
	return Error.Wrap(err)
}

// UpdatePieces calls UpdatePiecesCheckDuplicates with checkDuplicates equal to false.
func (s *Service) UpdatePieces(ctx context.Context, path string, ref *pb.Pointer, toAdd, toRemove []*pb.RemotePiece) (pointer *pb.Pointer, err error) {
	return s.UpdatePiecesCheckDuplicates(ctx, path, ref, toAdd, toRemove, false)
}

// UpdatePiecesCheckDuplicates atomically adds toAdd pieces and removes toRemove pieces from
// the pointer under path. ref is the pointer that caller received via Get
// prior to calling this method.
//
// It will first check if the pointer has been deleted or replaced.
// Then if checkDuplicates is true it will return an error if the nodes to be
// added are already in the pointer.
// Then it will remove the toRemove pieces and then it will add the toAdd pieces.
// Replacing the node ID and the hash of a piece can be done by adding the
// piece to both toAdd and toRemove.
func (s *Service) UpdatePiecesCheckDuplicates(ctx context.Context, path string, ref *pb.Pointer, toAdd, toRemove []*pb.RemotePiece, checkDuplicates bool) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		// read the pointer
		oldPointerBytes, err := s.db.Get(ctx, []byte(path))
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				err = storj.ErrObjectNotFound.Wrap(err)
			}
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
		nodePieceMap := make(map[storj.NodeID]struct{})
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			pieceMap[piece.PieceNum] = piece
			if checkDuplicates {
				nodePieceMap[piece.NodeId] = struct{}{}
			}
		}

		// Return an error if the pointer already has a piece for this node
		if checkDuplicates {
			for _, piece := range toAdd {
				_, ok := nodePieceMap[piece.NodeId]
				if ok {
					return nil, ErrNodeAlreadyExists.New("node id already exists in pointer. Path: %s, NodeID: %s", path, piece.NodeId.String())
				}
				nodePieceMap[piece.NodeId] = struct{}{}
			}
		}
		// remove the toRemove pieces from the map
		// only if all piece number, node id and hash match
		for _, piece := range toRemove {
			if piece == nil {
				continue
			}
			existing := pieceMap[piece.PieceNum]
			if existing != nil && existing.NodeId == piece.NodeId {
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
			// clear hashes so we don't store them
			piece.Hash = nil
			pieces = append(pieces, piece)
		}
		pointer.GetRemote().RemotePieces = pieces

		pointer.LastRepaired = ref.LastRepaired
		pointer.RepairCount = ref.RepairCount

		// marshal the pointer
		newPointerBytes, err := proto.Marshal(pointer)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// write the pointer using compare-and-swap
		err = s.db.CompareAndSwap(ctx, []byte(path), oldPointerBytes, newPointerBytes)
		if storage.ErrValueChanged.Has(err) {
			continue
		}
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				err = storj.ErrObjectNotFound.Wrap(err)
			}
			return nil, Error.Wrap(err)
		}
		return pointer, nil
	}
}

// Get gets decoded pointer from DB.
func (s *Service) Get(ctx context.Context, path string) (_ *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)
	_, pointer, err := s.GetWithBytes(ctx, path)
	if err != nil {
		return nil, err
	}

	return pointer, nil
}

// GetWithBytes gets the protocol buffers encoded and decoded pointer from the DB.
func (s *Service) GetWithBytes(ctx context.Context, path string) (pointerBytes []byte, pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	pointerBytes, err = s.db.Get(ctx, []byte(path))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrObjectNotFound.Wrap(err)
		}
		return nil, nil, Error.Wrap(err)
	}

	pointer = &pb.Pointer{}
	err = proto.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return pointerBytes, pointer, nil
}

// List returns all Path keys in the pointers bucket
func (s *Service) List(ctx context.Context, prefix string, startAfter string, recursive bool, limit int32,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var prefixKey storage.Key
	if prefix != "" {
		prefixKey = storage.Key(prefix)
		if prefix[len(prefix)-1] != storage.Delimiter {
			prefixKey = append(prefixKey, storage.Delimiter)
		}
	}

	rawItems, more, err := storage.ListV2(ctx, s.db, storage.ListOptions{
		Prefix:       prefixKey,
		StartAfter:   storage.Key(startAfter),
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

// Delete deletes a pointer bytes when it matches oldPointerBytes, otherwise it'll fail.
func (s *Service) Delete(ctx context.Context, path string, oldPointerBytes []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.CompareAndSwap(ctx, []byte(path), oldPointerBytes, nil)
	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}
	return Error.Wrap(err)
}

// UnsynchronizedDelete deletes from item from db without verifying whether the pointer has changed in the database.
func (s *Service) UnsynchronizedDelete(ctx context.Context, path string) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Delete(ctx, []byte(path))
	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}
	return Error.Wrap(err)
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
