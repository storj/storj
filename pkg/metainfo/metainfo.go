// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/console"
)

var (
	mon          = monkit.Package()
	segmentError = errs.Class("metainfo error")
)

// APIKeys is api keys store methods used by endpoint
type APIKeys interface {
	GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error)
}

type Server struct {
	logger *zap.Logger
	// pointerdb  *pointerdb.Service
	// allocation *pointerdb.AllocationSigner
	// cache      *overlay.Cache
	apiKeys APIKeys
	// config     Config
	// identity   *identity.FullIdentity
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

func (s *Server) CreateSegment(ctx context.Context, req *pb.SegmentWriteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.OrderLimitResponse{}, nil
}

func (s *Server) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.SegmentCommitResponse{}, nil
}

func (s *Server) DownloadSegment(ctx context.Context, req *pb.SegementDownloadRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.OrderLimitResponse{}, nil
}

func (s *Server) DeleteSegment(ctx context.Context, req *pb.SegmentDeleteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.OrderLimitResponse{}, nil
}
