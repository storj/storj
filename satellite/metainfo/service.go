// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
	"storj.io/uplink/private/storage/meta"
)

var (
	// ErrBucketNotEmpty is returned when bucket is required to be empty for an operation.
	ErrBucketNotEmpty = errs.Class("bucket not empty")
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
func (s *Service) Put(ctx context.Context, key metabase.SegmentKey, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := sanityCheckPointer(key, pointer); err != nil {
		return Error.Wrap(err)
	}

	// Update the pointer with the creation date
	pointer.CreationDate = time.Now()

	pointerBytes, err := pb.Marshal(pointer)
	if err != nil {
		return Error.Wrap(err)
	}

	// CompareAndSwap is used instead of Put to avoid overwriting existing pointers
	err = s.db.CompareAndSwap(ctx, storage.Key(key), nil, pointerBytes)
	return Error.Wrap(err)
}

// UnsynchronizedPut puts pointer to db under specific path without verifying for existing pointer under the same path.
func (s *Service) UnsynchronizedPut(ctx context.Context, key metabase.SegmentKey, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := sanityCheckPointer(key, pointer); err != nil {
		return Error.Wrap(err)
	}

	// Update the pointer with the creation date
	pointer.CreationDate = time.Now()

	pointerBytes, err := pb.Marshal(pointer)
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.db.Put(ctx, storage.Key(key), pointerBytes)
	return Error.Wrap(err)
}

// UpdatePieces calls UpdatePiecesCheckDuplicates with checkDuplicates equal to false.
func (s *Service) UpdatePieces(ctx context.Context, key metabase.SegmentKey, ref *pb.Pointer, toAdd, toRemove []*pb.RemotePiece) (pointer *pb.Pointer, err error) {
	return s.UpdatePiecesCheckDuplicates(ctx, key, ref, toAdd, toRemove, false)
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
func (s *Service) UpdatePiecesCheckDuplicates(ctx context.Context, key metabase.SegmentKey, ref *pb.Pointer, toAdd, toRemove []*pb.RemotePiece, checkDuplicates bool) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := sanityCheckPointer(key, ref); err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		if err == nil {
			err = sanityCheckPointer(key, pointer)
		}
	}()

	for {
		// read the pointer
		oldPointerBytes, err := s.db.Get(ctx, storage.Key(key))
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				err = storj.ErrObjectNotFound.Wrap(err)
			}
			return nil, Error.Wrap(err)
		}

		// unmarshal the pointer
		pointer = &pb.Pointer{}
		err = pb.Unmarshal(oldPointerBytes, pointer)
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
					return nil, ErrNodeAlreadyExists.New("node id already exists in pointer. Key: %s, NodeID: %s", key, piece.NodeId.String())
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
		newPointerBytes, err := pb.Marshal(pointer)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// write the pointer using compare-and-swap
		err = s.db.CompareAndSwap(ctx, storage.Key(key), oldPointerBytes, newPointerBytes)
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
func (s *Service) Get(ctx context.Context, key metabase.SegmentKey) (_ *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)
	_, pointer, err := s.GetWithBytes(ctx, key)
	if err != nil {
		return nil, err
	}

	return pointer, nil
}

// GetItems gets decoded pointers from DB.
// The return value is in the same order as the argument paths.
func (s *Service) GetItems(ctx context.Context, keys []metabase.SegmentKey) (_ []*pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)
	storageKeys := make(storage.Keys, len(keys))
	for i := range keys {
		storageKeys[i] = storage.Key(keys[i])
	}
	pointerBytes, err := s.db.GetAll(ctx, storageKeys)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pointers := make([]*pb.Pointer, len(pointerBytes))
	for i, p := range pointerBytes {
		if p == nil {
			continue
		}
		var pointer pb.Pointer
		err = pb.Unmarshal([]byte(p), &pointer)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		pointers[i] = &pointer
	}
	return pointers, nil
}

// GetWithBytes gets the protocol buffers encoded and decoded pointer from the DB.
func (s *Service) GetWithBytes(ctx context.Context, key metabase.SegmentKey) (pointerBytes []byte, pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	pointerBytes, err = s.db.Get(ctx, storage.Key(key))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrObjectNotFound.Wrap(err)
		}
		return nil, nil, Error.Wrap(err)
	}

	pointer = &pb.Pointer{}
	err = pb.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return pointerBytes, pointer, nil
}

