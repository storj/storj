// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pointerdb/auth"
	pb "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage"
)

// ListPageLimit is the maximum number of items that will be returned by a list
// request.
// TODO(kaloyan): make it configurable
const ListPageLimit = 1000

// Server implements the network state RPC service
type Server struct {
	DB     storage.KeyValueStore
	logger *zap.Logger
}

// NewServer creates instance of Server
func NewServer(db storage.KeyValueStore, logger *zap.Logger) *Server {
	return &Server{
		DB:     db,
		logger: logger,
	}
}

func (s *Server) validateAuth(APIKey []byte) error {
	if !auth.ValidateAPIKey(string(APIKey)) {
		s.logger.Error("unauthorized request: ", zap.Error(grpc.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return grpc.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

// Put formats and hands off a key/value (path/pointer) to be saved to boltdb
func (s *Server) Put(ctx context.Context, req *pb.PutRequest) (resp *pb.PutResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering pointerdb put")

	if err = s.validateAuth(req.GetAPIKey()); err != nil {
		return nil, err
	}

	// Update the pointer with the creation date
	req.GetPointer().CreationDate = ptypes.TimestampNow()

	pointerBytes, err := proto.Marshal(req.GetPointer())
	if err != nil {
		s.logger.Error("err marshaling pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// TODO(kaloyan): make sure that we know we are overwriting the pointer!
	// In such case we should delete the pieces of the old segment if it was
	// a remote one.
	if err = s.DB.Put([]byte(req.GetPath()), pointerBytes); err != nil {
		s.logger.Error("err putting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.logger.Debug("put to the db: " + string(req.GetPath()))

	return &pb.PutResponse{}, nil
}

// Get formats and hands off a file path to get from boltdb
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (resp *pb.GetResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering pointerdb get")

	if err = s.validateAuth(req.GetAPIKey()); err != nil {
		return nil, err
	}

	pointerBytes, err := s.DB.Get([]byte(req.GetPath()))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		s.logger.Error("err getting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.GetResponse{
		Pointer: pointerBytes,
	}, nil
}

// List calls the bolt client's List function and returns all Path keys in the Pointers bucket
func (s *Server) List(ctx context.Context, req *pb.ListRequest) (resp *pb.ListResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering pointerdb list")

	limit := int(req.GetLimit())
	if limit <= 0 || limit > ListPageLimit {
		limit = ListPageLimit
	}

	if err = s.validateAuth(req.GetAPIKey()); err != nil {
		return nil, err
	}

	prefix := paths.New(req.GetPrefix())

	// TODO(kaloyan): here we query the DB without limit. We must optimize it!
	keys, err := s.DB.List([]byte(req.GetPrefix()+"/"), 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	var more bool
	var items []*pb.ListResponse_Item
	if req.GetEndBefore() != "" && req.GetStartAfter() == "" {
		items, more = s.processKeysBackwards(ctx, keys, prefix,
			req.GetEndBefore(), req.GetRecursive(), limit, req.GetMetaFlags())
	} else {
		items, more = s.processKeysForwards(ctx, keys, prefix, req.GetStartAfter(),
			req.GetEndBefore(), req.GetRecursive(), limit, req.GetMetaFlags())
	}

	s.logger.Debug("path keys retrieved")
	return &pb.ListResponse{Items: items, More: more}, nil
}

// processKeysForwards iterates forwards through given keys, and returns them
// as list items
func (s *Server) processKeysForwards(ctx context.Context, keys storage.Keys,
	prefix paths.Path, startAfter, endBefore string, recursive bool, limit int,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool) {
	skip := startAfter != ""
	startAfterPath := prefix.Append(startAfter)
	endBeforePath := prefix.Append(endBefore)

	for _, key := range keys {
		p := paths.New(string(key))

		if skip {
			if reflect.DeepEqual(p, startAfterPath) {
				// TODO(kaloyan): Better check - what if there is no path equal to startAfter?
				// TODO(kaloyan): Add Equal method in Path type
				skip = false
			}
			continue
		}

		// TODO(kaloyan): Better check - what if there is no path equal to endBefore?
		// TODO(kaloyan): Add Equal method in Path type
		if reflect.DeepEqual(p, endBeforePath) {
			break
		}

		if !p.HasPrefix(prefix) {
			// We went through all keys that start with the prefix
			break
		}

		if !recursive && len(p) > len(prefix)+1 {
			continue
		}

		item := s.createListItem(ctx, p, metaFlags)
		items = append(items, item)

		if len(items) == limit {
			more = true
			break
		}
	}
	return items, more
}

// processKeysBackwards iterates backwards through given keys, and returns them
// as list items
func (s *Server) processKeysBackwards(ctx context.Context, keys storage.Keys,
	prefix paths.Path, endBefore string, recursive bool, limit int,
	metaFlags uint32) (items []*pb.ListResponse_Item, more bool) {
	skip := endBefore != ""
	endBeforePath := prefix.Append(endBefore)

	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		p := paths.New(string(key))

		if skip {
			if reflect.DeepEqual(p, endBeforePath) {
				// TODO(kaloyan): Better check - what if there is no path equal to endBefore?
				// TODO(kaloyan): Add Equal method in Path type
				skip = false
			}
			continue
		}

		if !p.HasPrefix(prefix) || len(p) <= len(prefix) {
			// We went through all keys that start with the prefix
			break
		}

		if !recursive && len(p) > len(prefix)+1 {
			continue
		}

		item := s.createListItem(ctx, p, metaFlags)
		items = append([]*pb.ListResponse_Item{item}, items...)

		if len(items) == limit {
			more = true
			break
		}
	}
	return items, more
}

// createListItem creates a new list item with the given path. It also adds
// the metadata according to the given metaFlags.
func (s *Server) createListItem(ctx context.Context, p paths.Path,
	metaFlags uint32) *pb.ListResponse_Item {
	item := &pb.ListResponse_Item{Path: p.String()}
	err := s.getMetadata(ctx, item, metaFlags)
	if err != nil {
		s.logger.Warn("err retrieving metadata", zap.Error(err))
	}
	return item
}

// getMetadata adds the metadata to the given item pointer according to the
// given metaFlags
func (s *Server) getMetadata(ctx context.Context, item *pb.ListResponse_Item,
	metaFlags uint32) (err error) {
	defer mon.Task()(&ctx)(&err)

	if metaFlags == meta.None {
		return nil
	}

	b, err := s.DB.Get([]byte(item.GetPath()))
	if err != nil {
		return err
	}

	pr := &pb.Pointer{}
	err = proto.Unmarshal(b, pr)
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
		item.Pointer.Size = pr.GetSize()
	}
	if metaFlags&meta.UserDefined != 0 {
		item.Pointer.Metadata = pr.GetMetadata()
	}

	return nil
}

// Delete formats and hands off a file path to delete from boltdb
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (resp *pb.DeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering pointerdb delete")

	if err = s.validateAuth(req.GetAPIKey()); err != nil {
		return nil, err
	}

	err = s.DB.Delete([]byte(req.GetPath()))
	if err != nil {
		s.logger.Error("err deleting path and pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.logger.Debug("deleted pointer at path: " + string(req.GetPath()))
	return &pb.DeleteResponse{}, nil
}
