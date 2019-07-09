// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/storage"
)

const pieceHashExpiration = 2 * time.Hour

var (
	mon = monkit.Package()
	// Error general metainfo error
	Error = errs.Class("metainfo error")
)

// APIKeys is api keys store methods used by endpoint
type APIKeys interface {
	GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error)
}

// Revocations is the revocations store methods used by the endpoint
type Revocations interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([][]byte, error)
}

// Containment is a copy/paste of containment interface to avoid import cycle error
type Containment interface {
	Delete(ctx context.Context, nodeID pb.NodeID) (bool, error)
}

// Endpoint metainfo endpoint
type Endpoint struct {
	log            *zap.Logger
	metainfo       *Service
	orders         *orders.Service
	cache          *overlay.Cache
	partnerinfo    attribution.DB
	projectUsage   *accounting.ProjectUsage
	containment    Containment
	apiKeys        APIKeys
	createRequests *createRequests
	rsConfig       RSConfig
}

// NewEndpoint creates new metainfo endpoint instance
func NewEndpoint(log *zap.Logger, metainfo *Service, orders *orders.Service, cache *overlay.Cache, partnerinfo attribution.DB,
	containment Containment, apiKeys APIKeys, projectUsage *accounting.ProjectUsage, rsConfig RSConfig) *Endpoint {
	// TODO do something with too many params
	return &Endpoint{
		log:            log,
		metainfo:       metainfo,
		orders:         orders,
		cache:          cache,
		partnerinfo:    partnerinfo,
		containment:    containment,
		apiKeys:        apiKeys,
		projectUsage:   projectUsage,
		createRequests: newCreateRequests(),
		rsConfig:       rsConfig,
	}
}

// Close closes resources
func (endpoint *Endpoint) Close() error { return nil }

