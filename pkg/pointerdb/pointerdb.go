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
	pointerdbAuth "storj.io/storj/pkg/pointerdb/auth"
	"storj.io/storj/storage"
)

var (
	mon          = monkit.Package()
	segmentError = errs.Class("segment error")
)

// Server implements the network state RPC service
type Server struct {
	logger     *zap.Logger
	service    *Service
	allocation *AllocationSigner
	cache      *overlay.Cache
	config     Config
	identity   *identity.FullIdentity
}

// NewServer creates instance of Server
func NewServer(logger *zap.Logger, service *Service, allocation *AllocationSigner, cache *overlay.Cache, config Config, identity *identity.FullIdentity) *Server {
	return &Server{
		logger:     logger,
		service:    service,
		allocation: allocation,
		cache:      cache,
		config:     config,
		identity:   identity,
	}
}

// Close closes resources
func (s *Server) Close() error { return nil }

// TODO: ZZZ temporarily disabled until endpoint and service split
const disableAuth = true

func (s *Server) validateAuth(ctx context.Context) error {
	// TODO: ZZZ temporarily disabled until endpoint and service split
	if disableAuth {
		return nil
	}

	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok || !pointerdbAuth.ValidateAPIKey(string(APIKey)) {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
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

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	if err = s.service.Put(req.GetPath(), req.GetPointer()); err != nil {
		s.logger.Error("err putting pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.PutResponse{}, nil
}

// Get formats and hands off a file path to get from boltdb
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (resp *pb.GetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	pointer, err := s.service.Get(req.GetPath())
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

	authorization, err := s.getSignedMessage()
	if err != nil {
		s.logger.Error("err getting signed message", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodes := []*pb.Node{}

	var r = &pb.GetResponse{
		Pointer:       pointer,
		Nodes:         nil,
		Pba:           pba.GetPba(),
		Authorization: authorization,
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
		Pointer:       pointer,
		Nodes:         nodes,
		Pba:           pba.GetPba(),
		Authorization: authorization,
	}

	return r, nil
}

// List returns all Path keys in the Pointers bucket
func (s *Server) List(ctx context.Context, req *pb.ListRequest) (resp *pb.ListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	items, more, err := s.service.List(req.Prefix, req.StartAfter, req.EndBefore, req.Recursive, req.Limit, req.MetaFlags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ListV2: %v", err)
	}

	return &pb.ListResponse{Items: items, More: more}, nil
}

// Delete formats and hands off a file path to delete from boltdb
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (resp *pb.DeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	err = s.service.Delete(req.GetPath())
	if err != nil {
		s.logger.Error("err deleting path and pointer", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.DeleteResponse{}, nil
}

// Iterate iterates over items based on IterateRequest
func (s *Server) Iterate(ctx context.Context, req *pb.IterateRequest, f func(it storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return err
	}

	return s.service.Iterate(req.Prefix, req.First, req.Recurse, req.Reverse, f)
}

// PayerBandwidthAllocation returns PayerBandwidthAllocation struct, signed and with given action type
func (s *Server) PayerBandwidthAllocation(ctx context.Context, req *pb.PayerBandwidthAllocationRequest) (res *pb.PayerBandwidthAllocationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
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

func (s *Server) getSignedMessage() (*pb.SignedMessage, error) {
	signature, err := auth.GenerateSignature(s.identity.ID.Bytes(), s.identity)
	if err != nil {
		return nil, err
	}

	return auth.NewSignedMessage(signature, s.identity)
}
