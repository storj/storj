// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	lrucache "storj.io/storj/pkg/cache"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/metainfo/objectdeletion"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/satellite/metainfo/pointerverification"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/rewards"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/storage/meta"
)

const (
	satIDExpiration = 48 * time.Hour

	deleteObjectPiecesSuccessThreshold = 0.75
)

var (
	mon = monkit.Package()
	// Error general metainfo error.
	Error = errs.Class("metainfo error")
	// ErrNodeAlreadyExists pointer already has a piece for a node err.
	ErrNodeAlreadyExists = errs.Class("metainfo error: node already exists")
)

// APIKeys is api keys store methods used by endpoint.
//
// architecture: Database
type APIKeys interface {
	GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error)
}

// Endpoint metainfo endpoint.
//
// architecture: Endpoint
type Endpoint struct {
	log                  *zap.Logger
	metainfo             *Service
	deletePieces         *piecedeletion.Service
	deleteObjects        *objectdeletion.Service
	orders               *orders.Service
	overlay              *overlay.Service
	attributions         attribution.DB
	partners             *rewards.PartnersService
	pointerVerification  *pointerverification.Service
	projectUsage         *accounting.Service
	projects             console.Projects
	apiKeys              APIKeys
	satellite            signing.Signer
	limiterCache         *lrucache.ExpiringLRU
	encInlineSegmentSize int64 // max inline segment size + encryption overhead
	revocations          revocation.DB
	defaultRS            *pb.RedundancyScheme
	config               Config
}

// NewEndpoint creates new metainfo endpoint instance.
func NewEndpoint(log *zap.Logger, metainfo *Service, deletePieces *piecedeletion.Service,
	orders *orders.Service, cache *overlay.Service, attributions attribution.DB,
	partners *rewards.PartnersService, peerIdentities overlay.PeerIdentities,
	apiKeys APIKeys, projectUsage *accounting.Service, projects console.Projects,
	satellite signing.Signer, revocations revocation.DB, config Config) (*Endpoint, error) {
	// TODO do something with too many params

	encInlineSegmentSize, err := encryption.CalcEncryptedSize(config.MaxInlineSegmentSize.Int64(), storj.EncryptionParameters{
		CipherSuite: storj.EncAESGCM,
		BlockSize:   128, // intentionally low block size to allow maximum possible encryption overhead
	})
	if err != nil {
		return nil, err
	}
	objectDeletion, err := objectdeletion.NewService(log, metainfo, config.ObjectDeletion)
	if err != nil {
		return nil, err
	}

	defaultRSScheme := &pb.RedundancyScheme{
		Type:             pb.RedundancyScheme_RS,
		MinReq:           int32(config.RS.Min),
		RepairThreshold:  int32(config.RS.Repair),
		SuccessThreshold: int32(config.RS.Success),
		Total:            int32(config.RS.Total),
		ErasureShareSize: config.RS.ErasureShareSize.Int32(),
	}

	return &Endpoint{
		log:                 log,
		metainfo:            metainfo,
		deletePieces:        deletePieces,
		deleteObjects:       objectDeletion,
		orders:              orders,
		overlay:             cache,
		attributions:        attributions,
		partners:            partners,
		pointerVerification: pointerverification.NewService(peerIdentities),
		apiKeys:             apiKeys,
		projectUsage:        projectUsage,
		projects:            projects,
		satellite:           satellite,
		limiterCache: lrucache.New(lrucache.Options{
			Capacity:   config.RateLimiter.CacheCapacity,
			Expiration: config.RateLimiter.CacheExpiration,
		}),
		encInlineSegmentSize: encInlineSegmentSize,
		revocations:          revocations,
		defaultRS:            defaultRSScheme,
		config:               config,
	}, nil
}

// Close closes resources.
func (endpoint *Endpoint) Close() error { return nil }

func calculateSpaceUsed(ptr *pb.Pointer) (segmentSize, totalStored int64) {
	inline := ptr.GetInlineSegment()
	if inline != nil {
		inlineSize := int64(len(inline))
		return inlineSize, inlineSize
	}
	segmentSize = ptr.GetSegmentSize()
	remote := ptr.GetRemote()
	if remote == nil {
		return 0, 0
	}
	minReq := remote.GetRedundancy().GetMinReq()
	pieceSize := segmentSize / int64(minReq)
	pieces := remote.GetRemotePieces()
	return segmentSize, pieceSize * int64(len(pieces))
}

// filterValidPieces filter out the invalid remote pieces held by pointer.
//
// This method expect the pointer to be valid, so it has to be validated before
// calling it.
//
// The method always return a gRPC status error so the caller can directly
// return it to the client.
func (endpoint *Endpoint) filterValidPieces(ctx context.Context, pointer *pb.Pointer, originalLimits []*pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pointer.Type != pb.Pointer_REMOTE {
		return nil
	}

	// verify that the piece sizes matches what we would expect.
	err = endpoint.pointerVerification.VerifySizes(ctx, pointer)
	if err != nil {
		endpoint.log.Debug("piece sizes are invalid", zap.Error(err))
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece sizes are invalid: %v", err)
	}

	validPieces, invalidPieces, err := endpoint.pointerVerification.SelectValidPieces(ctx, pointer, originalLimits)
	if err != nil {
		endpoint.log.Debug("pointer verification failed", zap.Error(err))
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "pointer verification failed: %s", err)
	}

	remote := pointer.Remote

	if int32(len(validPieces)) < remote.Redundancy.SuccessThreshold {
		endpoint.log.Debug("Number of valid pieces is less than the success threshold",
			zap.Int("totalReceivedPieces", len(remote.RemotePieces)),
			zap.Int("validPieces", len(validPieces)),
			zap.Int("invalidPieces", len(invalidPieces)),
			zap.Int32("successThreshold", remote.Redundancy.SuccessThreshold),
		)

		errMsg := fmt.Sprintf("Number of valid pieces (%d) is less than the success threshold (%d). Found %d invalid pieces",
			len(validPieces),
			remote.Redundancy.SuccessThreshold,
			len(remote.RemotePieces),
		)
		if len(invalidPieces) > 0 {
			errMsg = fmt.Sprintf("%s. Invalid Pieces:", errMsg)

			for _, p := range invalidPieces {
				errMsg = fmt.Sprintf("%s\nNodeID: %v, PieceNum: %d, Reason: %s",
					errMsg, p.NodeID, p.PieceNum, p.Reason,
				)
			}
		}

		return rpcstatus.Error(rpcstatus.InvalidArgument, errMsg)
	}

	remote.RemotePieces = validPieces

	return nil
}