// SegmentInfoOld returns segment metadata info
func (endpoint *Endpoint) SegmentInfoOld(ctx context.Context, req *pb.SegmentInfoRequestOld) (resp *pb.SegmentInfoResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.Path,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.metainfo.Get(ctx, path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentInfoResponseOld{Pointer: pointer}, nil
}

// CreateSegmentOld will generate requested number of OrderLimit with coresponding node addresses for them
func (endpoint *Endpoint) CreateSegmentOld(ctx context.Context, req *pb.SegmentWriteRequestOld) (resp *pb.SegmentWriteResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	if !req.Expiration.IsZero() && !req.Expiration.After(time.Now()) {
		return nil, errs.New("Invalid expiration time")
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.Path,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.validateRedundancy(ctx, req.Redundancy)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	exceeded, limit, err := endpoint.projectUsage.ExceedsStorageUsage(ctx, keyInfo.ProjectID)
	if err != nil {
		endpoint.log.Error("retrieving project storage totals", zap.Error(err))
	}
	if exceeded {
		endpoint.log.Sugar().Errorf("monthly project limits are %s of storage and bandwidth usage. This limit has been exceeded for storage for projectID %s",
			limit, keyInfo.ProjectID,
		)
		return nil, status.Errorf(codes.ResourceExhausted, "Exceeded Usage Limit")
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(req.GetRedundancy())
	if err != nil {
		return nil, err
	}

	maxPieceSize := eestream.CalcPieceSize(req.GetMaxEncryptedSegmentSize(), redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: int(req.Redundancy.Total),
		FreeBandwidth:  maxPieceSize,
		FreeDisk:       maxPieceSize,
	}
	nodes, err := endpoint.cache.FindStorageNodes(ctx, request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	uplinkIdentity, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	bucketID := createBucketID(keyInfo.ProjectID, req.Bucket)
	rootPieceID, addressedLimits, err := endpoint.orders.CreatePutOrderLimits(ctx, uplinkIdentity, bucketID, nodes, req.Expiration, maxPieceSize)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if len(addressedLimits) > 0 {
		endpoint.createRequests.Put(addressedLimits[0].Limit.SerialNumber, &createRequest{
			Expiration: req.Expiration,
			Redundancy: req.Redundancy,
		})
	}

	return &pb.SegmentWriteResponseOld{AddressedLimits: addressedLimits, RootPieceId: rootPieceID}, nil
}

func calculateSpaceUsed(ptr *pb.Pointer) (inlineSpace, remoteSpace int64) {
	inline := ptr.GetInlineSegment()
	if inline != nil {
		return int64(len(inline)), 0
	}
	segmentSize := ptr.GetSegmentSize()
	remote := ptr.GetRemote()
	if remote == nil {
		return 0, 0
	}
	minReq := remote.GetRedundancy().GetMinReq()
	pieceSize := segmentSize / int64(minReq)
	pieces := remote.GetRemotePieces()
	return 0, pieceSize * int64(len(pieces))
}

// CommitSegmentOld commits segment metadata
func (endpoint *Endpoint) CommitSegmentOld(ctx context.Context, req *pb.SegmentCommitRequestOld) (resp *pb.SegmentCommitResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.Path,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.validateCommitSegment(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = endpoint.filterValidPieces(ctx, req.Pointer, req.OriginalLimits)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	exceeded, limit, err := endpoint.projectUsage.ExceedsStorageUsage(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	if exceeded {
		endpoint.log.Sugar().Errorf("monthly project limits are %s of storage and bandwidth usage. This limit has been exceeded for storage for projectID %s.",
			limit, keyInfo.ProjectID,
		)
		return nil, status.Errorf(codes.ResourceExhausted, "Exceeded Usage Limit")
	}

	// clear hashes so we don't store them
	for _, piece := range req.GetPointer().GetRemote().GetRemotePieces() {
		piece.Hash = nil
	}

	inlineUsed, remoteUsed := calculateSpaceUsed(req.Pointer)

	// ToDo: Replace with hash & signature validation
	// Ensure neither uplink or storage nodes are cheating on us
	if req.Pointer.Type == pb.Pointer_REMOTE {
		//We cannot have more redundancy than total/min
		if float64(remoteUsed) > (float64(req.Pointer.SegmentSize)/float64(req.Pointer.Remote.Redundancy.MinReq))*float64(req.Pointer.Remote.Redundancy.Total) {
			endpoint.log.Sugar().Debugf("data size mismatch, got segment: %d, pieces: %d, RS Min, Total: %d,%d", req.Pointer.SegmentSize, remoteUsed, req.Pointer.Remote.Redundancy.MinReq, req.Pointer.Remote.Redundancy.Total)
			return nil, status.Errorf(codes.InvalidArgument, "mismatched segment size and piece usage")
		}
	}

	if err := endpoint.projectUsage.AddProjectStorageUsage(ctx, keyInfo.ProjectID, inlineUsed, remoteUsed); err != nil {
		endpoint.log.Sugar().Errorf("Could not track new storage usage by project %v: %v", keyInfo.ProjectID, err)
		// but continue. it's most likely our own fault that we couldn't track it, and the only thing
		// that will be affected is our per-project bandwidth and storage limits.
	}

	err = endpoint.metainfo.Put(ctx, path, req.Pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if req.Pointer.Type == pb.Pointer_INLINE {
		// TODO or maybe use pointer.SegmentSize ??
		err = endpoint.orders.UpdatePutInlineOrder(ctx, keyInfo.ProjectID, req.Bucket, int64(len(req.Pointer.InlineSegment)))
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
	}

	pointer, err := endpoint.metainfo.Get(ctx, path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if len(req.OriginalLimits) > 0 {
		endpoint.createRequests.Remove(req.OriginalLimits[0].SerialNumber)
	}

	return &pb.SegmentCommitResponseOld{Pointer: pointer}, nil
}

// DownloadSegmentOld gets Pointer incase of INLINE data or list of OrderLimit necessary to download remote data
func (endpoint *Endpoint) DownloadSegmentOld(ctx context.Context, req *pb.SegmentDownloadRequestOld) (resp *pb.SegmentDownloadResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.Path,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	bucketID := createBucketID(keyInfo.ProjectID, req.Bucket)

	exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfo.ProjectID, bucketID)
	if err != nil {
		endpoint.log.Error("retrieving project bandwidth total", zap.Error(err))
	}
	if exceeded {
		endpoint.log.Sugar().Errorf("monthly project limits are %s of storage and bandwidth usage. This limit has been exceeded for bandwidth for projectID %s.",
			limit, keyInfo.ProjectID,
		)
		return nil, status.Errorf(codes.ResourceExhausted, "Exceeded Usage Limit")
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.metainfo.Get(ctx, path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_INLINE {
		// TODO or maybe use pointer.SegmentSize ??
		err := endpoint.orders.UpdateGetInlineOrder(ctx, keyInfo.ProjectID, req.Bucket, int64(len(pointer.InlineSegment)))
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.SegmentDownloadResponseOld{Pointer: pointer}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		uplinkIdentity, err := identity.PeerIdentityFromContext(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		limits, err := endpoint.orders.CreateGetOrderLimits(ctx, uplinkIdentity, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.SegmentDownloadResponseOld{Pointer: pointer, AddressedLimits: limits}, nil
	}

	return &pb.SegmentDownloadResponseOld{}, nil
}

// DeleteSegmentOld deletes segment metadata from satellite and returns OrderLimit array to remove them from storage node
func (endpoint *Endpoint) DeleteSegmentOld(ctx context.Context, req *pb.SegmentDeleteRequestOld) (resp *pb.SegmentDeleteResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        req.Bucket,
		EncryptedPath: req.Path,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// TODO refactor to use []byte directly
	pointer, err := endpoint.metainfo.Get(ctx, path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = endpoint.metainfo.Delete(ctx, path)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		uplinkIdentity, err := identity.PeerIdentityFromContext(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			_, err := endpoint.containment.Delete(ctx, piece.NodeId)
			if err != nil {
				return nil, status.Errorf(codes.Internal, err.Error())
			}
		}

		bucketID := createBucketID(keyInfo.ProjectID, req.Bucket)
		limits, err := endpoint.orders.CreateDeleteOrderLimits(ctx, uplinkIdentity, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.SegmentDeleteResponseOld{AddressedLimits: limits}, nil
	}

	return &pb.SegmentDeleteResponseOld{}, nil
}

// ListSegmentsOld returns all Path keys in the Pointers bucket
func (endpoint *Endpoint) ListSegmentsOld(ctx context.Context, req *pb.ListSegmentsRequestOld) (resp *pb.ListSegmentsResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: req.Prefix,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	prefix, err := CreatePath(ctx, keyInfo.ProjectID, -1, req.Bucket, req.Prefix)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	items, more, err := endpoint.metainfo.List(ctx, prefix, string(req.StartAfter), string(req.EndBefore), req.Recursive, req.Limit, req.MetaFlags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ListV2: %v", err)
	}

	segmentItems := make([]*pb.ListSegmentsResponseOld_Item, len(items))
	for i, item := range items {
		segmentItems[i] = &pb.ListSegmentsResponseOld_Item{
			Path:     []byte(item.Path),
			Pointer:  item.Pointer,
			IsPrefix: item.IsPrefix,
		}
	}

	return &pb.ListSegmentsResponseOld{Items: segmentItems, More: more}, nil
}

func createBucketID(projectID uuid.UUID, bucket []byte) []byte {
	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, string(bucket))
	return []byte(storj.JoinPaths(entries...))
}

func (endpoint *Endpoint) filterValidPieces(ctx context.Context, pointer *pb.Pointer, limits []*pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pointer.Type == pb.Pointer_REMOTE {
		var remotePieces []*pb.RemotePiece
		remote := pointer.Remote
		allSizesValid := true
		lastPieceSize := int64(0)
		for _, piece := range remote.RemotePieces {
			// TODO enable verification

			// err := auth.VerifyMsg(piece.Hash, piece.NodeId)
			// if err == nil {
			// 	remotePieces = append(remotePieces, piece)
			// } else {
			// 	// TODO satellite should send Delete request for piece that failed
			// 	s.logger.Warn("unable to verify piece hash: %v", zap.Error(err))
			// }

			err = endpoint.validatePieceHash(ctx, piece, limits)
			if err != nil {
				// TODO maybe this should be logged also to uplink too
				endpoint.log.Sugar().Warn(err)
				continue
			}

			if piece.Hash.PieceSize <= 0 || (lastPieceSize > 0 && lastPieceSize != piece.Hash.PieceSize) {
				allSizesValid = false
				break
			}
			lastPieceSize = piece.Hash.PieceSize

			remotePieces = append(remotePieces, piece)
		}

		if allSizesValid {
			redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
			if err != nil {
				return Error.Wrap(err)
			}

			expectedPieceSize := eestream.CalcPieceSize(pointer.SegmentSize, redundancy)
			if expectedPieceSize != lastPieceSize {
				return Error.New("expected piece size is different from provided (%v != %v)", expectedPieceSize, lastPieceSize)
			}
		} else {
			return Error.New("all pieces needs to have the same size")
		}

		// we repair when the number of healthy files is less than or equal to the repair threshold
		// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
		if int32(len(remotePieces)) <= remote.Redundancy.RepairThreshold && int32(len(remotePieces)) < remote.Redundancy.SuccessThreshold {
			return Error.New("Number of valid pieces (%d) is less than or equal to the repair threshold (%d)",
				len(remotePieces),
				remote.Redundancy.RepairThreshold,
			)
		}

		remote.RemotePieces = remotePieces
	}
	return nil
}

// CreatePath will create a Segment path
func CreatePath(ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket, path []byte) (_ storj.Path, err error) {
	defer mon.Task()(&ctx)(&err)
	if segmentIndex < -1 {
		return "", errors.New("invalid segment index")
	}
	segment := "l"
	if segmentIndex > -1 {
		segment = "s" + strconv.FormatInt(segmentIndex, 10)
	}

	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, segment)
	if len(bucket) != 0 {
		entries = append(entries, string(bucket))
	}
	if len(path) != 0 {
		entries = append(entries, string(path))
	}
	return storj.JoinPaths(entries...), nil
}

// SetAttributionOld tries to add attribution to the bucket.
func (endpoint *Endpoint) SetAttributionOld(ctx context.Context, req *pb.SetAttributionRequestOld) (_ *pb.SetAttributionResponseOld, err error) {
	defer mon.Task()(&ctx)(&err)

	// try to add an attribution that doesn't exist
	partnerID, err := bytesToUUID(req.GetPartnerId())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.BucketName,
		EncryptedPath: []byte(""),
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// check if attribution is set for given bucket
	_, err = endpoint.partnerinfo.Get(ctx, keyInfo.ProjectID, req.GetBucketName())
	if err == nil {
		endpoint.log.Sugar().Info("Bucket:", string(req.BucketName), " PartnerID:", partnerID.String(), "already attributed")
		return &pb.SetAttributionResponseOld{}, nil
	}

	if !attribution.ErrBucketNotAttributed.Has(err) {
		// try only to set the attribution, when it's missing
		return nil, Error.Wrap(err)
	}

	prefix, err := CreatePath(ctx, keyInfo.ProjectID, -1, req.BucketName, []byte(""))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	items, _, err := endpoint.metainfo.List(ctx, prefix, "", "", true, 1, 0)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if len(items) > 0 {
		return nil, Error.New("Bucket(%q) , PartnerID(%s) cannot be attributed", req.BucketName, req.PartnerId)
	}

	_, err = endpoint.partnerinfo.Insert(ctx, &attribution.Info{
		ProjectID:  keyInfo.ProjectID,
		BucketName: req.GetBucketName(),
		PartnerID:  partnerID,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.SetAttributionResponseOld{}, nil
}

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}

// ProjectInfo returns allowed ProjectInfo for the provided API key
func (endpoint *Endpoint) ProjectInfo(ctx context.Context, req *pb.ProjectInfoRequest) (_ *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:   macaroon.ActionProjectInfo,
		Time: time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	salt := sha256.Sum256(keyInfo.ProjectID[:])

	return &pb.ProjectInfoResponse{
		ProjectSalt: salt[:],
	}, nil
}

// GetBucket returns a bucket
func (endpoint *Endpoint) GetBucket(ctx context.Context, req *pb.BucketGetRequest) (resp *pb.BucketGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	bucket, err := endpoint.metainfo.GetBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.BucketGetResponse{
		Bucket: convertBucketToProto(ctx, bucket),
	}, nil
}

// CreateBucket creates a new bucket
func (endpoint *Endpoint) CreateBucket(ctx context.Context, req *pb.BucketCreateRequest) (resp *pb.BucketCreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:     macaroon.ActionWrite,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.validateRedundancy(ctx, req.GetDefaultRedundancyScheme())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	bucket, err := convertProtoToBucket(req, keyInfo.ProjectID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	bucket, err = endpoint.metainfo.CreateBucket(ctx, bucket)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.BucketCreateResponse{
		Bucket: convertBucketToProto(ctx, bucket),
	}, nil
}

// DeleteBucket deletes a bucket
func (endpoint *Endpoint) DeleteBucket(ctx context.Context, req *pb.BucketDeleteRequest) (resp *pb.BucketDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:     macaroon.ActionDelete,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.metainfo.DeleteBucket(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.BucketDeleteResponse{}, nil
}

// ListBuckets returns buckets in a project where the bucket name matches the request cursor
func (endpoint *Endpoint) ListBuckets(ctx context.Context, req *pb.BucketListRequest) (resp *pb.BucketListResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	action := macaroon.Action{
		Op:   macaroon.ActionRead,
		Time: time.Now(),
	}
	keyInfo, err := endpoint.validateAuth(ctx, action)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	allowedBuckets, err := getAllowedBuckets(ctx, action)
	if err != nil {
		return nil, err
	}

	listOpts := storj.BucketListOptions{
		Cursor: string(req.Cursor),
		Limit:  int(req.Limit),
		// We are only supporting the forward direction for listing buckets
		Direction: storj.Forward,
	}
	bucketList, err := endpoint.metainfo.ListBuckets(ctx, keyInfo.ProjectID, listOpts, allowedBuckets)
	if err != nil {
		return nil, err
	}

	bucketItems := make([]*pb.BucketListItem, len(bucketList.Items))
	for i, item := range bucketList.Items {
		bucketItems[i] = &pb.BucketListItem{
			Name:      []byte(item.Name),
			CreatedAt: item.Created,
		}
	}

	return &pb.BucketListResponse{
		Items: bucketItems,
		More:  bucketList.More,
	}, nil
}

func getAllowedBuckets(ctx context.Context, action macaroon.Action) (allowedBuckets map[string]struct{}, err error) {
	keyData, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential GetAPIKey: %v", err)
	}
	key, err := macaroon.ParseAPIKey(string(keyData))
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential ParseAPIKey: %v", err)
	}
	allowedBuckets, err = key.GetAllowedBuckets(ctx, action)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "GetAllowedBuckets: %v", err)
	}
	return allowedBuckets, err
}

// SetBucketAttribution sets the bucket attribution.
func (endpoint *Endpoint) SetBucketAttribution(context.Context, *pb.BucketSetAttributionRequest) (resp *pb.BucketSetAttributionResponse, err error) {
	return resp, status.Error(codes.Unimplemented, "not implemented")
}

func convertProtoToBucket(req *pb.BucketCreateRequest, projectID uuid.UUID) (storj.Bucket, error) {
	bucketID, err := uuid.New()
	if err != nil {
		return storj.Bucket{}, err
	}

	defaultRS := req.GetDefaultRedundancyScheme()
	defaultEP := req.GetDefaultEncryptionParameters()
	return storj.Bucket{
		ID:                  *bucketID,
		Name:                string(req.GetName()),
		ProjectID:           projectID,
		Attribution:         string(req.GetAttributionId()),
		PathCipher:          storj.CipherSuite(req.GetPathCipher()),
		DefaultSegmentsSize: req.GetDefaultSegmentSize(),
		DefaultRedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(defaultRS.GetType()),
			ShareSize:      defaultRS.GetErasureShareSize(),
			RequiredShares: int16(defaultRS.GetMinReq()),
			RepairShares:   int16(defaultRS.GetRepairThreshold()),
			OptimalShares:  int16(defaultRS.GetSuccessThreshold()),
			TotalShares:    int16(defaultRS.GetTotal()),
		},
		DefaultEncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(defaultEP.CipherSuite),
			BlockSize:   int32(defaultEP.BlockSize),
		},
	}, nil
}

func convertBucketToProto(ctx context.Context, bucket storj.Bucket) (pbBucket *pb.Bucket) {
	rs := bucket.DefaultRedundancyScheme
	return &pb.Bucket{
		Name:               []byte(bucket.Name),
		PathCipher:         pb.CipherSuite(int(bucket.PathCipher)),
		AttributionId:      []byte(bucket.Attribution),
		CreatedAt:          bucket.Created,
		DefaultSegmentSize: bucket.DefaultSegmentsSize,
		DefaultRedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_RS,
			MinReq:           int32(rs.RequiredShares),
			Total:            int32(rs.TotalShares),
			RepairThreshold:  int32(rs.RepairShares),
			SuccessThreshold: int32(rs.OptimalShares),
			ErasureShareSize: rs.ShareSize,
		},
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(int(bucket.DefaultEncryptionParameters.CipherSuite)),
			BlockSize:   int64(bucket.DefaultEncryptionParameters.BlockSize),
		},
	}
}
