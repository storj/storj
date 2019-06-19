// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/storage"
)

// Service structure
type Service struct {
	logger *zap.Logger
	DB     storage.KeyValueStore
}

// NewService creates new metainfo service
func NewService(logger *zap.Logger, db storage.KeyValueStore) *Service {
	return &Service{logger: logger, DB: db}
}

// Put puts pointer to db under specific key
func (s *Service) Put(ctx context.Context, path Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Update the pointer with the creation date
	pointer.CreationDate = ptypes.TimestampNow()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		return err
	}

	// TODO(kaloyan): make sure that we know we are overwriting the pointer!
	// In such case we should delete the pieces of the old segment if it was
	// a remote one.
	if err = s.DB.Put(ctx, path.Raw(), pointerBytes); err != nil {
		return err
	}

	return nil
}

// Get gets pointer from db
func (s *Service) Get(ctx context.Context, path Path) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	pointerBytes, err := s.DB.Get(ctx, path.Raw())
	if err != nil {
		return nil, err
	}

	pointer = &pb.Pointer{}
	err = proto.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return nil, errs.New("error unmarshaling pointer: %v", err)
	}

	return pointer, nil
}

// List returns all Path keys in the pointers bucket
func (s *Service) List(ctx context.Context, prefix Path, startAfter, endBefore string, recursive bool, limit int32,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var prefixKey storage.Key
	if prefix.Valid() {
		// We always append the delimiter because a valid Path can never be empty, and
		// always ends with a segment, bucket, or encrypted path, none of which
		// can end with a `/` (except for an encrypted path that is using the null
		// encryption cipher, but consider if you upload to `foo//bar` and list
		// for `foo` vs list for `foo/`. They should contain distinct result sets.)
		prefixKey = append(storage.Key(prefix.Raw()), storage.Delimiter)
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
		return nil, false, err
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
		return err
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
func (s *Service) Delete(ctx context.Context, path Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	return s.DB.Delete(ctx, path.Raw())
}

// Iterate iterates over items in db
func (s *Service) Iterate(ctx context.Context, prefix Path, first Path, recurse bool, reverse bool, f func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	opts := storage.IterateOptions{
		Prefix:  storage.Key(prefix.Raw()),
		First:   storage.Key(first.Raw()),
		Recurse: recurse,
		Reverse: reverse,
	}
	return s.DB.Iterate(ctx, opts, f)
}