// ProjectInfo returns allowed ProjectInfo for the provided API key.
func (endpoint *Endpoint) ProjectInfo(ctx context.Context, req *pb.ProjectInfoRequest) (_ *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:   macaroon.ActionProjectInfo,
		Time: time.Now(),
	})
	if err != nil {
		return nil, err
	}

	salt := sha256.Sum256(keyInfo.ProjectID[:])

	return &pb.ProjectInfoResponse{
		ProjectSalt: salt[:],
	}, nil
}

// GetBucket returns a bucket.
func (endpoint *Endpoint) GetBucket(ctx context.Context, req *pb.BucketGetRequest) (resp *pb.BucketGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, err
	}

	bucket, err := endpoint.metainfo.GetBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// override RS to fit satellite settings
	convBucket, err := convertBucketToProto(bucket, endpoint.defaultRS)
	if err != nil {
		return resp, err
	}

	return &pb.BucketGetResponse{
		Bucket: convBucket,
	}, nil
}

// CreateBucket creates a new bucket.
func (endpoint *Endpoint) CreateBucket(ctx context.Context, req *pb.BucketCreateRequest) (resp *pb.BucketCreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionWrite,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	// checks if bucket exists before updates it or makes a new entry
	_, err = endpoint.metainfo.GetBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err == nil {
		// When the bucket exists, try to set the attribution.
		if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName()); err != nil {
			return nil, err
		}
		return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "bucket already exists")
	}
	if !storj.ErrBucketNotFound.Has(err) {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// check if project has exceeded its allocated bucket limit
	maxBuckets, err := endpoint.projects.GetMaxBuckets(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}
	if maxBuckets == nil {
		defaultMaxBuckets := endpoint.config.ProjectLimits.MaxBuckets
		maxBuckets = &defaultMaxBuckets
	}
	bucketCount, err := endpoint.metainfo.CountBuckets(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}
	if bucketCount >= *maxBuckets {
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, fmt.Sprintf("number of allocated buckets (%d) exceeded", endpoint.config.ProjectLimits.MaxBuckets))
	}

	bucketReq, err := convertProtoToBucket(req, keyInfo.ProjectID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	bucket, err := endpoint.metainfo.CreateBucket(ctx, bucketReq)
	if err != nil {
		endpoint.log.Error("error while creating bucket", zap.String("bucketName", bucketReq.Name), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create bucket")
	}

	// Once we have created the bucket, we can try setting the attribution.
	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName()); err != nil {
		return nil, err
	}

	// override RS to fit satellite settings
	convBucket, err := convertBucketToProto(bucket, endpoint.defaultRS)
	if err != nil {
		endpoint.log.Error("error while converting bucket to proto", zap.String("bucketName", bucket.Name), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create bucket")
	}

	return &pb.BucketCreateResponse{
		Bucket: convBucket,
	}, nil
}

// DeleteBucket deletes a bucket.
func (endpoint *Endpoint) DeleteBucket(ctx context.Context, req *pb.BucketDeleteRequest) (resp *pb.BucketDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionDelete,
		Bucket: req.Name,
		Time:   now,
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   now,
	})
	canRead := err == nil

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionList,
		Bucket: req.Name,
		Time:   now,
	})
	canList := err == nil

	var (
		bucket     storj.Bucket
		convBucket *pb.Bucket
	)
	if canRead || canList {
		// Info about deleted bucket is returned only if either Read, or List permission is granted.
		bucket, err = endpoint.metainfo.GetBucket(ctx, req.Name, keyInfo.ProjectID)
		if err != nil {
			if storj.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
			}
			return nil, err
		}

		convBucket, err = convertBucketToProto(bucket, endpoint.defaultRS)
		if err != nil {
			return nil, err
		}
	}

	err = endpoint.metainfo.DeleteBucket(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		if !canRead && !canList {
			// No error info is returned if neither Read, nor List permission is granted.
			return &pb.BucketDeleteResponse{}, nil
		}
		if ErrBucketNotEmpty.Has(err) {
			// List permission is required to delete all objects in a bucket.
			if !req.GetDeleteAll() || !canList {
				return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
			}

			_, deletedObjCount, err := endpoint.deleteBucketNotEmpty(ctx, keyInfo.ProjectID, req.Name)
			if err != nil {
				return nil, err
			}

			return &pb.BucketDeleteResponse{Bucket: convBucket, DeletedObjectsCount: int64(deletedObjCount)}, nil
		}
		if storj.ErrBucketNotFound.Has(err) {
			return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
		}
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
}