// List returns all Path keys in the pointers bucket.
func (s *Service) List(ctx context.Context, prefix metabase.SegmentKey, startAfter string, recursive bool, limit int32,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var prefixKey storage.Key
	if len(prefix) != 0 {
		prefixKey = storage.Key(prefix)
		if prefix[len(prefix)-1] != storage.Delimiter {
			prefixKey = append(prefixKey, storage.Delimiter)
		}
	}

	more, err = storage.ListV2Iterate(ctx, s.db, storage.ListOptions{
		Prefix:       prefixKey,
		StartAfter:   storage.Key(startAfter),
		Recursive:    recursive,
		Limit:        int(limit),
		IncludeValue: metaFlags != meta.None,
	}, func(ctx context.Context, item *storage.ListItem) error {
		items = append(items, s.createListItem(ctx, *item, metaFlags))
		return nil
	})
	if err != nil {
		return nil, false, Error.Wrap(err)
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
// given metaFlags.
func (s *Service) setMetadata(item *pb.ListResponse_Item, data []byte, metaFlags uint32) (err error) {
	if metaFlags == meta.None || len(data) == 0 {
		return nil
	}

	pr := &pb.Pointer{}
	err = pb.Unmarshal(data, pr)
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
func (s *Service) Delete(ctx context.Context, key metabase.SegmentKey, oldPointerBytes []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.CompareAndSwap(ctx, storage.Key(key), oldPointerBytes, nil)
	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}
	return Error.Wrap(err)
}

// UnsynchronizedGetDel deletes items from db without verifying whether the pointers have changed in the database,
// and it returns deleted items.
func (s *Service) UnsynchronizedGetDel(ctx context.Context, keys []metabase.SegmentKey) ([]metabase.SegmentKey, []*pb.Pointer, error) {
	storageKeys := make(storage.Keys, len(keys))
	for i := range keys {
		storageKeys[i] = storage.Key(keys[i])
	}

	items, err := s.db.DeleteMultiple(ctx, storageKeys)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	pointerPaths := make([]metabase.SegmentKey, 0, len(items))
	pointers := make([]*pb.Pointer, 0, len(items))

	for _, item := range items {
		data := &pb.Pointer{}
		err = pb.Unmarshal(item.Value, data)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}

		pointerPaths = append(pointerPaths, metabase.SegmentKey(item.Key))
		pointers = append(pointers, data)
	}

	return pointerPaths, pointers, nil
}

// UnsynchronizedDelete deletes item from db without verifying whether the pointer has changed in the database.
func (s *Service) UnsynchronizedDelete(ctx context.Context, key metabase.SegmentKey) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Delete(ctx, storage.Key(key))
	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}
	return Error.Wrap(err)
}

// CreateBucket creates a new bucket in the buckets db.
func (s *Service) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.CreateBucket(ctx, bucket)
}

// GetBucket returns an existing bucket in the buckets db.
func (s *Service) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.GetBucket(ctx, bucketName, projectID)
}

// UpdateBucket returns an updated bucket in the buckets db.
func (s *Service) UpdateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.UpdateBucket(ctx, bucket)
}

// DeleteBucket deletes a bucket from the bucekts db.
func (s *Service) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	empty, err := s.IsBucketEmpty(ctx, projectID, bucketName)
	if err != nil {
		return err
	}
	if !empty {
		return ErrBucketNotEmpty.New("")
	}

	return s.bucketsDB.DeleteBucket(ctx, bucketName, projectID)
}

// IsBucketEmpty returns whether bucket is empty.
func (s *Service) IsBucketEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) (bool, error) {
	prefix, err := CreatePath(ctx, projectID, -1, bucketName, []byte{})
	if err != nil {
		return false, Error.Wrap(err)
	}

	items, _, err := s.List(ctx, prefix.Encode(), "", true, 1, 0)
	if err != nil {
		return false, Error.Wrap(err)
	}
	return len(items) == 0, nil
}

// ListBuckets returns a list of buckets for a project.
func (s *Service) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.ListBuckets(ctx, projectID, listOpts, allowedBuckets)
}

// CountBuckets returns the number of buckets a project currently has.
func (s *Service) CountBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.CountBuckets(ctx, projectID)
}
