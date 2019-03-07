// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

var (
	mon = monkit.Package()
	// Error general metainfo error
	Error = errs.Class("metainfo error")
)

// APIKeys is api keys store methods used by endpoint
type APIKeys interface {
	GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error)
}

// Endpoint metainfo endpoint
type Endpoint struct {
	log                  *zap.Logger
	pointerdb            *pointerdb.Service
	allocation           *pointerdb.AllocationSigner
	cache                *overlay.Cache
	apiKeys              APIKeys
	pointerDBConfig      pointerdb.Config
	selectionPreferences *overlay.NodeSelectionConfig
}

// NewEndpoint creates new metainfo endpoint instance
func NewEndpoint(log *zap.Logger, pointerdb *pointerdb.Service, allocation *pointerdb.AllocationSigner, cache *overlay.Cache, apiKeys APIKeys, pointerDBConfig pointerdb.Config, selectionPreferences *overlay.NodeSelectionConfig) *Endpoint {
	return &Endpoint{
		log:                  log,
		pointerdb:            pointerdb,
		allocation:           allocation,
		cache:                cache,
		apiKeys:              apiKeys,
		pointerDBConfig:      pointerDBConfig,
		selectionPreferences: selectionPreferences,
	}
}

// Close closes resources
func (endpoint *Endpoint) Close() error { return nil }

func (endpoint *Endpoint) validateAuth(ctx context.Context) (*console.APIKeyInfo, error) {
	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok {
		endpoint.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	key, err := console.APIKeyFromBase64(string(APIKey))
	if err != nil {
		endpoint.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	keyInfo, err := endpoint.apiKeys.GetByKey(ctx, *key)
	if err != nil {
		endpoint.log.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	return keyInfo, nil
}

// SegmentInfo returns segment metadata info
func (endpoint *Endpoint) SegmentInfo(ctx context.Context, req *pb.SegmentInfoRequest) (resp *pb.SegmentInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validatePathElements(req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPath()))
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentInfoResponse{Pointer: pointer}, nil
}

// CreateSegment will generate requested number of OrderLimit with coresponding node addresses for them
func (endpoint *Endpoint) CreateSegment(ctx context.Context, req *pb.SegmentWriteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO most probably needs more params
	request := &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount: int64(req.Redundancy.Total),
		},
	}
	nodes, err := endpoint.cache.FindStorageNodes(ctx, request, endpoint.selectionPreferences)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	uplinkIdentity, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	pieceID := storj.NewPieceID()
	limits := make([]*pb.AddressedOrderLimit, len(nodes))
	for i, node := range nodes {
		orderLimit, err := endpoint.createOrderLimit(ctx, uplinkIdentity, node.Id, pieceID, req.Expiration, req.MaxSegmentSize, pb.Action_PUT)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		limits[i] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return &pb.OrderLimitResponse{AddressedLimits: limits}, nil
}

// CommitSegment commits segment metadata
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validatePathElements(req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO validate pointer with OriginalOrderLimits

	err = endpoint.filterValidPieces(req.Pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	path := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPath()))
	err = endpoint.pointerdb.Put(path, req.GetPointer())
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// TODO should this be Pointer from request or DB
	return &pb.SegmentCommitResponse{Pointer: req.GetPointer()}, nil
}

// DownloadSegment gets Pointer incase of INLINE data or list of OrderLimit necessary to download remote data
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validatePathElements(req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	path := storj.JoinPaths(keyInfo.ProjectID.String(), strconv.FormatInt(req.Segment, 10), string(req.GetBucket()), string(req.GetPath()))
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_INLINE {
		return &pb.SegmentDownloadResponse{Pointer: pointer}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, err := endpoint.createOrderLimitsForSegment(ctx, pointer.Remote, pb.Action_GET)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.SegmentDownloadResponse{Pointer: pointer, AddressedLimits: limits}, nil
	}

	return &pb.SegmentDownloadResponse{}, nil
}

