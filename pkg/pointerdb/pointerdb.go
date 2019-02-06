// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	_ "storj.io/storj/pkg/pointerdb/auth" // ensures that we add api key flag to current executable
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"storj.io/storj/storage"
)

var (
	mon          = monkit.Package()
	segmentError = errs.Class("segment error")
)

// APIKeys is api keys store methods used by pointerdb
type APIKeys interface {
	GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error)
}

// Server implements the network state RPC service
type Server struct {
	logger     *zap.Logger
	service    *Service
	allocation *AllocationSigner
	cache      *overlay.Cache
	config     Config
	identity   *identity.FullIdentity
	apiKeys    APIKeys
}

// NewServer creates instance of Server
func NewServer(logger *zap.Logger, service *Service, allocation *AllocationSigner, cache *overlay.Cache, config Config, identity *identity.FullIdentity, apiKeys APIKeys) *Server {
	return &Server{
		logger:     logger,
		service:    service,
		allocation: allocation,
		cache:      cache,
		config:     config,
		identity:   identity,
		apiKeys:    apiKeys,
	}
}

// Close closes resources
func (s *Server) Close() error { return nil }

func (s *Server) validateAuth(ctx context.Context) (*console.APIKeyInfo, error) {
	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	key, err := console.APIKeyFromBase64(string(APIKey))
	if err != nil {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	keyInfo, err := s.apiKeys.GetByKey(ctx, *key)
	if err != nil {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	return keyInfo, nil
}

func (s *Server) validateSegment(req *pb.PutRequest) error {
	min := s.config.MinRemoteSegmentSize
	remote := req.GetPointer().Remote
	remoteSize := req.GetPointer().GetSegmentSize()

	if remote != nil && remoteSize < int64(min) {
		return segmentError.New("remote segment size %d less than minimum allowed %d", remoteSize, min)
	}

	max := s.config.MaxInlineSegmentSize.Int()
	inlineSize := len(req.GetPointer().InlineSegment)

	if inlineSize > max {
		return segmentError.New("inline segment size %d greater than maximum allowed %d", inlineSize, max)
	}

	return nil
}

// Put formats and hands off a key/value (path/pointer) to be saved to boltdb
func (s *Server) Put(ctx context.Context, req *pb.PutRequest) (resp *pb.PutResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.validateSegment(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	path := storj.JoinPaths(keyInfo.ProjectID.String(), req.GetPath())
	if err = s.service.Put(path, req.GetPointer()); err != nil {
		s.logger.Error("err putting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.PutResponse{}, nil
}

// Get formats and hands off a file path to get from boltdb
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (resp *pb.GetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	path := storj.JoinPaths(keyInfo.ProjectID.String(), req.GetPath())
	pointer, err := s.service.Get(path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		s.logger.Error("err getting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	pba, err := s.PayerBandwidthAllocation(ctx, &pb.PayerBandwidthAllocationRequest{Action: pb.BandwidthAction_GET})
	if err != nil {
		s.logger.Error("err getting payer bandwidth allocation", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodes := []*pb.Node{}

	var r = &pb.GetResponse{
		Pointer: pointer,
		Nodes:   nil,
		Pba:     pba.GetPba(),
	}

	if !s.config.Overlay || pointer.Remote == nil {
		return r, nil
	}

	for _, piece := range pointer.Remote.RemotePieces {
		node, err := s.cache.Get(ctx, piece.NodeId)
		if err != nil {
			s.logger.Error("Error getting node from cache", zap.String("ID", piece.NodeId.String()), zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	for _, v := range nodes {
		if v != nil {
			v.Type.DPanicOnInvalid("pdb server Get")
		}
	}
	r = &pb.GetResponse{
		Pointer: pointer,
		Nodes:   nodes,
		Pba:     pba.GetPba(),
	}

	return r, nil
}

// List returns all Path keys in the Pointers bucket
func (s *Server) List(ctx context.Context, req *pb.ListRequest) (resp *pb.ListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	prefix := storj.JoinPaths(keyInfo.ProjectID.String(), req.Prefix)
	items, more, err := s.service.List(prefix, req.StartAfter, req.EndBefore, req.Recursive, req.Limit, req.MetaFlags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ListV2: %v", err)
	}

	return &pb.ListResponse{Items: items, More: more}, nil
}

// Delete formats and hands off a file path to delete from boltdb
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (resp *pb.DeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	path := storj.JoinPaths(keyInfo.ProjectID.String(), req.GetPath())
	err = s.service.Delete(path)
	if err != nil {
		s.logger.Error("err deleting path and pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.DeleteResponse{}, nil
}

// Iterate iterates over items based on IterateRequest
func (s *Server) Iterate(ctx context.Context, req *pb.IterateRequest, f func(it storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return err
	}

	prefix := storj.JoinPaths(keyInfo.ProjectID.String(), req.Prefix)
	return s.service.Iterate(prefix, req.First, req.Recurse, req.Reverse, f)
}

// PayerBandwidthAllocation returns PayerBandwidthAllocation struct, signed and with given action type
func (s *Server) PayerBandwidthAllocation(ctx context.Context, req *pb.PayerBandwidthAllocationRequest) (res *pb.PayerBandwidthAllocationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO(michal) should be replaced with renter id when available
	// retrieve the public key
	pi, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}

	pba, err := s.allocation.PayerBandwidthAllocation(ctx, pi, req.GetAction())
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.PayerBandwidthAllocationResponse{Pba: pba}, nil
}