// deleteBucketNotEmpty deletes all objects that're complete or have first segment.
// On success, it returns only the number of complete objects that has been deleted
// since from the user's perspective, objects without last segment are invisible.
func (endpoint *Endpoint) deleteBucketNotEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) ([]byte, int, error) {
	// Delete all objects that has last segment.
	deletedCount, err := endpoint.deleteByPrefix(ctx, projectID, bucketName, metabase.LastSegmentIndex)
	if err != nil {
		return nil, 0, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	// Delete all zombie objects that have first segment.
	_, err = endpoint.deleteByPrefix(ctx, projectID, bucketName, metabase.FirstSegmentIndex)
	if err != nil {
		return nil, deletedCount, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = endpoint.metainfo.DeleteBucket(ctx, bucketName, projectID)
	if err != nil {
		if ErrBucketNotEmpty.Has(err) {
			return nil, deletedCount, rpcstatus.Error(rpcstatus.FailedPrecondition, "cannot delete the bucket because it's being used by another process")
		}
		if storj.ErrBucketNotFound.Has(err) {
			return bucketName, 0, nil
		}
		return nil, deletedCount, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return bucketName, deletedCount, nil
}

// deleteByPrefix deletes all objects that matches with a prefix.
func (endpoint *Endpoint) deleteByPrefix(ctx context.Context, projectID uuid.UUID, bucketName []byte, segmentIdx int64) (deletedCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	location, err := CreatePath(ctx, projectID, segmentIdx, bucketName, []byte{})
	if err != nil {
		return deletedCount, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	prefix := location.Encode()
	for {
		segments, more, err := endpoint.metainfo.List(ctx, prefix, "", true, 0, meta.None)
		if err != nil {
			return deletedCount, err
		}

		deleteReqs := make([]*metabase.ObjectLocation, len(segments))
		for i, segment := range segments {
			deleteReqs[i] = &metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: string(bucketName),
				ObjectKey:  metabase.ObjectKey(segment.Path),
			}
		}
		rep, err := endpoint.deleteObjectsPieces(ctx, deleteReqs...)
		if err != nil {
			return deletedCount, err
		}

		deletedCount += len(rep.Deleted)

		if !more {
			break
		}
	}
	return deletedCount, nil
}

// ListBuckets returns buckets in a project where the bucket name matches the request cursor.
func (endpoint *Endpoint) ListBuckets(ctx context.Context, req *pb.BucketListRequest) (resp *pb.BucketListResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	action := macaroon.Action{
		// TODO: This has to be ActionList, but it seems to be set to
		// ActionRead as a hacky workaround to make bucket listing possible.
		Op:   macaroon.ActionRead,
		Time: time.Now(),
	}
	keyInfo, err := endpoint.validateAuth(ctx, req.Header, action)
	if err != nil {
		return nil, err
	}

	allowedBuckets, err := getAllowedBuckets(ctx, req.Header, action)
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

// CountBuckets returns the number of buckets a project currently has.
// TODO: add this to the uplink client side.
func (endpoint *Endpoint) CountBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error) {
	count, err = endpoint.metainfo.CountBuckets(ctx, projectID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func getAllowedBuckets(ctx context.Context, header *pb.RequestHeader, action macaroon.Action) (_ macaroon.AllowedBuckets, err error) {
	key, err := getAPIKey(ctx, header)
	if err != nil {
		return macaroon.AllowedBuckets{}, rpcstatus.Errorf(rpcstatus.InvalidArgument, "Invalid API credentials: %v", err)
	}
	allowedBuckets, err := key.GetAllowedBuckets(ctx, action)
	if err != nil {
		return macaroon.AllowedBuckets{}, rpcstatus.Errorf(rpcstatus.Internal, "GetAllowedBuckets: %v", err)
	}
	return allowedBuckets, err
}

func convertProtoToBucket(req *pb.BucketCreateRequest, projectID uuid.UUID) (bucket storj.Bucket, err error) {
	bucketID, err := uuid.New()
	if err != nil {
		return storj.Bucket{}, err
	}

	defaultRS := req.GetDefaultRedundancyScheme()
	defaultEP := req.GetDefaultEncryptionParameters()

	// TODO: resolve partner id
	var partnerID uuid.UUID
	err = partnerID.UnmarshalJSON(req.GetPartnerId())

	// bucket's partnerID should never be set
	// it is always read back from buckets DB
	if err != nil && !partnerID.IsZero() {
		return bucket, errs.New("Invalid uuid")
	}

	return storj.Bucket{
		ID:                  bucketID,
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

func convertBucketToProto(bucket storj.Bucket, rs *pb.RedundancyScheme) (pbBucket *pb.Bucket, err error) {
	if bucket == (storj.Bucket{}) {
		return nil, nil
	}

	partnerID, err := bucket.PartnerID.MarshalJSON()
	if err != nil {
		return pbBucket, rpcstatus.Error(rpcstatus.Internal, "UUID marshal error")
	}

	pbBucket = &pb.Bucket{
		Name:                    []byte(bucket.Name),
		PathCipher:              pb.CipherSuite(bucket.PathCipher),
		PartnerId:               partnerID,
		CreatedAt:               bucket.Created,
		DefaultSegmentSize:      bucket.DefaultSegmentsSize,
		DefaultRedundancyScheme: rs,
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(bucket.DefaultEncryptionParameters.CipherSuite),
			BlockSize:   int64(bucket.DefaultEncryptionParameters.BlockSize),
		},
	}

	// this part is to provide default ciphers (path and encryption) for old uplinks
	// new uplinks are using ciphers from encryption access
	if pbBucket.PathCipher == pb.CipherSuite_ENC_UNSPECIFIED {
		pbBucket.PathCipher = pb.CipherSuite_ENC_AESGCM
	}
	if pbBucket.DefaultEncryptionParameters.CipherSuite == pb.CipherSuite_ENC_UNSPECIFIED {
		pbBucket.DefaultEncryptionParameters.CipherSuite = pb.CipherSuite_ENC_AESGCM
		pbBucket.DefaultEncryptionParameters.BlockSize = int64(rs.ErasureShareSize * rs.MinReq)
	}

	return pbBucket, nil
}

// BeginObject begins object.
func (endpoint *Endpoint) BeginObject(ctx context.Context, req *pb.ObjectBeginRequest) (resp *pb.ObjectBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	if !req.ExpiresAt.IsZero() && !req.ExpiresAt.After(time.Now()) {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "Invalid expiration time")
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	// TODO this needs to be optimized to avoid DB call on each request
	_, err = endpoint.metainfo.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}

		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.Bucket); err != nil {
		return nil, err
	}

	// use only satellite values for Redundancy Scheme
	pbRS := endpoint.defaultRS

	streamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:         req.Bucket,
		EncryptedPath:  req.EncryptedPath,
		Version:        req.Version,
		Redundancy:     pbRS,
		CreationDate:   time.Now(),
		ExpirationDate: req.ExpiresAt,
	})
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	canDelete := err == nil

	if canDelete {
		_, err = endpoint.DeleteObjectPieces(ctx, keyInfo.ProjectID, req.Bucket, req.EncryptedPath)
		if err != nil {
			return nil, err
		}
	} else {
		location, err := CreatePath(ctx, keyInfo.ProjectID, metabase.LastSegmentIndex, req.Bucket, req.EncryptedPath)
		if err != nil {
			endpoint.log.Error("unable to create path", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		// TODO maybe we can have different Get without pointer unmarshaling
		_, _, err = endpoint.metainfo.GetWithBytes(ctx, location.Encode())
		if err == nil {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized API credentials")
		}
	}

	endpoint.log.Info("Object Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "object"))
	mon.Meter("req_put_object").Mark(1)

	return &pb.ObjectBeginResponse{
		Bucket:           req.Bucket,
		EncryptedPath:    req.EncryptedPath,
		Version:          req.Version,
		StreamId:         streamID,
		RedundancyScheme: pbRS,
	}, nil
}

// CommitObject commits an object when all its segments have already been committed.
func (endpoint *Endpoint) CommitObject(ctx context.Context, req *pb.ObjectCommitRequest) (resp *pb.ObjectCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return endpoint.commitObject(ctx, req, nil)
}

func (endpoint *Endpoint) commitObject(ctx context.Context, req *pb.ObjectCommitRequest, pointer *pb.Pointer) (resp *pb.ObjectCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	metadataSize := memory.Size(len(req.EncryptedMetadata))
	if metadataSize > endpoint.config.MaxMetadataSize {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, fmt.Sprintf("Metadata is too large, got %v, maximum allowed is %v", metadataSize, endpoint.config.MaxMetadataSize))
	}
	streamMeta := pb.StreamMeta{}
	err = pb.Unmarshal(req.EncryptedMetadata, &streamMeta)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "invalid metadata structure")
	}

	lastSegmentPointer := pointer
	if pointer == nil {
		lastSegmentIndex := streamMeta.NumberOfSegments - 1
		lastSegmentLocation, err := CreatePath(ctx, keyInfo.ProjectID, lastSegmentIndex, streamID.Bucket, streamID.EncryptedPath)
		if err != nil {
			return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "unable to create segment path: %s", err.Error())
		}

		var lastSegmentPointerBytes []byte
		lastSegmentKey := lastSegmentLocation.Encode()
		lastSegmentPointerBytes, lastSegmentPointer, err = endpoint.metainfo.GetWithBytes(ctx, lastSegmentKey)
		if err != nil {
			endpoint.log.Error("unable to get pointer", zap.ByteString("segmentPath", lastSegmentKey), zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to commit object")
		}
		if lastSegmentPointer == nil {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "unable to find object: %q/%q", streamID.Bucket, streamID.EncryptedPath)
		}

		err = endpoint.metainfo.Delete(ctx, lastSegmentKey, lastSegmentPointerBytes)
		if err != nil {
			endpoint.log.Error("unable to delete pointer", zap.ByteString("segmentPath", lastSegmentKey), zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to commit object")
		}
	}

	if lastSegmentPointer.Remote == nil {
		lastSegmentPointer.Remote = &pb.RemoteSegment{}
	}
	// RS is set always for last segment to emulate RS per object
	lastSegmentPointer.Remote.Redundancy = streamID.Redundancy
	lastSegmentPointer.Metadata = req.EncryptedMetadata

	lastSegmentLocation, err := CreatePath(ctx, keyInfo.ProjectID, int64(metabase.LastSegmentIndex), streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		endpoint.log.Error("unable to create path", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to commit object")
	}

	err = endpoint.metainfo.UnsynchronizedPut(ctx, lastSegmentLocation.Encode(), lastSegmentPointer)
	if err != nil {
		endpoint.log.Error("unable to put pointer", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to commit object")
	}

	return &pb.ObjectCommitResponse{}, nil
}

// GetObject gets single object.
func (endpoint *Endpoint) GetObject(ctx context.Context, req *pb.ObjectGetRequest) (resp *pb.ObjectGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	object, err := endpoint.getObject(ctx, keyInfo.ProjectID, req.Bucket, req.EncryptedPath, req.Version)
	if err != nil {
		return nil, err
	}

	endpoint.log.Info("Object Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "object"))
	mon.Meter("req_get_object").Mark(1)

	return &pb.ObjectGetResponse{
		Object: object,
	}, nil
}

func (endpoint *Endpoint) getObject(ctx context.Context, projectID uuid.UUID, bucket, encryptedPath []byte, version int32) (*pb.Object, error) {
	pointer, _, err := endpoint.getPointer(ctx, projectID, metabase.LastSegmentIndex, bucket, encryptedPath)
	if err != nil {
		return nil, err
	}

	streamMeta := &pb.StreamMeta{}
	err = pb.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	streamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:        bucket,
		EncryptedPath: encryptedPath,
		Version:       version,
		CreationDate:  time.Now(),
	})
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	object := &pb.Object{
		Bucket:            bucket,
		EncryptedPath:     encryptedPath,
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

		// NumberOfSegments == 0 - pointer with encrypted num of segments
		// NumberOfSegments > 1 - pointer with unencrypted num of segments and multiple segments
	} else if streamMeta.NumberOfSegments == 0 || streamMeta.NumberOfSegments > 1 {
		// workaround
		// The new metainfo API redundancy scheme is on object level (not per segment).
		// Because of that, RS is always taken from the last segment.
		// The old implementation saves RS per segment, and in some cases
		// when the remote file's last segment is an inline segment, we end up
		// missing an RS scheme. This loop will search for RS in segments other than the last one.

		index := int64(0)
		for {
			location, err := CreatePath(ctx, projectID, index, bucket, encryptedPath)
			if err != nil {
				endpoint.log.Error("unable to get pointer path", zap.Error(err))
				return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get object")
			}

			pointer, err = endpoint.metainfo.Get(ctx, location.Encode())
			if err != nil {
				if storj.ErrObjectNotFound.Has(err) {
					break
				}

				endpoint.log.Error("unable to get pointer", zap.Error(err))
				return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get object")
			}
			if pointer.Remote != nil {
				object.RedundancyScheme = pointer.Remote.Redundancy
				break
			}
			index++
		}
	}

	return object, nil
}

// ListObjects list objects according to specific parameters.
func (endpoint *Endpoint) ListObjects(ctx context.Context, req *pb.ObjectListRequest) (resp *pb.ObjectListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPrefix,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	// TODO this needs to be optimized to avoid DB call on each request
	_, err = endpoint.metainfo.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}

		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	prefix, err := CreatePath(ctx, keyInfo.ProjectID, metabase.LastSegmentIndex, req.Bucket, req.EncryptedPrefix)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	metaflags := meta.All
	// TODO use flags
	segments, more, err := endpoint.metainfo.List(ctx, prefix.Encode(), string(req.EncryptedCursor), req.Recursive, req.Limit, metaflags)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
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
	endpoint.log.Info("Object List", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "list"), zap.String("type", "object"))
	mon.Meter("req_list_object").Mark(1)

	return &pb.ObjectListResponse{
		Items: items,
		More:  more,
	}, nil
}

// BeginDeleteObject begins object deletion process.
func (endpoint *Endpoint) BeginDeleteObject(ctx context.Context, req *pb.ObjectBeginDeleteRequest) (resp *pb.ObjectBeginDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionDelete,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          now,
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          now,
	})
	canRead := err == nil

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          now,
	})
	canList := err == nil

	report, err := endpoint.DeleteObjectPieces(ctx, keyInfo.ProjectID, req.Bucket, req.EncryptedPath)
	if err != nil {
		if !canRead && !canList {
			// No error info is returned if neither Read, nor List permission is granted
			return &pb.ObjectBeginDeleteResponse{}, nil
		}
		return nil, err
	}

	var object *pb.Object
	if canRead || canList {
		// Info about deleted object is returned only if either Read, or List permission is granted
		deletedObjects, err := report.DeletedObjects()
		if err != nil {
			endpoint.log.Error("failed to construct deleted object information",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.String("Bucket", string(req.Bucket)),
				zap.String("Encrypted Path", string(req.EncryptedPath)),
				zap.Error(err),
			)
		}
		if len(deletedObjects) > 0 {
			object = deletedObjects[0]
		}
	}

	endpoint.log.Info("Object Delete", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "delete"), zap.String("type", "object"))
	mon.Meter("req_delete_object").Mark(1)

	return &pb.ObjectBeginDeleteResponse{
		Object: object,
	}, nil
}