// DeleteSegment deletes segment metadata from satellite and returns OrderLimit array to remove them from storage node
func (endpoint *Endpoint) DeleteSegment(ctx context.Context, req *pb.SegmentDeleteRequest) (resp *pb.OrderLimitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validatePathElements(req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	path := storj.JoinPaths(keyInfo.ProjectID.String(), strconv.FormatInt(req.Segment, 10), string(req.GetBucket()), string(req.GetPath()))
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = endpoint.pointerdb.Delete(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, err := endpoint.createOrderLimitsForSegment(ctx, pointer.Remote, pb.Action_DELETE)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.OrderLimitResponse{AddressedLimits: limits}, nil
	}

	return &pb.OrderLimitResponse{}, nil
}

func (endpoint *Endpoint) createOrderLimitsForSegment(ctx context.Context, remote *pb.RemoteSegment, action pb.Action) ([]*pb.AddressedOrderLimit, error) {
	if remote == nil {
		return nil, nil
	}

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
		orderLimit, err := endpoint.createOrderLimit(ctx, uplinkIdentity, piece.NodeId, pieceID, nil, 0, action)
		if err != nil {
			return nil, err
		}

		node, err := endpoint.cache.Get(ctx, piece.NodeId)
		if err != nil {
			return nil, err
		}

		if node != nil {
			node.Type.DPanicOnInvalid("metainfo server order limits")
		}

		limits[i] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.GetAddress(),
		}

	}
	return limits, nil
}

func (endpoint *Endpoint) createOrderLimit(ctx context.Context, uplinkIdentity *identity.PeerIdentity, nodeID storj.NodeID, pieceID pb.PieceID, expiration *timestamp.Timestamp, limit int64, action pb.Action) (*pb.OrderLimit2, error) {
	parameters := pointerdb.OrderLimitParameters{
		UplinkIdentity:  uplinkIdentity,
		StorageNodeID:   nodeID,
		PieceID:         pieceID,
		Action:          action,
		PieceExpiration: expiration,
		Limit:           limit,
	}

	orderLimit, err := endpoint.allocation.OrderLimit(ctx, parameters)
	if err != nil {
		return nil, err
	}
	return orderLimit, nil
}

// ListSegments returns all Path keys in the Pointers bucket
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.ListSegmentsRequest) (resp *pb.ListSegmentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO refactor to use []byte directly
	prefix := storj.JoinPaths(keyInfo.ProjectID.String(), string(req.GetBucket()), string(req.GetPrefix()))
	items, more, err := endpoint.pointerdb.List(prefix, string(req.StartAfter), string(req.EndBefore), req.Recursive, req.Limit, req.MetaFlags)
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

func (endpoint *Endpoint) filterValidPieces(pointer *pb.Pointer) error {
	if pointer.Type == pb.Pointer_REMOTE {
		var remotePieces []*pb.RemotePiece
		remote := pointer.Remote
		for _, piece := range remote.RemotePieces {
			err := auth.VerifyMsg(piece.Hash, piece.NodeId)
			if err == nil {
				// set to nil after verification to avoid storing in DB
				piece.Hash.SetCerts(nil)
				piece.Hash.SetSignature(nil)
				remotePieces = append(remotePieces, piece)
			} else {
				// TODO satellite should send Delete request for piece that failed
				endpoint.log.Warn("unable to verify piece hash: %v", zap.Error(err))
			}
		}

		if int32(len(remotePieces)) < remote.Redundancy.SuccessThreshold {
			return Error.New("Number of valid pieces is lower then success threshold: %v < %v",
				len(remotePieces),
				remote.Redundancy.SuccessThreshold,
			)
		}

		remote.RemotePieces = remotePieces
	}
	return nil
}

func (endpoint *Endpoint) validatePathElements(bucket, path []byte) error {
	if len(bucket) == 0 {
		return errs.New("bucket not specified")
	}
	if len(path) == 0 {
		return errs.New("path not specified")
	}
	return nil
}
