// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"storj.io/storj/pkg/identity"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
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

// Endpoint metainfo endpoint
type Endpoint struct {
	log        *zap.Logger
	pointerdb  *pointerdb.Service
	allocation *pointerdb.AllocationSigner
	cache      *overlay.Cache
	apiKeys    APIKeys
	config     pointerdb.Config
	// identity   *identity.FullIdentity
}

// Close closes resources
func (s *Endpoint) Close() error { return nil }

func (s *Endpoint) validateAuth(ctx context.Context) (*console.APIKeyInfo, error) {
	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok {
		s.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	key, err := console.APIKeyFromBase64(string(APIKey))
	if err != nil {
		s.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	keyInfo, err := s.apiKeys.GetByKey(ctx, *key)
	if err != nil {
		s.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	return keyInfo, nil
}

// CreateSegment
func (s *Endpoint) CreateSegment(ctx context.Context, req *pb.SegmentWriteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.OrderLimitResponse{}, nil
}

// CommitSegment
func (s *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.SegmentCommitResponse{}, nil
}

// DownloadSegment DownloadSegment
func (s *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO refactor to use []byte directly
	path := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPath()))
	pointer, err := s.pointerdb.Get(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_INLINE {
		return &pb.SegmentDownloadResponse{Pointer: pointer}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, err := s.getOrderLimits(ctx, pointer.Remote, pb.Action_GET)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.SegmentDownloadResponse{AddressedLimits: limits}, nil
	}

	return &pb.SegmentDownloadResponse{}, nil
}

// DeleteSegment deletes segment metadata from satellite and returns OrderLimit array to remove them from storage node
func (s *Endpoint) DeleteSegment(ctx context.Context, req *pb.SegmentDeleteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO refactor to use []byte directly
	path := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPath()))
	pointer, err := s.pointerdb.Get(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = s.pointerdb.Delete(path)
	if err != nil {
		return &pb.OrderLimitResponse{}, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, err := s.getOrderLimits(ctx, pointer.Remote, pb.Action_DELETE)
		if err != nil {
			return &pb.OrderLimitResponse{}, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.OrderLimitResponse{AddressedLimits: limits}, nil
	}

	return &pb.OrderLimitResponse{}, nil
}

func (s *Endpoint) getOrderLimits(ctx context.Context, remote *pb.RemoteSegment, action pb.Action) ([]*pb.AddressedOrderLimit, error) {
	uplinkIdentity, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}

	pieceID, err := storj.PieceIDFromString(remote.PieceId)
	if err != nil {
		return nil, err
	}

	limits := make([]*pb.AddressedOrderLimit, len(remote.RemotePieces))
	for i, piece := range remote.RemotePieces {
		orderLimit, err := s.allocation.OrderLimit(ctx, uplinkIdentity, piece.NodeId, pieceID, action)
		if err != nil {
			return nil, err
		}

		node, err := s.cache.Get(ctx, piece.NodeId)
		if err != nil {
			return nil, err
		}

		if node != nil {
			node.Type.DPanicOnInvalid("metainfo server order limits")
		}

		limits[i] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}
	return limits, nil
}

// ListSegment returns all Path keys in the Pointers bucket
func (s *Endpoint) ListSegment(ctx context.Context, req *pb.ListSegmentsRequest) (resp *pb.ListSegmentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO refactor to use []byte directly
	prefix := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPrefix()))
	items, more, err := s.pointerdb.List(prefix, string(req.StartAfter), string(req.EndBefore), req.Recursive, req.Limit, req.MetaFlags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ListV2: %v", err)
	}

	segmentItems := make([]*pb.ListSegmentsResponse_Item, len(items))
	for i, item := range items {
		segmentItems[i] = &pb.ListSegmentsResponse_Item{
			Path:     []byte(item.Path),
			Pointer:  item.Pointer,
			IsPrefix: item.IsPrefix,
		}
	}

	return &pb.ListSegmentsResponse{Items: segmentItems, More: more}, nil
}
