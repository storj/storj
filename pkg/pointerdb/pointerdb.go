// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pointerdb/auth"
	pb "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage"
)

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

func (s *Server) validateAuth(APIKeyBytes []byte) error {
	if !auth.ValidateAPIKey(string(APIKeyBytes)) {
		s.logger.Error("unauthorized request: ", zap.Error(grpc.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return grpc.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

// Put formats and hands off a key/value (path/pointer) to be saved to boltdb
func (s *Server) Put(ctx context.Context, putReq *pb.PutRequest) (*pb.PutResponse, error) {
	s.logger.Debug("entering pointerdb put")

	if err := s.validateAuth(putReq.APIKey); err != nil {
		return nil, err
	}

	pointerBytes, err := proto.Marshal(putReq.Pointer)
	if err != nil {
		s.logger.Error("err marshaling pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if err := s.DB.Put(putReq.Path, pointerBytes); err != nil {
		s.logger.Error("err putting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.logger.Debug("put to the db: " + string(putReq.Path))

	return &pb.PutResponse{}, nil
}

// Get formats and hands off a file path to get from boltdb
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	s.logger.Debug("entering pointerdb get")

	APIKeyBytes := []byte(req.APIKey)
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	pointerBytes, err := s.DB.Get(req.Path)
	if err != nil {
		s.logger.Error("err getting file", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.GetResponse{
		Pointer: pointerBytes,
	}, nil
}

// List calls the bolt client's List function and returns all Path keys in the Pointers bucket
func (s *Server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	s.logger.Debug("entering pointerdb list")

	if req.Limit <= 0 {
		return nil, Error.New("err Limit is less than or equal to 0")
	}

	APIKeyBytes := []byte(req.APIKey)
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	var keyList storage.Keys
	if req.StartingPathKey == nil {
		pathKeys, err := s.DB.List(nil, storage.Limit(req.Limit))
		if err != nil {
			s.logger.Error("err listing path keys with no starting key", zap.Error(err))
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		keyList = pathKeys
	} else if req.StartingPathKey != nil {
		pathKeys, err := s.DB.List(storage.Key(req.StartingPathKey), storage.Limit(req.Limit))
		if err != nil {
			s.logger.Error("err listing path keys", zap.Error(err))
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		keyList = pathKeys
	}

	truncated := isItTruncated(keyList, int(req.Limit))

	s.logger.Debug("path keys retrieved")
	return &pb.ListResponse{
		Paths:     keyList.ByteSlices(),
		Truncated: truncated,
	}, nil
}

func isItTruncated(keyList storage.Keys, limit int) bool {
	if len(keyList) == limit {
		return true
	}
	return false
}

// Delete formats and hands off a file path to delete from boltdb
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	s.logger.Debug("entering pointerdb delete")

	APIKeyBytes := []byte(req.APIKey)
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	err := s.DB.Delete(req.Path)
	if err != nil {
		s.logger.Error("err deleting path and pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.logger.Debug("deleted pointer at path: " + string(req.Path))
	return &pb.DeleteResponse{}, nil
}
