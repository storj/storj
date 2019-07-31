// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/storage/meta"
)

const (
	pieceHashExpiration = 2 * time.Hour
	satIDExpiration     = 24 * time.Hour
	lastSegment         = -1
	listLimit           = 1000
)

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
	satellite      signing.Signer
}

// NewEndpoint creates new metainfo endpoint instance
func NewEndpoint(log *zap.Logger, metainfo *Service, orders *orders.Service, cache *overlay.Cache, partnerinfo attribution.DB,
	containment Containment, apiKeys APIKeys, projectUsage *accounting.ProjectUsage, rsConfig RSConfig, satellite signing.Signer) *Endpoint {
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
		satellite:      satellite,
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

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, err
	}

	return &pb.SegmentInfoResponseOld{Pointer: pointer}, nil
}

// CreateSegmentOld will generate requested number of OrderLimit with coresponding node addresses for them
func (endpoint *Endpoint) CreateSegmentOld(ctx context.Context, req *pb.SegmentWriteRequestOld) (resp *pb.SegmentWriteResponseOld, err error) {
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

	if !req.Expiration.IsZero() && !req.Expiration.After(time.Now()) {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid expiration time")
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

	bucketID := createBucketID(keyInfo.ProjectID, req.Bucket)
	rootPieceID, addressedLimits, piecePrivateKey, err := endpoint.orders.CreatePutOrderLimits(ctx, bucketID, nodes, req.Expiration, maxPieceSize)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if len(addressedLimits) > 0 {
		endpoint.createRequests.Put(addressedLimits[0].Limit.SerialNumber, &createRequest{
			Expiration: req.Expiration,
			Redundancy: req.Redundancy,
		})
	}

	return &pb.SegmentWriteResponseOld{AddressedLimits: addressedLimits, RootPieceId: rootPieceID, PrivateKey: piecePrivateKey}, nil
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

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, req.Segment, req.Bucket, req.Path)
	if err != nil {
		return nil, err
	}

	if pointer.Type == pb.Pointer_INLINE {
		// TODO or maybe use pointer.SegmentSize ??
		err := endpoint.orders.UpdateGetInlineOrder(ctx, keyInfo.ProjectID, req.Bucket, int64(len(pointer.InlineSegment)))
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.SegmentDownloadResponseOld{Pointer: pointer}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.SegmentDownloadResponseOld{Pointer: pointer, AddressedLimits: limits, PrivateKey: privateKey}, nil
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
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			_, err := endpoint.containment.Delete(ctx, piece.NodeId)
			if err != nil {
				return nil, status.Errorf(codes.Internal, err.Error())
			}
		}

		bucketID := createBucketID(keyInfo.ProjectID, req.Bucket)
		limits, privateKey, err := endpoint.orders.CreateDeleteOrderLimits(ctx, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.SegmentDeleteResponseOld{AddressedLimits: limits, PrivateKey: privateKey}, nil
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

			// TODO enable piece hash signature verification

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
		if storj.ErrBucketNotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	convBucket, err := convertBucketToProto(ctx, bucket)
	if err != nil {
		return resp, err
	}

	return &pb.BucketGetResponse{
		Bucket: convBucket,
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

	// TODO set default Redundancy if not set

	err = endpoint.validateRedundancy(ctx, req.GetDefaultRedundancyScheme())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// checks if bucket exists before updates it or makes a new entry
	bucket, err := endpoint.metainfo.GetBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err == nil {
		var partnerID uuid.UUID
		err = partnerID.UnmarshalJSON(req.GetPartnerId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		}

		// partnerID not set
		if partnerID.IsZero() {
			return resp, status.Errorf(codes.AlreadyExists, "Bucket already exists")
		}

		//update the bucket
		bucket.PartnerID = partnerID
		bucket, err = endpoint.metainfo.UpdateBucket(ctx, bucket)

		pbBucket, err := convertBucketToProto(ctx, bucket)
		if err != nil {
			return resp, status.Errorf(codes.Internal, err.Error())
		}

		return &pb.BucketCreateResponse{
			Bucket: pbBucket,
		}, nil
	}

	// create the bucket
	if storj.ErrBucketNotFound.Has(err) {
		bucket, err := convertProtoToBucket(req, keyInfo.ProjectID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		}

		bucket, err = endpoint.metainfo.CreateBucket(ctx, bucket)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		convBucket, err := convertBucketToProto(ctx, bucket)
		if err != nil {
			return resp, err
		}

		return &pb.BucketCreateResponse{
			Bucket: convBucket,
		}, nil
	}
	return nil, Error.Wrap(err)
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
		Cursor:    string(req.Cursor),
		Limit:     int(req.Limit),
		Direction: storj.ListDirection(req.Direction),
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

func getAllowedBuckets(ctx context.Context, action macaroon.Action) (_ macaroon.AllowedBuckets, err error) {
	keyData, ok := auth.GetAPIKey(ctx)
	if !ok {
		return macaroon.AllowedBuckets{}, status.Errorf(codes.Unauthenticated, "Invalid API credential GetAPIKey: %v", err)
	}
	key, err := macaroon.ParseAPIKey(string(keyData))
	if err != nil {
		return macaroon.AllowedBuckets{}, status.Errorf(codes.Unauthenticated, "Invalid API credential ParseAPIKey: %v", err)
	}
	allowedBuckets, err := key.GetAllowedBuckets(ctx, action)
	if err != nil {
		return macaroon.AllowedBuckets{}, status.Errorf(codes.Internal, "GetAllowedBuckets: %v", err)
	}
	return allowedBuckets, err
}

// SetBucketAttribution sets the bucket attribution.
func (endpoint *Endpoint) SetBucketAttribution(context.Context, *pb.BucketSetAttributionRequest) (resp *pb.BucketSetAttributionResponse, err error) {
	return resp, status.Error(codes.Unimplemented, "not implemented")
}

func convertProtoToBucket(req *pb.BucketCreateRequest, projectID uuid.UUID) (bucket storj.Bucket, err error) {
	bucketID, err := uuid.New()
	if err != nil {
		return storj.Bucket{}, err
	}

	defaultRS := req.GetDefaultRedundancyScheme()
	defaultEP := req.GetDefaultEncryptionParameters()

	var partnerID uuid.UUID
	err = partnerID.UnmarshalJSON(req.GetPartnerId())

	// bucket's partnerID should never be set
	// it is always read back from buckets DB
	if err != nil && !partnerID.IsZero() {
		return bucket, errs.New("Invalid uuid")
	}

	return storj.Bucket{
		ID:                  *bucketID,
		Name:                string(req.GetName()),
		ProjectID:           projectID,
		PartnerID:           partnerID,
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

func convertBucketToProto(ctx context.Context, bucket storj.Bucket) (pbBucket *pb.Bucket, err error) {
	rs := bucket.DefaultRedundancyScheme
	partnerID, err := bucket.PartnerID.MarshalJSON()
	if err != nil {
		return pbBucket, status.Errorf(codes.Internal, "UUID marshal error")
	}
	return &pb.Bucket{
		Name:               []byte(bucket.Name),
		PathCipher:         pb.CipherSuite(int(bucket.PathCipher)),
		PartnerId:          partnerID,
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
	}, nil
}

// BeginObject begins object
func (endpoint *Endpoint) BeginObject(ctx context.Context, req *pb.ObjectBeginRequest) (resp *pb.ObjectBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	bucket, err := endpoint.metainfo.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// take bucket RS values if not set in request
	pbRS := req.RedundancyScheme
	if pbRS.Type == 0 {
		pbRS.Type = pb.RedundancyScheme_SchemeType(bucket.DefaultRedundancyScheme.Algorithm)
	}
	if pbRS.ErasureShareSize == 0 {
		pbRS.ErasureShareSize = bucket.DefaultRedundancyScheme.ShareSize
	}
	if pbRS.MinReq == 0 {
		pbRS.MinReq = int32(bucket.DefaultRedundancyScheme.RequiredShares)
	}
	if pbRS.RepairThreshold == 0 {
		pbRS.RepairThreshold = int32(bucket.DefaultRedundancyScheme.RepairShares)
	}
	if pbRS.SuccessThreshold == 0 {
		pbRS.SuccessThreshold = int32(bucket.DefaultRedundancyScheme.OptimalShares)
	}
	if pbRS.Total == 0 {
		pbRS.Total = int32(bucket.DefaultRedundancyScheme.TotalShares)
	}

	pbEP := req.EncryptionParameters
	if pbEP.CipherSuite == 0 {
		pbEP.CipherSuite = pb.CipherSuite(bucket.DefaultEncryptionParameters.CipherSuite)
	}
	if pbEP.BlockSize == 0 {
		pbEP.BlockSize = int64(bucket.DefaultEncryptionParameters.BlockSize)
	}

	streamID, err := endpoint.packStreamID(ctx, &pb.SatStreamID{
		Bucket:         req.Bucket,
		EncryptedPath:  req.EncryptedPath,
		Version:        req.Version,
		Redundancy:     pbRS,
		CreationDate:   time.Now(),
		ExpirationDate: req.ExpiresAt,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.ObjectBeginResponse{
		Bucket:               req.Bucket,
		EncryptedPath:        req.EncryptedPath,
		Version:              req.Version,
		StreamId:             streamID,
		RedundancyScheme:     pbRS,
		EncryptionParameters: pbEP,
	}, nil
}

// CommitObject commits object when all segments are also committed
func (endpoint *Endpoint) CommitObject(ctx context.Context, req *pb.ObjectCommitRequest) (resp *pb.ObjectCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID := &pb.SatStreamID{}
	err = proto.Unmarshal(req.StreamId, streamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = signing.VerifyStreamID(ctx, endpoint.satellite, streamID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	if streamID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, status.Errorf(codes.InvalidArgument, "stream ID expired")
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	segmentIndex := int64(0)
	var lastSegmentPointer *pb.Pointer
	var lastSegmentPath string
	for {
		path, err := CreatePath(ctx, keyInfo.ProjectID, segmentIndex, streamID.Bucket, streamID.EncryptedPath)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unable to create segment path: %v", err.Error())
		}

		pointer, err := endpoint.metainfo.Get(ctx, path)
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				break
			}
			return nil, status.Errorf(codes.Internal, "unable to create get segment: %v", err.Error())
		}

		lastSegmentPointer = pointer
		lastSegmentPath = path
		segmentIndex++
	}
	if lastSegmentPointer == nil {
		return nil, status.Errorf(codes.NotFound, "unable to find object: %s/%s", streamID.Bucket, streamID.EncryptedPath)
	}

	lastSegmentPointer.Metadata = req.EncryptedMetadata

	err = endpoint.metainfo.Delete(ctx, lastSegmentPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	lastSegmentPath, err = CreatePath(ctx, keyInfo.ProjectID, -1, streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.metainfo.Put(ctx, lastSegmentPath, lastSegmentPointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.ObjectCommitResponse{}, nil
}

// GetObject gets single object
func (endpoint *Endpoint) GetObject(ctx context.Context, req *pb.ObjectGetRequest) (resp *pb.ObjectGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, -1, req.Bucket, req.EncryptedPath)
	if err != nil {
		return nil, err
	}

	streamMeta := &pb.StreamMeta{}
	err = proto.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	streamID, err := endpoint.packStreamID(ctx, &pb.SatStreamID{
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Version:       req.Version,
		CreationDate:  time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	object := &pb.Object{
		Bucket:            req.Bucket,
		EncryptedPath:     req.EncryptedPath,
		Version:           -1,
		StreamId:          streamID,
		ExpiresAt:         pointer.ExpirationDate,
		CreatedAt:         pointer.CreationDate,
		EncryptedMetadata: pointer.Metadata,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(streamMeta.EncryptionType),
			BlockSize:   int64(streamMeta.EncryptionBlockSize),
		},
	}

	if pointer.Remote != nil {
		object.RedundancyScheme = pointer.Remote.Redundancy
	}

	return &pb.ObjectGetResponse{
		Object: object,
	}, nil
}

// ListObjects list objects according to specific parameters
func (endpoint *Endpoint) ListObjects(ctx context.Context, req *pb.ObjectListRequest) (resp *pb.ObjectListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: []byte{},
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	prefix, err := CreatePath(ctx, keyInfo.ProjectID, -1, req.Bucket, req.EncryptedPrefix)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	metaflags := meta.All
	// TODO use flags
	// TODO find out how EncryptedCursor -> startAfter/endAfter
	segments, more, err := endpoint.metainfo.List(ctx, prefix, string(req.EncryptedCursor), "", req.Recursive, req.Limit, metaflags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	items := make([]*pb.ObjectListItem, len(segments))
	for i, segment := range segments {
		items[i] = &pb.ObjectListItem{
			EncryptedPath: []byte(segment.Path),
		}
		if segment.Pointer != nil {
			items[i].EncryptedMetadata = segment.Pointer.Metadata
			items[i].CreatedAt = segment.Pointer.CreationDate
			items[i].ExpiresAt = segment.Pointer.ExpirationDate
		}
	}

	return &pb.ObjectListResponse{
		Items: items,
		More:  more,
	}, nil
}

// BeginDeleteObject begins object deletion process
func (endpoint *Endpoint) BeginDeleteObject(ctx context.Context, req *pb.ObjectBeginDeleteRequest) (resp *pb.ObjectBeginDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	satStreamID := &pb.SatStreamID{
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Version:       req.Version,
		CreationDate:  time.Now(),
	}

	satStreamID, err = signing.SignStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	encodedStreamID, err := proto.Marshal(satStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	streamID, err := storj.StreamIDFromBytes(encodedStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	_, _, err = endpoint.getPointer(ctx, keyInfo.ProjectID, -1, satStreamID.Bucket, satStreamID.EncryptedPath)
	if err != nil {
		return nil, err
	}

	return &pb.ObjectBeginDeleteResponse{
		StreamId: streamID,
	}, nil
}

// FinishDeleteObject finishes object deletion
func (endpoint *Endpoint) FinishDeleteObject(ctx context.Context, req *pb.ObjectFinishDeleteRequest) (resp *pb.ObjectFinishDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID := &pb.SatStreamID{}
	err = proto.Unmarshal(req.StreamId, streamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = signing.VerifyStreamID(ctx, endpoint.satellite, streamID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	if streamID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, status.Errorf(codes.InvalidArgument, "stream ID expired")
	}

	_, err = endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// we don't need to do anything for shim implementation

	return &pb.ObjectFinishDeleteResponse{}, nil
}

// BeginSegment begins segment uploading
func (endpoint *Endpoint) BeginSegment(ctx context.Context, req *pb.SegmentBeginRequest) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// no need to validate streamID fields because it was validated during BeginObject

	if req.Position.Index < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "segment index must be greater then 0")
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

	redundancy, err := eestream.NewRedundancyStrategyFromProto(streamID.Redundancy)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	maxPieceSize := eestream.CalcPieceSize(req.MaxOrderLimit, redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.TotalCount(),
		FreeBandwidth:  maxPieceSize,
		FreeDisk:       maxPieceSize,
	}
	nodes, err := endpoint.cache.FindStorageNodes(ctx, request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	bucketID := createBucketID(keyInfo.ProjectID, streamID.Bucket)
	rootPieceID, addressedLimits, piecePrivateKey, err := endpoint.orders.CreatePutOrderLimits(ctx, bucketID, nodes, streamID.ExpirationDate, maxPieceSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	segmentID, err := endpoint.packSegmentID(ctx, &pb.SatSegmentID{
		StreamId:            streamID,
		Index:               req.Position.Index,
		OriginalOrderLimits: addressedLimits,
		RootPieceId:         rootPieceID,
		CreationDate:        time.Now(),
	})

	return &pb.SegmentBeginResponse{
		SegmentId:       segmentID,
		AddressedLimits: addressedLimits,
		PrivateKey:      piecePrivateKey,
	}, nil
}

// CommitSegment commits segment after uploading
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	streamID := segmentID.StreamId

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	pieces := make([]*pb.RemotePiece, len(req.UploadResult))
	for i, result := range req.UploadResult {
		pieces[i] = &pb.RemotePiece{
			PieceNum: result.PieceNum,
			NodeId:   result.NodeId,
			Hash:     result.Hash,
		}
	}
	remote := &pb.RemoteSegment{
		Redundancy:   streamID.Redundancy,
		RootPieceId:  segmentID.RootPieceId,
		RemotePieces: pieces,
	}

	pointer := &pb.Pointer{
		Type:        pb.Pointer_REMOTE,
		Remote:      remote,
		SegmentSize: req.SizeEncryptedData,

		CreationDate:   streamID.CreationDate,
		ExpirationDate: streamID.ExpirationDate,
	}

	orderLimits := make([]*pb.OrderLimit, len(segmentID.OriginalOrderLimits))
	for i, orderLimit := range segmentID.OriginalOrderLimits {
		orderLimits[i] = orderLimit.Limit
	}

	err = endpoint.validatePointer(ctx, pointer, orderLimits)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	err = endpoint.filterValidPieces(ctx, pointer, orderLimits)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, int64(segmentID.Index), streamID.Bucket, streamID.EncryptedPath)
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
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		piece.Hash = nil
	}

	inlineUsed, remoteUsed := calculateSpaceUsed(pointer)

	// ToDo: Replace with hash & signature validation
	// Ensure neither uplink or storage nodes are cheating on us
	if pointer.Type == pb.Pointer_REMOTE {
		//We cannot have more redundancy than total/min
		if float64(remoteUsed) > (float64(pointer.SegmentSize)/float64(pointer.Remote.Redundancy.MinReq))*float64(pointer.Remote.Redundancy.Total) {
			endpoint.log.Sugar().Debugf("data size mismatch, got segment: %d, pieces: %d, RS Min, Total: %d,%d", pointer.SegmentSize, remoteUsed, pointer.Remote.Redundancy.MinReq, pointer.Remote.Redundancy.Total)
			return nil, status.Errorf(codes.InvalidArgument, "mismatched segment size and piece usage")
		}
	}

	if err := endpoint.projectUsage.AddProjectStorageUsage(ctx, keyInfo.ProjectID, inlineUsed, remoteUsed); err != nil {
		endpoint.log.Sugar().Errorf("Could not track new storage usage by project %v: %v", keyInfo.ProjectID, err)
		// but continue. it's most likely our own fault that we couldn't track it, and the only thing
		// that will be affected is our per-project bandwidth and storage limits.
	}

	err = endpoint.metainfo.Put(ctx, path, pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentCommitResponse{}, nil
}

// MakeInlineSegment makes inline segment on satellite
func (endpoint *Endpoint) MakeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest) (resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	if req.Position.Index < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "segment index must be greater then 0")
	}

	path, err := CreatePath(ctx, keyInfo.ProjectID, int64(req.Position.Index), streamID.Bucket, streamID.EncryptedPath)
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

	inlineUsed := int64(len(req.EncryptedInlineData))

	if err := endpoint.projectUsage.AddProjectStorageUsage(ctx, keyInfo.ProjectID, inlineUsed, 0); err != nil {
		endpoint.log.Sugar().Errorf("Could not track new storage usage by project %v: %v", keyInfo.ProjectID, err)
		// but continue. it's most likely our own fault that we couldn't track it, and the only thing
		// that will be affected is our per-project bandwidth and storage limits.
	}

	pointer := &pb.Pointer{
		Type:           pb.Pointer_INLINE,
		SegmentSize:    inlineUsed,
		CreationDate:   streamID.CreationDate,
		ExpirationDate: streamID.ExpirationDate,
		InlineSegment:  req.EncryptedInlineData,
	}

	err = endpoint.metainfo.Put(ctx, path, pointer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	err = endpoint.orders.UpdatePutInlineOrder(ctx, keyInfo.ProjectID, streamID.Bucket, inlineUsed)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentMakeInlineResponse{}, nil
}

// BeginDeleteSegment begins segment deletion process
func (endpoint *Endpoint) BeginDeleteSegment(ctx context.Context, req *pb.SegmentBeginDeleteRequest) (resp *pb.SegmentBeginDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, int64(req.Position.Index), streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		return nil, err
	}

	var limits []*pb.AddressedOrderLimit
	var privateKey storj.PiecePrivateKey
	if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		bucketID := createBucketID(keyInfo.ProjectID, streamID.Bucket)
		limits, privateKey, err = endpoint.orders.CreateDeleteOrderLimits(ctx, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
	}

	segmentID, err := endpoint.packSegmentID(ctx, &pb.SatSegmentID{
		StreamId:            streamID,
		OriginalOrderLimits: limits,
		Index:               req.Position.Index,
		CreationDate:        time.Now(),
	})

	return &pb.SegmentBeginDeleteResponse{
		SegmentId:       segmentID,
		AddressedLimits: limits,
		PrivateKey:      privateKey,
	}, nil
}

// FinishDeleteSegment finishes segment deletion process
func (endpoint *Endpoint) FinishDeleteSegment(ctx context.Context, req *pb.SegmentFinishDeleteRequest) (resp *pb.SegmentFinishDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	streamID := segmentID.StreamId

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	pointer, path, err := endpoint.getPointer(ctx, keyInfo.ProjectID, int64(segmentID.Index), streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		return nil, err
	}

	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		_, err := endpoint.containment.Delete(ctx, piece.NodeId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
	}

	err = endpoint.metainfo.Delete(ctx, path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.SegmentFinishDeleteResponse{}, nil
}

// ListSegments list object segments
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.SegmentListRequest) (resp *pb.SegmentListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	limit := req.Limit
	if limit == 0 || limit > listLimit {
		limit = listLimit
	}

	index := int64(req.CursorPosition.Index)
	more := false
	segmentItems := make([]*pb.SegmentListItem, 0)
	// TODO think about better implementation
	for {
		path, err := CreatePath(ctx, keyInfo.ProjectID, index, streamID.Bucket, streamID.EncryptedPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		_, err = endpoint.metainfo.Get(ctx, path)
		if err != nil {
			if storage.ErrKeyNotFound.Has(err) {
				if index == lastSegment {
					break
				}
				index = lastSegment
				continue
			}
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		if limit == 0 {
			more = true
			break
		}
		segmentItems = append(segmentItems, &pb.SegmentListItem{
			Position: &pb.SegmentPosition{
				Index: int32(index),
			},
		})

		if index == lastSegment {
			break
		}
		index++
		limit--
	}

	return &pb.SegmentListResponse{
		Items: segmentItems,
		More:  more,
	}, nil
}

// DownloadSegment returns data necessary to download segment
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	bucketID := createBucketID(keyInfo.ProjectID, streamID.Bucket)

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

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, int64(req.CursorPosition.Index), streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		return nil, err
	}

	segmentID, err := endpoint.packSegmentID(ctx, &pb.SatSegmentID{})

	var encryptedKeyNonce storj.Nonce
	var encryptedKey []byte
	if len(pointer.Metadata) != 0 {
		var segmentMeta *pb.SegmentMeta
		if req.CursorPosition.Index == lastSegment {
			streamMeta := &pb.StreamMeta{}
			err = proto.Unmarshal(pointer.Metadata, streamMeta)
			if err != nil {
				return nil, status.Errorf(codes.Internal, err.Error())
			}
			segmentMeta = streamMeta.LastSegmentMeta
		} else {
			err = proto.Unmarshal(pointer.Metadata, segmentMeta)
			if err != nil {
				return nil, status.Errorf(codes.Internal, err.Error())
			}
		}
		if segmentMeta != nil {
			encryptedKeyNonce, err = storj.NonceFromBytes(segmentMeta.KeyNonce)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "unable to get encryption key nonce from metadata: %v", err.Error())
			}

			encryptedKey = segmentMeta.EncryptedKey
		}
	}

	if pointer.Type == pb.Pointer_INLINE {
		err := endpoint.orders.UpdateGetInlineOrder(ctx, keyInfo.ProjectID, streamID.Bucket, int64(len(pointer.InlineSegment)))
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return &pb.SegmentDownloadResponse{
			SegmentId:           segmentID,
			SegmentSize:         pointer.SegmentSize,
			EncryptedInlineData: pointer.InlineSegment,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      encryptedKey,
		}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, bucketID, pointer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		limits = sortLimits(limits, pointer)

		// workaround to avoid sending nil values on top level
		for i := range limits {
			if limits[i] == nil {
				limits[i] = &pb.AddressedOrderLimit{}
			}
		}

		return &pb.SegmentDownloadResponse{
			SegmentId:       segmentID,
			AddressedLimits: limits,
			PrivateKey:      privateKey,
			SegmentSize:     pointer.SegmentSize,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      encryptedKey,
		}, nil
	}

	return &pb.SegmentDownloadResponse{}, status.Errorf(codes.Internal, "invalid type of pointer")
}

func (endpoint *Endpoint) getPointer(ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket, encryptedPath []byte) (*pb.Pointer, string, error) {
	path, err := CreatePath(ctx, projectID, segmentIndex, bucket, encryptedPath)
	if err != nil {
		return nil, "", status.Errorf(codes.InvalidArgument, err.Error())
	}

	pointer, err := endpoint.metainfo.Get(ctx, path)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, "", status.Errorf(codes.NotFound, err.Error())
		}
		return nil, "", status.Errorf(codes.Internal, err.Error())
	}
	return pointer, path, nil
}

// sortLimits sorts order limits and fill missing ones with nil values
func sortLimits(limits []*pb.AddressedOrderLimit, pointer *pb.Pointer) []*pb.AddressedOrderLimit {
	sorted := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		sorted[piece.GetPieceNum()] = getLimitByStorageNodeID(limits, piece.NodeId)
	}
	return sorted
}

func getLimitByStorageNodeID(limits []*pb.AddressedOrderLimit, storageNodeID storj.NodeID) *pb.AddressedOrderLimit {
	for _, limit := range limits {
		if limit.GetLimit().StorageNodeId == storageNodeID {
			return limit
		}
	}
	return nil
}

func (endpoint *Endpoint) packStreamID(ctx context.Context, satStreamID *pb.SatStreamID) (streamID storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	signedStreamID, err := signing.SignStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	encodedStreamID, err := proto.Marshal(signedStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	streamID, err = storj.StreamIDFromBytes(encodedStreamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return streamID, nil
}

func (endpoint *Endpoint) packSegmentID(ctx context.Context, satSegmentID *pb.SatSegmentID) (segmentID storj.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	signedSegmentID, err := signing.SignSegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	encodedSegmentID, err := proto.Marshal(signedSegmentID)
	if err != nil {
		return nil, err
	}

	segmentID, err = storj.SegmentIDFromBytes(encodedSegmentID)
	if err != nil {
		return nil, err
	}
	return segmentID, nil
}

func (endpoint *Endpoint) unmarshalSatStreamID(ctx context.Context, streamID storj.StreamID) (_ *pb.SatStreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	satStreamID := &pb.SatStreamID{}
	err = proto.Unmarshal(streamID, satStreamID)
	if err != nil {
		return nil, err
	}

	err = signing.VerifyStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, err
	}

	if satStreamID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, errs.New("stream ID expired")
	}

	return satStreamID, nil
}

func (endpoint *Endpoint) unmarshalSatSegmentID(ctx context.Context, segmentID storj.SegmentID) (_ *pb.SatSegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	satSegmentID := &pb.SatSegmentID{}
	err = proto.Unmarshal(segmentID, satSegmentID)
	if err != nil {
		return nil, err
	}

	err = signing.VerifySegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	if satSegmentID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, errs.New("segment ID expired")
	}

	return satSegmentID, nil
}
