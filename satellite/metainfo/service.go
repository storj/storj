// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
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

// Put puts pointer to db under specific path
func (s *Service) Put(ctx context.Context, path string, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Update the pointer with the creation date
	pointer.CreationDate = time.Now()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		return err
	}

	// TODO(kaloyan): make sure that we know we are overwriting the pointer!
	// In such case we should delete the pieces of the old segment if it was
	// a remote one.
	if err = s.DB.Put(ctx, []byte(path), pointerBytes); err != nil {
		return err
	}

	return nil
}

// Get gets pointer from db
func (s *Service) Get(ctx context.Context, path string) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)
	pointerBytes, err := s.DB.Get(ctx, []byte(path))
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