// FinishDeleteObject finishes object deletion.
func (endpoint *Endpoint) FinishDeleteObject(ctx context.Context, req *pb.ObjectFinishDeleteRequest) (resp *pb.ObjectFinishDeleteResponse, err error) {
	// all logic for deleting is now in BeginDeleteObject
	return nil, rpcstatus.Error(rpcstatus.Unimplemented, "not implemented")
}

// GetObjectIPs returns the IP addresses of the nodes holding the pieces for
// the provided object. This is useful for knowing the locations of the pieces.
func (endpoint *Endpoint) GetObjectIPs(ctx context.Context, req *pb.ObjectGetIPsRequest) (resp *pb.ObjectGetIPsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	lastPointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, metabase.LastSegmentIndex, req.Bucket, req.EncryptedPath)
	if err != nil {
		// endpoint.getPointer already returns valid rpcstatus errors
		return nil, err
	}

	var nodeIDs []storj.NodeID
	addPointerToNodeIDs := func(pointer *pb.Pointer) {
		if pointer.Remote != nil {
			for _, piece := range pointer.Remote.RemotePieces {
				nodeIDs = append(nodeIDs, piece.NodeId)
			}
		}
	}

	addPointerToNodeIDs(lastPointer)

	streamMeta := &pb.StreamMeta{}
	err = pb.Unmarshal(lastPointer.Metadata, streamMeta)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	numSegmentsKnown := streamMeta.NumberOfSegments > 0
	numberOfSegmentsToFetch := int(streamMeta.NumberOfSegments) - 1 // remove last segment since we've already fetch manually

	// If we do not know the number of segments from the streamMeta, we want to
	// continue to run this loop until it cannot find another segment (the
	// break condition in the loop).
	//
	// If we do know the number of segments, we want to run the loop as long as
	// the numberOfSegmentsToFetch is > 0 and until we have fetched that many
	// segments.
	for i := metabase.FirstSegmentIndex; !numSegmentsKnown || (numSegmentsKnown && numberOfSegmentsToFetch > 0 && i < numberOfSegmentsToFetch); i++ {
		location, err := CreatePath(ctx, keyInfo.ProjectID, int64(i), req.Bucket, req.EncryptedPath)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}

		pointer, err := endpoint.metainfo.Get(ctx, location.Encode())
		if err != nil {
			if storj.ErrObjectNotFound.Has(err) {
				break
			}
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		addPointerToNodeIDs(pointer)
	}

	nodes, err := endpoint.overlay.GetOnlineNodesForGetDelete(ctx, nodeIDs)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	resp = &pb.ObjectGetIPsResponse{}
	for _, node := range nodes {
		address := node.Address.GetAddress()
		if address != "" {
			resp.Ips = append(resp.Ips, []byte(address))
		}
	}

	return resp, nil
}

