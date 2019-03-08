// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skyrings/skyring-common/tools/uuid"
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
	"storj.io/storj/storage"
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

	err = endpoint.validateBucket(req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path, err := endpoint.createPath(keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentInfoResponse{Pointer: pointer}, nil
}

// CreateSegment will generate requested number of OrderLimit with coresponding node addresses for them
func (endpoint *Endpoint) CreateSegment(ctx context.Context, req *pb.SegmentWriteRequest) (resp *pb.SegmentWriteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO most probably needs more params
	request := &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount:       int64(req.Redundancy.Total),
			Restrictions: &pb.NodeRestrictions{},
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

	rootPieceID := storj.NewPieceID()
	limits := make([]*pb.AddressedOrderLimit, len(nodes))
	for i, node := range nodes {
		derivedPieceID := rootPieceID.Derive(node.Id)
		orderLimit, err := endpoint.createOrderLimit(ctx, uplinkIdentity, node.Id, derivedPieceID, req.Expiration, req.MaxSegmentSize, pb.Action_PUT)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		limits[i] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return &pb.SegmentWriteResponse{AddressedLimits: limits, RootPieceId: rootPieceID}, nil
}

// CommitSegment commits segment metadata
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.validateCommit(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = endpoint.filterValidPieces(req.Pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	path, err := endpoint.createPath(keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.pointerdb.Put(path, req.Pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// TODO should this be Pointer from request or DB?
	return &pb.SegmentCommitResponse{Pointer: req.Pointer}, nil
}

// DownloadSegment gets Pointer incase of INLINE data or list of OrderLimit necessary to download remote data
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path, err := endpoint.createPath(keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
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
func (endpoint *Endpoint) DeleteSegment(ctx context.Context, req *pb.SegmentDeleteRequest) (resp *pb.SegmentDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path, err := endpoint.createPath(keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.pointerdb.Get(path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
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
		return &pb.SegmentDeleteResponse{AddressedLimits: limits}, nil
	}

	return &pb.SegmentDeleteResponse{}, nil
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

	limits := make([]*pb.AddressedOrderLimit, remote.Redundancy.Total)
	for _, piece := range remote.RemotePieces {
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

		limits[piece.PieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
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

	prefix, err := endpoint.createPath(keyInfo.ProjectID, -1, req.Bucket, req.Prefix)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

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

func (endpoint *Endpoint) createPath(projectID uuid.UUID, segmentIndex int64, bucket, path []byte) (string, error) {
	if segmentIndex < -1 {
		return "", Error.New("invalid segment index")
	}
	segment := "l"
	if segmentIndex > -1 {
		segment = "s" + strconv.FormatInt(segmentIndex, 10)
	}
	return storj.JoinPaths(projectID.String(), segment, string(bucket), string(path)), nil
}

func (endpoint *Endpoint) filterValidPieces(pointer *pb.Pointer) error {
	if pointer.Type == pb.Pointer_REMOTE {
		var remotePieces []*pb.RemotePiece
		remote := pointer.Remote
		for _, piece := range remote.RemotePieces {
			err := auth.VerifyMsg(piece.Hash, piece.NodeId)
			if err == nil {
				// set to nil after verification to avoid storing in DB
				piece.Hash = nil
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

func (endpoint *Endpoint) validateBucket(bucket []byte) error {
	if len(bucket) == 0 {
		return errs.New("bucket not specified")
	}
	return nil
}

func (endpoint *Endpoint) validateCommit(req *pb.SegmentCommitRequest) error {
	err := endpoint.validatePointer(req.Pointer)
	if err != nil {
		return err
	}

	if req.Pointer.Type == pb.Pointer_REMOTE {
		remote := req.Pointer.Remote

		if int32(len(req.OriginalLimits)) != remote.Redundancy.Total {
			return Error.New("invalid no order limit for piece")
		}

		for _, piece := range remote.RemotePieces {
			limit := req.OriginalLimits[piece.PieceNum]

			// TODO verify limit signature

			if limit == nil {
				return Error.New("invalid no order limit for piece")
			}
			if limit.PieceId.IsZero() || limit.PieceId.String() != remote.PieceId {
				return Error.New("invalid order limit piece id")
			}
			if bytes.Compare(piece.NodeId.Bytes(), limit.StorageNodeId.Bytes()) != 0 {
				return Error.New("piece NodeID != order limit NodeID")
			}
		}
	}
	return nil
}

func (endpoint *Endpoint) validatePointer(pointer *pb.Pointer) error {
	if pointer == nil {
		return Error.New("no pointer specified")
	}

	// TODO does it all?
	if pointer.Type == pb.Pointer_REMOTE {
		if pointer.Remote == nil {
			return Error.New("no remote segment specified")
		}
		if pointer.Remote.RemotePieces == nil {
			return Error.New("no remote segment pieces specified")
		}
		if pointer.Remote.Redundancy == nil {
			return Error.New("no redundancy scheme specified")
		}
	}
	return nil
}