// BeginSegment begins segment uploading.
func (endpoint *Endpoint) BeginSegment(ctx context.Context, req *pb.SegmentBeginRequest) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// no need to validate streamID fields because it was validated during BeginObject

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	if err := endpoint.checkExceedsStorageUsage(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(streamID.Redundancy)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	maxPieceSize := eestream.CalcPieceSize(req.MaxOrderLimit, redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.TotalCount(),
	}
	nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	rootPieceID, addressedLimits, piecePrivateKey, err := endpoint.orders.CreatePutOrderLimits(ctx, bucket, nodes, streamID.ExpirationDate, maxPieceSize)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{
		StreamId:            streamID,
		Index:               req.Position.Index,
		OriginalOrderLimits: addressedLimits,
		RootPieceId:         rootPieceID,
		CreationDate:        time.Now(),
	})

	endpoint.log.Info("Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "remote"))
	mon.Meter("req_put_remote").Mark(1)

	return &pb.SegmentBeginResponse{
		SegmentId:       segmentID,
		AddressedLimits: addressedLimits,
		PrivateKey:      piecePrivateKey,
	}, nil
}

// CommitSegment commits segment after uploading.
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, resp, err = endpoint.commitSegment(ctx, req, true)
	return resp, err
}

func (endpoint *Endpoint) commitSegment(ctx context.Context, req *pb.SegmentCommitRequest, savePointer bool) (pointer *pb.Pointer, resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamID := segmentID.StreamId

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}

	if numResults := len(req.UploadResult); numResults < int(streamID.Redundancy.GetSuccessThreshold()) {
		endpoint.log.Debug("the results of uploaded pieces for the segment is below the redundancy optimal threshold",
			zap.Int("upload pieces results", numResults),
			zap.Int32("redundancy optimal threshold", streamID.Redundancy.GetSuccessThreshold()),
			zap.Stringer("Segment ID", req.SegmentId),
		)
		return nil, nil, rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"the number of results of uploaded pieces (%d) is below the optimal threshold (%d)",
			numResults, streamID.Redundancy.GetSuccessThreshold(),
		)
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

	metadata, err := pb.Marshal(&pb.SegmentMeta{
		EncryptedKey: req.EncryptedKey,
		KeyNonce:     req.EncryptedKeyNonce.Bytes(),
	})
	if err != nil {
		endpoint.log.Error("unable to marshal segment metadata", zap.Error(err))
		return nil, nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	pointer = &pb.Pointer{
		Type:        pb.Pointer_REMOTE,
		Remote:      remote,
		SegmentSize: req.SizeEncryptedData,

		CreationDate:   streamID.CreationDate,
		ExpirationDate: streamID.ExpirationDate,
		Metadata:       metadata,

		PieceHashesVerified: true,
	}

	orderLimits := make([]*pb.OrderLimit, len(segmentID.OriginalOrderLimits))
	for i, orderLimit := range segmentID.OriginalOrderLimits {
		orderLimits[i] = orderLimit.Limit
	}

	err = endpoint.validatePointer(ctx, pointer, orderLimits)
	if err != nil {
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	err = endpoint.filterValidPieces(ctx, pointer, orderLimits)
	if err != nil {
		return nil, nil, err
	}

	if err := endpoint.checkExceedsStorageUsage(ctx, keyInfo.ProjectID); err != nil {
		return nil, nil, err
	}

	// clear hashes so we don't store them
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		piece.Hash = nil
	}

	segmentSize, totalStored := calculateSpaceUsed(pointer)

	// ToDo: Replace with hash & signature validation
	// Ensure neither uplink or storage nodes are cheating on us
	if pointer.Type == pb.Pointer_REMOTE {
		// We cannot have more redundancy than total/min
		if float64(totalStored) > (float64(pointer.SegmentSize)/float64(pointer.Remote.Redundancy.MinReq))*float64(pointer.Remote.Redundancy.Total) {
			endpoint.log.Debug("data size mismatch",
				zap.Int64("segment", pointer.SegmentSize),
				zap.Int64("pieces", totalStored),
				zap.Int32("redundancy minimum requested", pointer.Remote.Redundancy.MinReq),
				zap.Int32("redundancy total", pointer.Remote.Redundancy.Total),
			)
			return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, "mismatched segment size and piece usage")
		}
	}

	if err := endpoint.projectUsage.AddProjectStorageUsage(ctx, keyInfo.ProjectID, segmentSize); err != nil {
		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// storage limits.
		endpoint.log.Error("Could not track new project's storage usage",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	if savePointer {
		location, err := CreatePath(ctx, keyInfo.ProjectID, int64(segmentID.Index), streamID.Bucket, streamID.EncryptedPath)
		if err != nil {
			return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}

		err = endpoint.metainfo.UnsynchronizedPut(ctx, location.Encode(), pointer)
		if err != nil {
			return nil, nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	return pointer, &pb.SegmentCommitResponse{
		SuccessfulPieces: int32(len(pointer.Remote.RemotePieces)),
	}, nil
}

// MakeInlineSegment makes inline segment on satellite.
func (endpoint *Endpoint) MakeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest) (resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	_, resp, err = endpoint.makeInlineSegment(ctx, req, true)
	return resp, err
}

// MakeInlineSegment makes inline segment on satellite.
func (endpoint *Endpoint) makeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest, savePointer bool) (pointer *pb.Pointer, resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}

	if req.Position.Index < 0 {
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	inlineUsed := int64(len(req.EncryptedInlineData))
	if inlineUsed > endpoint.encInlineSegmentSize {
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, fmt.Sprintf("inline segment size cannot be larger than %s", endpoint.config.MaxInlineSegmentSize))
	}

	if err := endpoint.checkExceedsStorageUsage(ctx, keyInfo.ProjectID); err != nil {
		return nil, nil, err
	}

	if err := endpoint.projectUsage.AddProjectStorageUsage(ctx, keyInfo.ProjectID, inlineUsed); err != nil {
		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// bandwidth and storage limits.
		endpoint.log.Error("Could not track new project's storage usage",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	metadata, err := pb.Marshal(&pb.SegmentMeta{
		EncryptedKey: req.EncryptedKey,
		KeyNonce:     req.EncryptedKeyNonce.Bytes(),
	})

	pointer = &pb.Pointer{
		Type:           pb.Pointer_INLINE,
		SegmentSize:    inlineUsed,
		CreationDate:   streamID.CreationDate,
		ExpirationDate: streamID.ExpirationDate,
		InlineSegment:  req.EncryptedInlineData,
		Metadata:       metadata,
	}

	if savePointer {
		location, err := CreatePath(ctx, keyInfo.ProjectID, int64(req.Position.Index), streamID.Bucket, streamID.EncryptedPath)
		if err != nil {
			return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}

		err = endpoint.metainfo.UnsynchronizedPut(ctx, location.Encode(), pointer)
		if err != nil {
			return nil, nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	err = endpoint.orders.UpdatePutInlineOrder(ctx, bucket, inlineUsed)
	if err != nil {
		return nil, nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Inline Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "inline"))
	mon.Meter("req_put_inline").Mark(1)

	return pointer, &pb.SegmentMakeInlineResponse{}, nil
}

// BeginDeleteSegment begins segment deletion process.
func (endpoint *Endpoint) BeginDeleteSegment(ctx context.Context, req *pb.SegmentBeginDeleteRequest) (resp *pb.SegmentBeginDeleteResponse, err error) {
	// all logic for deleting is now in BeginDeleteObject
	return nil, rpcstatus.Error(rpcstatus.Unimplemented, "not implemented")
}

// FinishDeleteSegment finishes segment deletion process.
func (endpoint *Endpoint) FinishDeleteSegment(ctx context.Context, req *pb.SegmentFinishDeleteRequest) (resp *pb.SegmentFinishDeleteResponse, err error) {
	// all logic for deleting is now in BeginDeleteObject
	return nil, rpcstatus.Error(rpcstatus.Unimplemented, "not implemented")
}

// ListSegments list object segments.
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.SegmentListRequest) (resp *pb.SegmentListResponse, err error) {
	// nothing is using this method
	return nil, rpcstatus.Error(rpcstatus.Unimplemented, "not implemented")
}

// DownloadSegment returns data necessary to download segment.
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedPath,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}

	if exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfo.ProjectID); err != nil {
		endpoint.log.Error("Retrieving project bandwidth total failed; bandwidth limit won't be enforced", zap.Error(err))
	} else if exceeded {
		endpoint.log.Error("Monthly bandwidth limit exceeded",
			zap.Stringer("Limit", limit),
			zap.Stringer("Project ID", keyInfo.ProjectID),
		)
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}

	pointer, _, err := endpoint.getPointer(ctx, keyInfo.ProjectID, int64(req.CursorPosition.Index), streamID.Bucket, streamID.EncryptedPath)
	if err != nil {
		return nil, err
	}

	// Update the current bandwidth cache value incrementing the SegmentSize.
	if err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfo.ProjectID, pointer.SegmentSize); err != nil {
		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// bandwidth limits.
		endpoint.log.Error("Could not track the new project's bandwidth usage",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{})
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var encryptedKeyNonce storj.Nonce
	var encryptedKey []byte
	if len(pointer.Metadata) != 0 {
		var segmentMeta *pb.SegmentMeta
		if req.CursorPosition.Index == metabase.LastSegmentIndex {
			streamMeta := &pb.StreamMeta{}
			err = pb.Unmarshal(pointer.Metadata, streamMeta)
			if err != nil {
				return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
			segmentMeta = streamMeta.LastSegmentMeta
		} else {
			segmentMeta = &pb.SegmentMeta{}
			err = pb.Unmarshal(pointer.Metadata, segmentMeta)
			if err != nil {
				return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
		}
		if segmentMeta != nil {
			encryptedKeyNonce, err = storj.NonceFromBytes(segmentMeta.KeyNonce)
			if err != nil {
				endpoint.log.Error("unable to get encryption key nonce from metadata", zap.Error(err))
				return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			encryptedKey = segmentMeta.EncryptedKey
		}
	}

	if pointer.Type == pb.Pointer_INLINE {
		err := endpoint.orders.UpdateGetInlineOrder(ctx, bucket, int64(len(pointer.InlineSegment)))
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		endpoint.log.Info("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
		mon.Meter("req_get_inline").Mark(1)

		return &pb.SegmentDownloadResponse{
			SegmentId:           segmentID,
			SegmentSize:         pointer.SegmentSize,
			EncryptedInlineData: pointer.InlineSegment,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      encryptedKey,
		}, nil
	} else if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, bucket, pointer)
		if err != nil {
			if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
				endpoint.log.Error("Unable to create order limits.",
					zap.Stringer("Project ID", keyInfo.ProjectID),
					zap.Stringer("API Key ID", keyInfo.ID),
					zap.Error(err),
				)
			}
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		limits = sortLimits(limits, pointer)

		// workaround to avoid sending nil values on top level
		for i := range limits {
			if limits[i] == nil {
				limits[i] = &pb.AddressedOrderLimit{}
			}
		}

		endpoint.log.Info("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
		mon.Meter("req_get_remote").Mark(1)

		return &pb.SegmentDownloadResponse{
			SegmentId:       segmentID,
			AddressedLimits: limits,
			PrivateKey:      privateKey,
			SegmentSize:     pointer.SegmentSize,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      encryptedKey,
		}, nil
	}

	return &pb.SegmentDownloadResponse{}, rpcstatus.Error(rpcstatus.Internal, "invalid type of pointer")
}

// getPointer returns the pointer and the segment path projectID, bucket and
// encryptedPath. It returns an error with a specific RPC status.
func (endpoint *Endpoint) getPointer(
	ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket, encryptedPath []byte,
) (pointer *pb.Pointer, location metabase.SegmentLocation, err error) {
	defer mon.Task()(&ctx, projectID.String(), segmentIndex, bucket, encryptedPath)(&err)
	location, err = CreatePath(ctx, projectID, segmentIndex, bucket, encryptedPath)
	if err != nil {
		return nil, location, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	pointer, err = endpoint.metainfo.Get(ctx, location.Encode())
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			return nil, location, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}

		endpoint.log.Error("error getting the pointer from metainfo service", zap.Error(err))
		return nil, location, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	return pointer, location, nil
}

// sortLimits sorts order limits and fill missing ones with nil values.
func sortLimits(limits []*pb.AddressedOrderLimit, pointer *pb.Pointer) []*pb.AddressedOrderLimit {
	sorted := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		sorted[piece.GetPieceNum()] = getLimitByStorageNodeID(limits, piece.NodeId)
	}
	return sorted
}

func getLimitByStorageNodeID(limits []*pb.AddressedOrderLimit, storageNodeID storj.NodeID) *pb.AddressedOrderLimit {
	for _, limit := range limits {
		if limit == nil || limit.GetLimit() == nil {
			continue
		}

		if limit.GetLimit().StorageNodeId == storageNodeID {
			return limit
		}
	}
	return nil
}

func (endpoint *Endpoint) packStreamID(ctx context.Context, satStreamID *internalpb.StreamID) (streamID storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	signedStreamID, err := SignStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	encodedStreamID, err := pb.Marshal(signedStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	streamID, err = storj.StreamIDFromBytes(encodedStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	return streamID, nil
}

func (endpoint *Endpoint) packSegmentID(ctx context.Context, satSegmentID *internalpb.SegmentID) (segmentID storj.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	signedSegmentID, err := SignSegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	encodedSegmentID, err := pb.Marshal(signedSegmentID)
	if err != nil {
		return nil, err
	}

	segmentID, err = storj.SegmentIDFromBytes(encodedSegmentID)
	if err != nil {
		return nil, err
	}
	return segmentID, nil
}

func (endpoint *Endpoint) unmarshalSatStreamID(ctx context.Context, streamID storj.StreamID) (_ *internalpb.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	satStreamID := &internalpb.StreamID{}
	err = pb.Unmarshal(streamID, satStreamID)
	if err != nil {
		return nil, err
	}

	err = VerifyStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, err
	}

	if satStreamID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, errs.New("stream ID expired")
	}

	return satStreamID, nil
}

func (endpoint *Endpoint) unmarshalSatSegmentID(ctx context.Context, segmentID storj.SegmentID) (_ *internalpb.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	satSegmentID := &internalpb.SegmentID{}
	err = pb.Unmarshal(segmentID, satSegmentID)
	if err != nil {
		return nil, err
	}
	if satSegmentID.StreamId == nil {
		return nil, errs.New("stream ID missing")
	}

	err = VerifySegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	if satSegmentID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, errs.New("segment ID expired")
	}

	return satSegmentID, nil
}

// DeleteObjectPieces deletes all the pieces of the storage nodes that belongs
// to the specified object.
//
// NOTE: this method is exported for being able to individually test it without
// having import cycles.
func (endpoint *Endpoint) DeleteObjectPieces(
	ctx context.Context, projectID uuid.UUID, bucket, encryptedPath []byte,
) (report objectdeletion.Report, err error) {
	defer mon.Task()(&ctx, projectID.String(), bucket, encryptedPath)(&err)

	req := &metabase.ObjectLocation{
		ProjectID:  projectID,
		BucketName: string(bucket),
		ObjectKey:  metabase.ObjectKey(encryptedPath),
	}

	report, err = endpoint.deleteObjectsPieces(ctx, req)
	if err != nil {
		endpoint.log.Error("failed to delete pointers",
			zap.Stringer("project_id", projectID),
			zap.ByteString("bucket_name", bucket),
			zap.Binary("encrypted_path", encryptedPath),
			zap.Error(err),
		)
		// Only return an error if we failed to delete the pointers. If we failed
		// to delete pieces, let garbage collector take care of it.
		if objectdeletion.Error.Has(err) {
			return report, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	return report, nil
}

func (endpoint *Endpoint) deleteObjectsPieces(ctx context.Context, reqs ...*metabase.ObjectLocation) (report objectdeletion.Report, err error) {
	// We should ignore client cancelling and always try to delete segments.
	ctx = context2.WithoutCancellation(ctx)

	results, err := endpoint.deleteObjects.Delete(ctx, reqs...)
	if err != nil {
		return report, err
	}

	var requests []piecedeletion.Request
	for _, r := range results {
		pointers := r.DeletedPointers()
		report.Deleted = append(report.Deleted, r.Deleted...)
		report.Failed = append(report.Failed, r.Failed...)

		nodesPieces := objectdeletion.GroupPiecesByNodeID(pointers)

		if len(nodesPieces) == 0 {
			continue
		}

		for node, pieces := range nodesPieces {
			requests = append(requests, piecedeletion.Request{
				Node: storj.NodeURL{
					ID: node,
				},
				Pieces: pieces,
			})
		}
	}

	if err := endpoint.deletePieces.Delete(ctx, requests, deleteObjectPiecesSuccessThreshold); err != nil {
		endpoint.log.Error("failed to delete pieces", zap.Error(err))
	}

	return report, nil
}

// RevokeAPIKey handles requests to revoke an api key.
func (endpoint *Endpoint) RevokeAPIKey(ctx context.Context, req *pb.RevokeAPIKeyRequest) (resp *pb.RevokeAPIKeyResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	macToRevoke, err := macaroon.ParseMacaroon(req.GetApiKey())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "API key to revoke is not a macaroon")
	}
	keyInfo, err := endpoint.validateRevoke(ctx, req.Header, macToRevoke)
	if err != nil {
		return nil, err
	}

	err = endpoint.revocations.Revoke(ctx, macToRevoke.Tail(), keyInfo.ID[:])
	if err != nil {
		endpoint.log.Error("Failed to revoke API key", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "Failed to revoke API key")
	}

	return &pb.RevokeAPIKeyResponse{}, nil
}

func (endpoint *Endpoint) checkExceedsStorageUsage(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	exceeded, limit, err := endpoint.projectUsage.ExceedsStorageUsage(ctx, projectID)
	if err != nil {
		endpoint.log.Error(
			"Retrieving project storage totals failed; storage usage limit won't be enforced",
			zap.Error(err),
		)
	} else if exceeded {
		endpoint.log.Error("Monthly storage limit exceeded",
			zap.Stringer("Limit", limit),
			zap.Stringer("Project ID", projectID),
		)
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}

	return nil
}

// CreatePath creates a segment key.
func CreatePath(ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket, path []byte) (_ metabase.SegmentLocation, err error) {
	// TODO rename to CreateLocation
	defer mon.Task()(&ctx)(&err)
	if segmentIndex < metabase.LastSegmentIndex {
		return metabase.SegmentLocation{}, errors.New("invalid segment index")
	}
	return metabase.SegmentLocation{
		ProjectID:  projectID,
		BucketName: string(bucket),
		Index:      segmentIndex,
		ObjectKey:  metabase.ObjectKey(path),
	}, nil
}
