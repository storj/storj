// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/lrucache"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/satellite/metainfo/pointerverification"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/rewards"
	"storj.io/uplink/private/eestream"
)

const (
	satIDExpiration = 48 * time.Hour

	deleteObjectPiecesSuccessThreshold = 0.75
)

var (
	mon = monkit.Package()
	// Error general metainfo error.
	Error = errs.Class("metainfo")
	// ErrNodeAlreadyExists pointer already has a piece for a node err.
	ErrNodeAlreadyExists = errs.Class("metainfo: node already exists")
	// ErrBucketNotEmpty is returned when bucket is required to be empty for an operation.
	ErrBucketNotEmpty = errs.Class("bucket not empty")
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
	pb.DRPCMetainfoUnimplementedServer

	log                  *zap.Logger
	buckets              *buckets.Service
	metabase             *metabase.DB
	deletePieces         *piecedeletion.Service
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
	versionCollector     *versionCollector
}

// NewEndpoint creates new metainfo endpoint instance.
func NewEndpoint(log *zap.Logger, buckets *buckets.Service, metabaseDB *metabase.DB,
	deletePieces *piecedeletion.Service, orders *orders.Service, cache *overlay.Service,
	attributions attribution.DB, partners *rewards.PartnersService, peerIdentities overlay.PeerIdentities,
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
		buckets:             buckets,
		metabase:            metabaseDB,
		deletePieces:        deletePieces,
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
		versionCollector:     newVersionCollector(log),
	}, nil
}

// Close closes resources.
func (endpoint *Endpoint) Close() error { return nil }

func calculateSpaceUsed(segmentSize int64, numberOfPieces int, rs storj.RedundancyScheme) (totalStored int64) {
	pieceSize := segmentSize / int64(rs.RequiredShares)
	return pieceSize * int64(numberOfPieces)
}

// ProjectInfo returns allowed ProjectInfo for the provided API key.
func (endpoint *Endpoint) ProjectInfo(ctx context.Context, req *pb.ProjectInfoRequest) (_ *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, err
	}

	bucket, err := endpoint.buckets.GetMinimalBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// override RS to fit satellite settings
	convBucket, err := convertBucketToProto(bucket, endpoint.defaultRS, endpoint.config.MaxSegmentSize)
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

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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
	exists, err := endpoint.buckets.HasBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	} else if exists {
		// When the bucket exists, try to set the attribution.
		if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName()); err != nil {
			// ensureAttribution returns correct rpc error
			return nil, err
		}
		return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "bucket already exists")
	}

	// check if project has exceeded its allocated bucket limit
	maxBuckets, err := endpoint.projects.GetMaxBuckets(ctx, keyInfo.ProjectID)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if maxBuckets == nil {
		defaultMaxBuckets := endpoint.config.ProjectLimits.MaxBuckets
		maxBuckets = &defaultMaxBuckets
	}
	bucketCount, err := endpoint.buckets.CountBuckets(ctx, keyInfo.ProjectID)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if bucketCount >= *maxBuckets {
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, fmt.Sprintf("number of allocated buckets (%d) exceeded", endpoint.config.ProjectLimits.MaxBuckets))
	}

	bucketReq, err := convertProtoToBucket(req, keyInfo.ProjectID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	bucket, err := endpoint.buckets.CreateBucket(ctx, bucketReq)
	if err != nil {
		endpoint.log.Error("error while creating bucket", zap.String("bucketName", bucketReq.Name), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create bucket")
	}

	// Once we have created the bucket, we can try setting the attribution.
	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName()); err != nil {
		// ensureAttribution returns correct rpc error
		return nil, err
	}

	// override RS to fit satellite settings
	convBucket, err := convertBucketToProto(buckets.Bucket{
		Name:      []byte(bucket.Name),
		CreatedAt: bucket.Created,
	}, endpoint.defaultRS, endpoint.config.MaxSegmentSize)
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

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	now := time.Now()

	var canRead, canList bool

	keyInfo, err := endpoint.validateAuthN(ctx, req.Header,
		verifyPermission{
			action: macaroon.Action{
				Op:     macaroon.ActionDelete,
				Bucket: req.Name,
				Time:   now,
			},
		},
		verifyPermission{
			action: macaroon.Action{
				Op:     macaroon.ActionRead,
				Bucket: req.Name,
				Time:   now,
			},
			actionPermitted: &canRead,
			optional:        true,
		},
		verifyPermission{
			action: macaroon.Action{
				Op:     macaroon.ActionList,
				Bucket: req.Name,
				Time:   now,
			},
			actionPermitted: &canList,
			optional:        true,
		},
	)
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	var (
		bucket     buckets.Bucket
		convBucket *pb.Bucket
	)
	if canRead || canList {
		// Info about deleted bucket is returned only if either Read, or List permission is granted.
		bucket, err = endpoint.buckets.GetMinimalBucket(ctx, req.Name, keyInfo.ProjectID)
		if err != nil {
			if storj.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
			}
			return nil, err
		}

		convBucket, err = convertBucketToProto(bucket, endpoint.defaultRS, endpoint.config.MaxSegmentSize)
		if err != nil {
			return nil, err
		}
	}

	err = endpoint.deleteBucket(ctx, req.Name, keyInfo.ProjectID)
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

			return &pb.BucketDeleteResponse{Bucket: convBucket, DeletedObjectsCount: deletedObjCount}, nil
		}
		if storj.ErrBucketNotFound.Has(err) {
			return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
}

// deleteBucket deletes a bucket from the bucekts db.
func (endpoint *Endpoint) deleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	empty, err := endpoint.isBucketEmpty(ctx, projectID, bucketName)
	if err != nil {
		return err
	}
	if !empty {
		return ErrBucketNotEmpty.New("")
	}

	return endpoint.buckets.DeleteBucket(ctx, bucketName, projectID)
}

// isBucketEmpty returns whether bucket is empty.
func (endpoint *Endpoint) isBucketEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) (bool, error) {
	empty, err := endpoint.metabase.BucketEmpty(ctx, metabase.BucketEmpty{
		ProjectID:  projectID,
		BucketName: string(bucketName),
	})
	return empty, Error.Wrap(err)
}

// deleteBucketNotEmpty deletes all objects from bucket and deletes this bucket.
// On success, it returns only the number of deleted objects.
func (endpoint *Endpoint) deleteBucketNotEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) ([]byte, int64, error) {
	deletedCount, err := endpoint.deleteBucketObjects(ctx, projectID, bucketName)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, 0, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = endpoint.deleteBucket(ctx, bucketName, projectID)
	if err != nil {
		if ErrBucketNotEmpty.Has(err) {
			return nil, deletedCount, rpcstatus.Error(rpcstatus.FailedPrecondition, "cannot delete the bucket because it's being used by another process")
		}
		if storj.ErrBucketNotFound.Has(err) {
			return bucketName, 0, nil
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, deletedCount, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return bucketName, deletedCount, nil
}

// deleteBucketObjects deletes all objects in a bucket.
func (endpoint *Endpoint) deleteBucketObjects(ctx context.Context, projectID uuid.UUID, bucketName []byte) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketLocation := metabase.BucketLocation{ProjectID: projectID, BucketName: string(bucketName)}
	deletedObjects, err := endpoint.metabase.DeleteBucketObjects(ctx, metabase.DeleteBucketObjects{
		Bucket: bucketLocation,
		DeletePieces: func(ctx context.Context, deleted []metabase.DeletedSegmentInfo) error {
			endpoint.deleteSegmentPieces(ctx, deleted)
			return nil
		},
	})

	return deletedObjects, Error.Wrap(err)
}

// ListBuckets returns buckets in a project where the bucket name matches the request cursor.
func (endpoint *Endpoint) ListBuckets(ctx context.Context, req *pb.BucketListRequest) (resp *pb.BucketListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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
	bucketList, err := endpoint.buckets.ListBuckets(ctx, keyInfo.ProjectID, listOpts, allowedBuckets)
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
	count, err = endpoint.buckets.CountBuckets(ctx, projectID)
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

	// TODO: resolve partner id
	var partnerID uuid.UUID
	err = partnerID.UnmarshalJSON(req.GetPartnerId())

	// bucket's partnerID should never be set
	// it is always read back from buckets DB
	if err != nil && !partnerID.IsZero() {
		return bucket, errs.New("Invalid uuid")
	}

	return storj.Bucket{
		ID:        bucketID,
		Name:      string(req.GetName()),
		ProjectID: projectID,
		PartnerID: partnerID,
	}, nil
}

func convertBucketToProto(bucket buckets.Bucket, rs *pb.RedundancyScheme, maxSegmentSize memory.Size) (pbBucket *pb.Bucket, err error) {
	if len(bucket.Name) == 0 {
		return nil, nil
	}

	return &pb.Bucket{
		Name:      bucket.Name,
		CreatedAt: bucket.CreatedAt,

		// default satellite values
		PathCipher:              pb.CipherSuite_ENC_AESGCM,
		DefaultSegmentSize:      maxSegmentSize.Int64(),
		DefaultRedundancyScheme: rs,
		DefaultEncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite_ENC_AESGCM,
			BlockSize:   int64(rs.ErasureShareSize * rs.MinReq),
		},
	}, nil
}

// BeginObject begins object.
func (endpoint *Endpoint) BeginObject(ctx context.Context, req *pb.ObjectBeginRequest) (resp *pb.ObjectBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	now := time.Now()

	var canDelete bool

	keyInfo, err := endpoint.validateAuthN(ctx, req.Header,
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedPath,
				Time:          now,
			},
		},
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedPath,
				Time:          now,
			},
			actionPermitted: &canDelete,
			optional:        true,
		},
	)
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

	objectKeyLength := len(req.EncryptedPath)
	if objectKeyLength > endpoint.config.MaxEncryptedObjectKeyLength {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, fmt.Sprintf("key length is too big, got %v, maximum allowed is %v", objectKeyLength, endpoint.config.MaxEncryptedObjectKeyLength))
	}

	err = endpoint.checkUploadLimits(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	placement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if canDelete {
		_, err = endpoint.DeleteObjectAnyStatus(ctx, metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
		})
		if err != nil && !storj.ErrObjectNotFound.Has(err) {
			return nil, err
		}
	} else {
		_, err = endpoint.metabase.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: string(req.Bucket),
				ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
			},
		})
		if err == nil {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized API credentials")
		}
	}

	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.Bucket); err != nil {
		return nil, err
	}

	streamID, err := uuid.New()
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// TODO this will work only with newsest uplink
	// figue out what to do with this
	encryptionParameters := storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(req.EncryptionParameters.CipherSuite),
		BlockSize:   int32(req.EncryptionParameters.BlockSize), // TODO check conversion
	}

	var expiresAt *time.Time
	if req.ExpiresAt.IsZero() {
		expiresAt = nil
	} else {
		expiresAt = &req.ExpiresAt
	}

	object, err := endpoint.metabase.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
			StreamID:   streamID,
			Version:    metabase.Version(1),
		},
		ExpiresAt:  expiresAt,
		Encryption: encryptionParameters,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:               req.Bucket,
		EncryptedObjectKey:   req.EncryptedPath,
		Version:              int32(object.Version),
		CreationDate:         object.CreatedAt,
		ExpirationDate:       req.ExpiresAt,
		StreamId:             streamID[:],
		MultipartObject:      object.FixedSegmentSize <= 0,
		EncryptionParameters: req.EncryptionParameters,
		Placement:            int32(placement),
	})
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Object Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "object"))
	mon.Meter("req_put_object").Mark(1)

	return &pb.ObjectBeginResponse{
		Bucket:           req.Bucket,
		EncryptedPath:    req.EncryptedPath,
		Version:          req.Version,
		StreamId:         satStreamID,
		RedundancyScheme: endpoint.defaultRS,
	}, nil
}

// CommitObject commits an object when all its segments have already been committed.
func (endpoint *Endpoint) CommitObject(ctx context.Context, req *pb.ObjectCommitRequest) (resp *pb.ObjectCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	metadataSize := memory.Size(len(req.EncryptedMetadata))
	if metadataSize > endpoint.config.MaxMetadataSize {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, fmt.Sprintf("Metadata is too large, got %v, maximum allowed is %v", metadataSize, endpoint.config.MaxMetadataSize))
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// for old uplinks get Encryption from StreamMeta
	streamMeta := &pb.StreamMeta{}
	encryption := storj.EncryptionParameters{}
	err = pb.Unmarshal(req.EncryptedMetadata, streamMeta)
	if err == nil {
		encryption.CipherSuite = storj.CipherSuite(streamMeta.EncryptionType)
		encryption.BlockSize = streamMeta.EncryptionBlockSize
	}

	request := metabase.CommitObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(1),
		},
		Encryption: encryption,
	}
	// uplink can send empty metadata with not empty key/nonce
	// we need to fix it on uplink side but that part will be
	// needed for backward compatibility
	if len(req.EncryptedMetadata) != 0 {
		request.EncryptedMetadata = req.EncryptedMetadata
		request.EncryptedMetadataNonce = req.EncryptedMetadataNonce[:]
		request.EncryptedMetadataEncryptedKey = req.EncryptedMetadataEncryptedKey

		// older uplinks might send EncryptedMetadata directly with request but
		// key/nonce will be part of StreamMeta
		if req.EncryptedMetadataNonce.IsZero() && len(req.EncryptedMetadataEncryptedKey) == 0 &&
			streamMeta.LastSegmentMeta != nil {
			request.EncryptedMetadataNonce = streamMeta.LastSegmentMeta.KeyNonce
			request.EncryptedMetadataEncryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
		}
	}

	_, err = endpoint.metabase.CommitObject(ctx, request)
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	return &pb.ObjectCommitResponse{}, nil
}

// GetObject gets single object metadata.
func (endpoint *Endpoint) GetObject(ctx context.Context, req *pb.ObjectGetRequest) (resp *pb.ObjectGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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

	mbObject, err := endpoint.metabase.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
		},
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	var segmentRS *pb.RedundancyScheme
	// TODO we may try to avoid additional request for inline objects
	if !req.RedundancySchemePerSegment && mbObject.SegmentCount > 0 {
		segmentRS = endpoint.defaultRS
		segment, err := endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: mbObject.StreamID,
			Position: metabase.SegmentPosition{
				Index: 0,
			},
		})
		if err != nil {
			// don't fail because its possible that its multipart object
			endpoint.log.Error("internal", zap.Error(err))
		} else {
			segmentRS = &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm),
				ErasureShareSize: segment.Redundancy.ShareSize,

				MinReq:           int32(segment.Redundancy.RequiredShares),
				RepairThreshold:  int32(segment.Redundancy.RepairShares),
				SuccessThreshold: int32(segment.Redundancy.OptimalShares),
				Total:            int32(segment.Redundancy.TotalShares),
			}
		}

		// monitor how many uplinks is still using this additional code
		mon.Meter("req_get_object_rs_per_object").Mark(1)
	}

	object, err := endpoint.objectToProto(ctx, mbObject, segmentRS)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Object Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "object"))
	mon.Meter("req_get_object").Mark(1)

	return &pb.ObjectGetResponse{Object: object}, nil
}

// DownloadObject gets object information, creates a download for segments and lists the object segments.
func (endpoint *Endpoint) DownloadObject(ctx context.Context, req *pb.ObjectDownloadRequest) (resp *pb.ObjectDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if ctx.Err() != nil {
		return nil, rpcstatus.Error(rpcstatus.Canceled, "client has closed the connection")
	}

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	if exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfo.ProjectID); err != nil {
		if errs2.IsCanceled(err) {
			return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		endpoint.log.Error(
			"Retrieving project bandwidth total failed; bandwidth limit won't be enforced",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	} else if exceeded {
		endpoint.log.Warn("Monthly bandwidth limit exceeded",
			zap.Stringer("Limit", limit),
			zap.Stringer("Project ID", keyInfo.ProjectID),
		)
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}

	// get the object information

	object, err := endpoint.metabase.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	// get the range segments

	streamRange, err := calculateStreamRange(object, req.Range)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	segments, err := endpoint.metabase.ListStreamPositions(ctx, metabase.ListStreamPositions{
		StreamID: object.StreamID,
		Range:    streamRange,
		Limit:    int(req.Limit),
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	// get the download response for the first segment
	downloadSegments, err := func() ([]*pb.SegmentDownloadResponse, error) {
		if len(segments.Segments) == 0 {
			return nil, nil
		}
		if object.IsMigrated() && streamRange != nil && streamRange.PlainStart > 0 {
			return nil, nil
		}

		segment, err := endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: object.StreamID,
			Position: segments.Segments[0].Position,
		})
		if err != nil {
			return nil, endpoint.convertMetabaseErr(err)
		}

		downloadSizes := endpoint.calculateDownloadSizes(streamRange, segment, object.Encryption)

		// Update the current bandwidth cache value incrementing the SegmentSize.
		err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfo.ProjectID, downloadSizes.encryptedSize)
		if err != nil {
			if errs2.IsCanceled(err) {
				return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
			}

			// log it and continue. it's most likely our own fault that we couldn't
			// track it, and the only thing that will be affected is our per-project
			// bandwidth limits.
			endpoint.log.Error(
				"Could not track the new project's bandwidth usage when downloading an object",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Error(err),
			)
		}

		encryptedKeyNonce, err := storj.NonceFromBytes(segment.EncryptedKeyNonce)
		if err != nil {
			endpoint.log.Error("unable to get encryption key nonce from metadata", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		if segment.Inline() {
			err := endpoint.orders.UpdateGetInlineOrder(ctx, object.Location().Bucket(), downloadSizes.plainSize)
			if err != nil {
				endpoint.log.Error("internal", zap.Error(err))
				return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
			endpoint.log.Info("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
			mon.Meter("req_get_inline").Mark(1)

			return []*pb.SegmentDownloadResponse{{
				PlainOffset:         segment.PlainOffset,
				PlainSize:           int64(segment.PlainSize),
				SegmentSize:         int64(segment.EncryptedSize),
				EncryptedInlineData: segment.InlineData,

				EncryptedKeyNonce: encryptedKeyNonce,
				EncryptedKey:      segment.EncryptedKey,

				Position: &pb.SegmentPosition{
					PartNumber: int32(segment.Position.Part),
					Index:      int32(segment.Position.Index),
				},
			}}, nil
		}

		limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, object.Location().Bucket(), segment, downloadSizes.orderLimit)
		if err != nil {
			if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
				endpoint.log.Error("Unable to create order limits.",
					zap.Stringer("Project ID", keyInfo.ProjectID),
					zap.Stringer("API Key ID", keyInfo.ID),
					zap.Error(err),
				)
			}
			endpoint.log.Error("internal", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		endpoint.log.Info("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
		mon.Meter("req_get_remote").Mark(1)

		return []*pb.SegmentDownloadResponse{{
			AddressedLimits: limits,
			PrivateKey:      privateKey,
			PlainOffset:     segment.PlainOffset,
			PlainSize:       int64(segment.PlainSize),
			SegmentSize:     int64(segment.EncryptedSize),

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      segment.EncryptedKey,
			RedundancyScheme: &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm),
				ErasureShareSize: segment.Redundancy.ShareSize,

				MinReq:           int32(segment.Redundancy.RequiredShares),
				RepairThreshold:  int32(segment.Redundancy.RepairShares),
				SuccessThreshold: int32(segment.Redundancy.OptimalShares),
				Total:            int32(segment.Redundancy.TotalShares),
			},

			Position: &pb.SegmentPosition{
				PartNumber: int32(segment.Position.Part),
				Index:      int32(segment.Position.Index),
			},
		}}, nil
	}()
	if err != nil {
		return nil, err
	}

	// convert to response
	protoObject, err := endpoint.objectToProto(ctx, object, nil)
	if err != nil {
		endpoint.log.Error("unable to convert object to proto", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	segmentList, err := convertStreamListResults(segments)
	if err != nil {
		endpoint.log.Error("unable to convert stream list", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Download Object", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "download"), zap.String("type", "object"))
	mon.Meter("req_download_object").Mark(1)

	return &pb.ObjectDownloadResponse{
		Object: protoObject,

		// The RPC API allows for multiple segment download responses, but for now
		// we return only one. This can be changed in the future if it seems useful
		// to return more than one on the initial response.
		SegmentDownload: downloadSegments,

		// In the case where the client needs the segment list, it will contain
		// every segment. In the case where the segment list is not needed,
		// segmentListItems will be nil.
		SegmentList: segmentList,
	}, nil
}

type downloadSizes struct {
	// amount of data that uplink eventually gets
	plainSize int64
	// amount of data that's present after encryption
	encryptedSize int64
	// amount of data that's read from a storage node
	orderLimit int64
}

func (endpoint *Endpoint) calculateDownloadSizes(streamRange *metabase.StreamRange, segment metabase.Segment, encryptionParams storj.EncryptionParameters) downloadSizes {
	if segment.Inline() {
		return downloadSizes{
			plainSize:     int64(len(segment.InlineData)),
			encryptedSize: int64(segment.EncryptedSize),
		}
	}

	// calculate the range inside the given segment
	readStart := segment.PlainOffset
	if streamRange != nil && readStart <= streamRange.PlainStart {
		readStart = streamRange.PlainStart
	}
	readLimit := segment.PlainOffset + int64(segment.PlainSize)
	if streamRange != nil && streamRange.PlainLimit < readLimit {
		readLimit = streamRange.PlainLimit
	}

	plainSize := readLimit - readStart

	// calculate the read range given the segment start
	readStart -= segment.PlainOffset
	readLimit -= segment.PlainOffset

	// align to encryption block size
	enc, err := encryption.NewEncrypter(encryptionParams.CipherSuite, &storj.Key{1}, &storj.Nonce{1}, int(encryptionParams.BlockSize))
	if err != nil {
		// We ignore the error and fallback to the max amount to download.
		// It's unlikely that we fail here, but if we do, we don't want to block downloading.
		endpoint.log.Error("unable to create encrypter", zap.Error(err))
		return downloadSizes{
			plainSize:     int64(segment.PlainSize),
			encryptedSize: int64(segment.EncryptedSize),
			orderLimit:    0,
		}
	}

	encryptedStartBlock, encryptedLimitBlock := calculateBlocks(readStart, readLimit, int64(enc.InBlockSize()))
	encryptedStart, encryptedLimit := encryptedStartBlock*int64(enc.OutBlockSize()), encryptedLimitBlock*int64(enc.OutBlockSize())
	encryptedSize := encryptedLimit - encryptedStart

	if encryptedSize > int64(segment.EncryptedSize) {
		encryptedSize = int64(segment.EncryptedSize)
	}

	// align to blocks
	stripeSize := int64(segment.Redundancy.StripeSize())
	stripeStart, stripeLimit := alignToBlock(encryptedStart, encryptedLimit, stripeSize)

	// calculate how much shares we need to download from a node
	stripeCount := (stripeLimit - stripeStart) / stripeSize
	orderLimit := stripeCount * int64(segment.Redundancy.ShareSize)

	return downloadSizes{
		plainSize:     plainSize,
		encryptedSize: encryptedSize,
		orderLimit:    orderLimit,
	}
}

func calculateBlocks(start, limit int64, blockSize int64) (startBlock, limitBlock int64) {
	return start / blockSize, (limit + blockSize - 1) / blockSize
}

func alignToBlock(start, limit int64, blockSize int64) (alignedStart, alignedLimit int64) {
	return (start / blockSize) * blockSize, ((limit + blockSize - 1) / blockSize) * blockSize
}

func calculateStreamRange(object metabase.Object, req *pb.Range) (*metabase.StreamRange, error) {
	if req == nil || req.Range == nil {
		return nil, nil
	}

	if object.IsMigrated() {
		// The object is in old format, which does not have plain_offset specified.
		// We need to fallback to returning all segments.
		return nil, nil
	}

	switch r := req.Range.(type) {
	case *pb.Range_Start:
		if r.Start == nil {
			return nil, Error.New("Start missing for Range_Start")
		}

		return &metabase.StreamRange{
			PlainStart: r.Start.PlainStart,
			PlainLimit: object.TotalPlainSize,
		}, nil
	case *pb.Range_StartLimit:
		if r.StartLimit == nil {
			return nil, Error.New("StartEnd missing for Range_StartEnd")
		}
		return &metabase.StreamRange{
			PlainStart: r.StartLimit.PlainStart,
			PlainLimit: r.StartLimit.PlainLimit,
		}, nil
	case *pb.Range_Suffix:
		if r.Suffix == nil {
			return nil, Error.New("Suffix missing for Range_Suffix")
		}
		return &metabase.StreamRange{
			PlainStart: object.TotalPlainSize - r.Suffix.PlainSuffix,
			PlainLimit: object.TotalPlainSize,
		}, nil
	}

	// if it's a new unsupported range type, let's return all data
	return nil, nil
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
	placement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	limit := int(req.Limit)
	if limit < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "limit is negative")
	}
	metabase.ListLimit.Ensure(&limit)

	var prefix metabase.ObjectKey
	if len(req.EncryptedPrefix) != 0 {
		prefix = metabase.ObjectKey(req.EncryptedPrefix)
		if prefix[len(prefix)-1] != metabase.Delimiter {
			prefix += metabase.ObjectKey(metabase.Delimiter)
		}
	}

	// Default to Commmitted status for backward-compatibility with older uplinks.
	status := metabase.Committed
	if req.Status != pb.Object_INVALID {
		status = metabase.ObjectStatus(req.Status)
	}

	cursor := string(req.EncryptedCursor)
	if len(cursor) != 0 {
		cursor = string(prefix) + cursor
	}

	includeCustomMetadata := true
	includeSystemMetadata := true
	if req.UseObjectIncludes {
		includeCustomMetadata = req.ObjectIncludes.Metadata
		includeSystemMetadata = !req.ObjectIncludes.ExcludeSystemMetadata
	}

	resp = &pb.ObjectListResponse{}
	// TODO: Replace with IterateObjectsLatestVersion when ready
	err = endpoint.metabase.IterateObjectsAllVersionsWithStatus(ctx,
		metabase.IterateObjectsWithStatus{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			Prefix:     prefix,
			Cursor: metabase.IterateCursor{
				Key:     metabase.ObjectKey(cursor),
				Version: 1, // TODO: set to a the version from the protobuf request when it supports this
			},
			Recursive:             req.Recursive,
			BatchSize:             limit + 1,
			Status:                status,
			IncludeCustomMetadata: includeCustomMetadata,
			IncludeSystemMetadata: includeSystemMetadata,
		}, func(ctx context.Context, it metabase.ObjectsIterator) error {
			entry := metabase.ObjectEntry{}
			for len(resp.Items) < limit && it.Next(ctx, &entry) {
				item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, includeCustomMetadata, placement)
				if err != nil {
					return err
				}
				resp.Items = append(resp.Items, item)
			}
			resp.More = it.Next(ctx, &entry)
			return nil
		},
	)
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	endpoint.log.Info("Object List", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "list"), zap.String("type", "object"))
	mon.Meter("req_list_object").Mark(1)

	return resp, nil
}

// ListPendingObjectStreams list pending objects according to specific parameters.
func (endpoint *Endpoint) ListPendingObjectStreams(ctx context.Context, req *pb.ObjectListPendingStreamsRequest) (resp *pb.ObjectListPendingStreamsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionList,
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

	placement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	cursor := metabase.StreamIDCursor{}
	if req.StreamIdCursor != nil {
		streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamIdCursor)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
		cursor.StreamID, err = uuid.FromBytes(streamID.StreamId)
		if err != nil {
			endpoint.log.Error("internal", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	limit := int(req.Limit)
	if limit < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "limit is negative")
	}
	metabase.ListLimit.Ensure(&limit)

	resp = &pb.ObjectListPendingStreamsResponse{}
	resp.Items = []*pb.ObjectListItem{}
	err = endpoint.metabase.IteratePendingObjectsByKey(ctx,
		metabase.IteratePendingObjectsByKey{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: string(req.Bucket),
				ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
			},
			BatchSize: limit + 1,
			Cursor:    cursor,
		}, func(ctx context.Context, it metabase.ObjectsIterator) error {
			entry := metabase.ObjectEntry{}
			for len(resp.Items) < limit && it.Next(ctx, &entry) {
				item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, "", true, placement)
				if err != nil {
					return err
				}
				resp.Items = append(resp.Items, item)
			}
			resp.More = it.Next(ctx, &entry)
			return nil
		},
	)
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	endpoint.log.Info("List pending object streams", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "list"), zap.String("type", "object"))

	mon.Meter("req_list_pending_object_streams").Mark(1)

	return resp, nil
}

// BeginDeleteObject begins object deletion process.
func (endpoint *Endpoint) BeginDeleteObject(ctx context.Context, req *pb.ObjectBeginDeleteRequest) (resp *pb.ObjectBeginDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	now := time.Now()

	var canRead, canList bool

	keyInfo, err := endpoint.validateAuthN(ctx, req.Header,
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedPath,
				Time:          now,
			},
		},
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedPath,
				Time:          now,
			},
			actionPermitted: &canRead,
			optional:        true,
		},
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionList,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedPath,
				Time:          now,
			},
			actionPermitted: &canList,
			optional:        true,
		},
	)
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	var deletedObjects []*pb.Object

	if req.GetStatus() == int32(metabase.Pending) {
		if req.StreamId == nil {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "StreamID missing")
		}
		var pbStreamID *internalpb.StreamID
		pbStreamID, err = endpoint.unmarshalSatStreamID(ctx, *(req.StreamId))
		if err == nil {
			var streamID uuid.UUID
			streamID, err = uuid.FromBytes(pbStreamID.StreamId)
			if err == nil {
				deletedObjects, err = endpoint.DeletePendingObject(ctx,
					metabase.ObjectStream{
						ProjectID:  keyInfo.ProjectID,
						BucketName: string(req.Bucket),
						ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
						Version:    metabase.Version(req.GetVersion()),
						StreamID:   streamID,
					})
			}
		}
	} else {
		deletedObjects, err = endpoint.DeleteCommittedObject(ctx, keyInfo.ProjectID, string(req.Bucket), metabase.ObjectKey(req.EncryptedPath))
	}
	if err != nil {
		if !canRead && !canList {
			// No error info is returned if neither Read, nor List permission is granted
			return &pb.ObjectBeginDeleteResponse{}, nil
		}
		return nil, endpoint.convertMetabaseErr(err)
	}

	var object *pb.Object
	if canRead || canList {
		// Info about deleted object is returned only if either Read, or List permission is granted
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

// GetObjectIPs returns the IP addresses of the nodes holding the pieces for
// the provided object. This is useful for knowing the locations of the pieces.
func (endpoint *Endpoint) GetObjectIPs(ctx context.Context, req *pb.ObjectGetIPsRequest) (resp *pb.ObjectGetIPsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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

	// TODO we may need custom metabase request to avoid two DB calls
	object, err := endpoint.metabase.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedPath),
		},
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	pieceCountByNodeID, err := endpoint.metabase.GetStreamPieceCountByNodeID(ctx,
		metabase.GetStreamPieceCountByNodeID{
			StreamID: object.StreamID,
		})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	nodeIDs := make([]storj.NodeID, 0, len(pieceCountByNodeID))
	for nodeID := range pieceCountByNodeID {
		nodeIDs = append(nodeIDs, nodeID)
	}

	nodeIPMap, err := endpoint.overlay.GetNodeIPs(ctx, nodeIDs)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	nodeIPs := make([][]byte, 0, len(nodeIPMap))
	pieceCount := int64(0)
	reliablePieceCount := int64(0)
	for nodeID, count := range pieceCountByNodeID {
		pieceCount += count

		ip, reliable := nodeIPMap[nodeID]
		if !reliable {
			continue
		}
		nodeIPs = append(nodeIPs, []byte(ip))
		reliablePieceCount += count
	}

	return &pb.ObjectGetIPsResponse{
		Ips:                nodeIPs,
		SegmentCount:       int64(object.SegmentCount),
		ReliablePieceCount: reliablePieceCount,
		PieceCount:         pieceCount,
	}, nil
}

// UpdateObjectMetadata replaces object metadata.
func (endpoint *Endpoint) UpdateObjectMetadata(ctx context.Context, req *pb.ObjectUpdateMetadataRequest) (resp *pb.ObjectUpdateMetadataResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var encryptedMetadataNonce []byte
	if !req.EncryptedMetadataNonce.IsZero() {
		encryptedMetadataNonce = req.EncryptedMetadataNonce[:]
	}

	err = endpoint.metabase.UpdateObjectMetadata(ctx, metabase.UpdateObjectMetadata{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			Version:    metabase.Version(req.Version),
			StreamID:   id,
		},
		EncryptedMetadata:             req.EncryptedMetadata,
		EncryptedMetadataNonce:        encryptedMetadataNonce,
		EncryptedMetadataEncryptedKey: req.EncryptedMetadataEncryptedKey,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	return &pb.ObjectUpdateMetadataResponse{}, nil
}

// BeginSegment begins segment uploading.
func (endpoint *Endpoint) BeginSegment(ctx context.Context, req *pb.SegmentBeginRequest) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// no need to validate streamID fields because it was validated during BeginObject

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(endpoint.defaultRS)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	maxPieceSize := eestream.CalcPieceSize(req.MaxOrderLimit, redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.TotalCount(),
		Placement:      storj.PlacementConstraint(streamID.Placement),
	}
	nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	rootPieceID, addressedLimits, piecePrivateKey, err := endpoint.orders.CreatePutOrderLimits(ctx, bucket, nodes, streamID.ExpirationDate, maxPieceSize)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	pieces := metabase.Pieces{}
	for i, limit := range addressedLimits {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: limit.Limit.StorageNodeId,
		})
	}
	err = endpoint.metabase.BeginSegment(ctx, metabase.BeginSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
		},
		Position: metabase.SegmentPosition{
			Part:  uint32(req.Position.PartNumber),
			Index: uint32(req.Position.Index),
		},
		RootPieceID: rootPieceID,
		Pieces:      pieces,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{
		StreamId:            streamID,
		PartNumber:          req.Position.PartNumber,
		Index:               req.Position.Index,
		OriginalOrderLimits: addressedLimits,
		RootPieceId:         rootPieceID,
		CreationDate:        time.Now(),
	})

	endpoint.log.Info("Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "remote"))
	mon.Meter("req_put_remote").Mark(1)

	return &pb.SegmentBeginResponse{
		SegmentId:        segmentID,
		AddressedLimits:  addressedLimits,
		PrivateKey:       piecePrivateKey,
		RedundancyScheme: endpoint.defaultRS,
	}, nil
}

// CommitSegment commits segment after uploading.
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamID := segmentID.StreamId

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// cheap basic verification
	if numResults := len(req.UploadResult); numResults < int(endpoint.defaultRS.GetSuccessThreshold()) {
		endpoint.log.Debug("the results of uploaded pieces for the segment is below the redundancy optimal threshold",
			zap.Int("upload pieces results", numResults),
			zap.Int32("redundancy optimal threshold", endpoint.defaultRS.GetSuccessThreshold()),
			zap.Stringer("Segment ID", req.SegmentId),
		)
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"the number of results of uploaded pieces (%d) is below the optimal threshold (%d)",
			numResults, endpoint.defaultRS.GetSuccessThreshold(),
		)
	}

	rs := storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(endpoint.defaultRS.Type),
		RequiredShares: int16(endpoint.defaultRS.MinReq),
		RepairShares:   int16(endpoint.defaultRS.RepairThreshold),
		OptimalShares:  int16(endpoint.defaultRS.SuccessThreshold),
		TotalShares:    int16(endpoint.defaultRS.Total),
		ShareSize:      endpoint.defaultRS.ErasureShareSize,
	}

	err = endpoint.pointerVerification.VerifySizes(ctx, rs, req.SizeEncryptedData, req.UploadResult)
	if err != nil {
		endpoint.log.Debug("piece sizes are invalid", zap.Error(err))
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece sizes are invalid: %v", err)
	}

	// extract the original order limits
	originalLimits := make([]*pb.OrderLimit, len(segmentID.OriginalOrderLimits))
	for i, orderLimit := range segmentID.OriginalOrderLimits {
		originalLimits[i] = orderLimit.Limit
	}

	// verify the piece upload results
	validPieces, invalidPieces, err := endpoint.pointerVerification.SelectValidPieces(ctx, req.UploadResult, originalLimits)
	if err != nil {
		endpoint.log.Debug("pointer verification failed", zap.Error(err))
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "pointer verification failed: %s", err)
	}

	if len(validPieces) < int(rs.OptimalShares) {
		endpoint.log.Debug("Number of valid pieces is less than the success threshold",
			zap.Int("totalReceivedPieces", len(req.UploadResult)),
			zap.Int("validPieces", len(validPieces)),
			zap.Int("invalidPieces", len(invalidPieces)),
			zap.Int("successThreshold", int(rs.OptimalShares)),
		)

		errMsg := fmt.Sprintf("Number of valid pieces (%d) is less than the success threshold (%d). Found %d invalid pieces",
			len(validPieces),
			rs.OptimalShares,
			len(invalidPieces),
		)
		if len(invalidPieces) > 0 {
			errMsg = fmt.Sprintf("%s. Invalid Pieces:", errMsg)
			for _, p := range invalidPieces {
				errMsg = fmt.Sprintf("%s\nNodeID: %v, PieceNum: %d, Reason: %s",
					errMsg, p.NodeID, p.PieceNum, p.Reason,
				)
			}
		}
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errMsg)
	}

	pieces := metabase.Pieces{}
	for _, result := range validPieces {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(result.PieceNum),
			StorageNode: result.NodeId,
		})
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	mbCommitSegment := metabase.CommitSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
		},
		ExpiresAt:         expiresAt,
		EncryptedKey:      req.EncryptedKey,
		EncryptedKeyNonce: req.EncryptedKeyNonce[:],

		EncryptedSize: int32(req.SizeEncryptedData), // TODO incompatible types int32 vs int64
		PlainSize:     int32(req.PlainSize),         // TODO incompatible types int32 vs int64

		EncryptedETag: req.EncryptedETag,

		Position: metabase.SegmentPosition{
			Part:  uint32(segmentID.PartNumber),
			Index: uint32(segmentID.Index),
		},
		RootPieceID: segmentID.RootPieceId,
		Redundancy:  rs,
		Pieces:      pieces,
		Placement:   storj.PlacementConstraint(streamID.Placement),
	}

	err = endpoint.validateRemoteSegment(ctx, mbCommitSegment, originalLimits)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	segmentSize := req.SizeEncryptedData
	totalStored := calculateSpaceUsed(segmentSize, len(pieces), rs)

	// ToDo: Replace with hash & signature validation
	// Ensure neither uplink or storage nodes are cheating on us

	// We cannot have more redundancy than total/min
	if float64(totalStored) > (float64(segmentSize)/float64(rs.RequiredShares))*float64(rs.TotalShares) {
		endpoint.log.Debug("data size mismatch",
			zap.Int64("segment", segmentSize),
			zap.Int64("pieces", totalStored),
			zap.Int16("redundancy minimum requested", rs.RequiredShares),
			zap.Int16("redundancy total", rs.TotalShares),
		)
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "mismatched segment size and piece usage")
	}

	err = endpoint.metabase.CommitSegment(ctx, mbCommitSegment)
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	if err := endpoint.updateUploadLimits(ctx, keyInfo.ProjectID, segmentSize); err != nil {
		return nil, err
	}

	return &pb.SegmentCommitResponse{
		SuccessfulPieces: int32(len(pieces)),
	}, nil
}

// MakeInlineSegment makes inline segment on satellite.
func (endpoint *Endpoint) MakeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest) (resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	inlineUsed := int64(len(req.EncryptedInlineData))
	if inlineUsed > endpoint.encInlineSegmentSize {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "inline segment size cannot be larger than %s", endpoint.config.MaxInlineSegmentSize)
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	err = endpoint.metabase.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
		},
		ExpiresAt:         expiresAt,
		EncryptedKey:      req.EncryptedKey,
		EncryptedKeyNonce: req.EncryptedKeyNonce.Bytes(),

		Position: metabase.SegmentPosition{
			Part:  uint32(req.Position.PartNumber),
			Index: uint32(req.Position.Index),
		},

		PlainSize:     int32(req.PlainSize), // TODO incompatible types int32 vs int64
		EncryptedETag: req.EncryptedETag,

		InlineData: req.EncryptedInlineData,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	err = endpoint.orders.UpdatePutInlineOrder(ctx, bucket, inlineUsed)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if err := endpoint.updateUploadLimits(ctx, keyInfo.ProjectID, inlineUsed); err != nil {
		return nil, err
	}

	endpoint.log.Info("Inline Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "inline"))
	mon.Meter("req_put_inline").Mark(1)

	return &pb.SegmentMakeInlineResponse{}, nil
}

// ListSegments list object segments.
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.SegmentListRequest) (resp *pb.SegmentListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	cursor := req.CursorPosition
	if cursor == nil {
		cursor = &pb.SegmentPosition{}
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	result, err := endpoint.metabase.ListStreamPositions(ctx, metabase.ListStreamPositions{
		StreamID: id,
		Cursor: metabase.SegmentPosition{
			Part:  uint32(cursor.PartNumber),
			Index: uint32(cursor.Index),
		},
		Limit: int(req.Limit),
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	response, err := convertStreamListResults(result)
	if err != nil {
		endpoint.log.Error("unable to convert stream list", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	response.EncryptionParameters = streamID.EncryptionParameters
	return response, nil
}

func convertStreamListResults(result metabase.ListStreamPositionsResult) (*pb.SegmentListResponse, error) {
	items := make([]*pb.SegmentListItem, len(result.Segments))
	for i, item := range result.Segments {
		items[i] = &pb.SegmentListItem{
			Position: &pb.SegmentPosition{
				PartNumber: int32(item.Position.Part),
				Index:      int32(item.Position.Index),
			},
			PlainSize:   int64(item.PlainSize),
			PlainOffset: item.PlainOffset,
		}
		if item.CreatedAt != nil {
			items[i].CreatedAt = *item.CreatedAt
		}
		items[i].EncryptedETag = item.EncryptedETag
		var err error
		items[i].EncryptedKeyNonce, err = storj.NonceFromBytes(item.EncryptedKeyNonce)
		if err != nil {
			return nil, err
		}
		items[i].EncryptedKey = item.EncryptedKey
	}
	return &pb.SegmentListResponse{
		Items: items,
		More:  result.More,
	}, nil
}

// DownloadSegment returns data necessary to download segment.
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if ctx.Err() != nil {
		return nil, rpcstatus.Error(rpcstatus.Canceled, "client has closed the connection")
	}

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}

	if exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfo.ProjectID); err != nil {
		if errs2.IsCanceled(err) {
			return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		endpoint.log.Error(
			"Retrieving project bandwidth total failed; bandwidth limit won't be enforced",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	} else if exceeded {
		endpoint.log.Warn("Monthly bandwidth limit exceeded",
			zap.Stringer("Limit", limit),
			zap.Stringer("Project ID", keyInfo.ProjectID),
		)
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var segment metabase.Segment
	if req.CursorPosition.PartNumber == 0 && req.CursorPosition.Index == -1 {
		if streamID.MultipartObject {
			return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Used uplink version cannot download multipart objects.")
		}

		segment, err = endpoint.metabase.GetLatestObjectLastSegment(ctx, metabase.GetLatestObjectLastSegment{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: string(streamID.Bucket),
				ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			},
		})
	} else {
		segment, err = endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: id,
			Position: metabase.SegmentPosition{
				Part:  uint32(req.CursorPosition.PartNumber),
				Index: uint32(req.CursorPosition.Index),
			},
		})
	}
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	// Update the current bandwidth cache value incrementing the SegmentSize.
	err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfo.ProjectID, int64(segment.EncryptedSize))
	if err != nil {
		if errs2.IsCanceled(err) {
			return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// bandwidth limits.
		endpoint.log.Error("Could not track the new project's bandwidth usage when downloading a segment",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	encryptedKeyNonce, err := storj.NonceFromBytes(segment.EncryptedKeyNonce)
	if err != nil {
		endpoint.log.Error("unable to get encryption key nonce from metadata", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if segment.Inline() {
		err := endpoint.orders.UpdateGetInlineOrder(ctx, bucket, int64(len(segment.InlineData)))
		if err != nil {
			endpoint.log.Error("internal", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		endpoint.log.Info("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
		mon.Meter("req_get_inline").Mark(1)

		return &pb.SegmentDownloadResponse{
			PlainOffset:         segment.PlainOffset,
			PlainSize:           int64(segment.PlainSize),
			SegmentSize:         int64(segment.EncryptedSize),
			EncryptedInlineData: segment.InlineData,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      segment.EncryptedKey,
			Position: &pb.SegmentPosition{
				PartNumber: int32(segment.Position.Part),
				Index:      int32(segment.Position.Index),
			},
		}, nil
	}

	// Remote segment
	limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, bucket, segment, 0)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			endpoint.log.Error("Unable to create order limits.",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Stringer("API Key ID", keyInfo.ID),
				zap.Error(err),
			)
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
	mon.Meter("req_get_remote").Mark(1)

	return &pb.SegmentDownloadResponse{
		AddressedLimits: limits,
		PrivateKey:      privateKey,
		PlainOffset:     segment.PlainOffset,
		PlainSize:       int64(segment.PlainSize),
		SegmentSize:     int64(segment.EncryptedSize),

		EncryptedKeyNonce: encryptedKeyNonce,
		EncryptedKey:      segment.EncryptedKey,
		RedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm),
			ErasureShareSize: segment.Redundancy.ShareSize,

			MinReq:           int32(segment.Redundancy.RequiredShares),
			RepairThreshold:  int32(segment.Redundancy.RepairShares),
			SuccessThreshold: int32(segment.Redundancy.OptimalShares),
			Total:            int32(segment.Redundancy.TotalShares),
		},
		Position: &pb.SegmentPosition{
			PartNumber: int32(segment.Position.Part),
			Index:      int32(segment.Position.Index),
		},
	}, nil
}

// DeletePart is a no-op.
//
// It was used to perform the deletion of a single part from satellite db and
// from storage nodes. We made this method noop because now we can overwrite
// segments for pending objects. It's returning no error to avoid failures with
// uplinks that still are using this method.
func (endpoint *Endpoint) DeletePart(ctx context.Context, req *pb.PartDeleteRequest) (resp *pb.PartDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return &pb.PartDeleteResponse{}, nil
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

// DeleteCommittedObject deletes all the pieces of the storage nodes that belongs
// to the specified object.
//
// NOTE: this method is exported for being able to individually test it without
// having import cycles.
func (endpoint *Endpoint) DeleteCommittedObject(
	ctx context.Context, projectID uuid.UUID, bucket string, object metabase.ObjectKey,
) (deletedObjects []*pb.Object, err error) {
	defer mon.Task()(&ctx, projectID.String(), bucket, object)(&err)

	req := metabase.ObjectLocation{
		ProjectID:  projectID,
		BucketName: bucket,
		ObjectKey:  object,
	}

	result, err := endpoint.metabase.DeleteObjectsAllVersions(ctx, metabase.DeleteObjectsAllVersions{Locations: []metabase.ObjectLocation{req}})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	deletedObjects, err = endpoint.deleteObjectsPieces(ctx, result)
	if err != nil {
		endpoint.log.Error("failed to delete pointers",
			zap.Stringer("project", projectID),
			zap.String("bucket", bucket),
			zap.Binary("object", []byte(object)),
			zap.Error(err),
		)
		return deletedObjects, Error.Wrap(err)
	}

	return deletedObjects, nil
}

// DeleteObjectAnyStatus deletes all the pieces of the storage nodes that belongs
// to the specified object.
//
// NOTE: this method is exported for being able to individually test it without
// having import cycles.
func (endpoint *Endpoint) DeleteObjectAnyStatus(ctx context.Context, location metabase.ObjectLocation,
) (deletedObjects []*pb.Object, err error) {
	defer mon.Task()(&ctx, location.ProjectID.String(), location.BucketName, location.ObjectKey)(&err)

	result, err := endpoint.metabase.DeleteObjectAnyStatusAllVersions(ctx, metabase.DeleteObjectAnyStatusAllVersions{
		ObjectLocation: location,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	deletedObjects, err = endpoint.deleteObjectsPieces(ctx, result)
	if err != nil {
		endpoint.log.Error("failed to delete pointers",
			zap.Stringer("project", location.ProjectID),
			zap.String("bucket", location.BucketName),
			zap.Binary("object", []byte(location.ObjectKey)),
			zap.Error(err),
		)
		return deletedObjects, err
	}

	return deletedObjects, nil
}

// DeletePendingObject deletes all the pieces of the storage nodes that belongs
// to the specified pending object.
//
// NOTE: this method is exported for being able to individually test it without
// having import cycles.
func (endpoint *Endpoint) DeletePendingObject(ctx context.Context, stream metabase.ObjectStream) (deletedObjects []*pb.Object, err error) {
	req := metabase.DeletePendingObject{
		ObjectStream: stream,
	}
	result, err := endpoint.metabase.DeletePendingObject(ctx, req)
	if err != nil {
		return nil, err
	}

	return endpoint.deleteObjectsPieces(ctx, result)
}

func (endpoint *Endpoint) deleteObjectsPieces(ctx context.Context, result metabase.DeleteObjectResult) (deletedObjects []*pb.Object, err error) {
	defer mon.Task()(&ctx)(&err)
	// We should ignore client cancelling and always try to delete segments.
	ctx = context2.WithoutCancellation(ctx)
	deletedObjects = make([]*pb.Object, len(result.Objects))
	for i, object := range result.Objects {
		deletedObject, err := endpoint.objectToProto(ctx, object, endpoint.defaultRS)
		if err != nil {
			return nil, err
		}
		deletedObjects[i] = deletedObject
	}

	endpoint.deleteSegmentPieces(ctx, result.Segments)

	return deletedObjects, nil
}

func (endpoint *Endpoint) deleteSegmentPieces(ctx context.Context, segments []metabase.DeletedSegmentInfo) {
	var err error
	defer mon.Task()(&ctx)(&err)

	nodesPieces := groupPiecesByNodeID(segments)

	var requests []piecedeletion.Request
	for node, pieces := range nodesPieces {
		requests = append(requests, piecedeletion.Request{
			Node: storj.NodeURL{
				ID: node,
			},
			Pieces: pieces,
		})
	}

	// Only return an error if we failed to delete the objects. If we failed
	// to delete pieces, let garbage collector take care of it.
	err = endpoint.deletePieces.Delete(ctx, requests, deleteObjectPiecesSuccessThreshold)
	if err != nil {
		endpoint.log.Error("failed to delete pieces", zap.Error(err))
	}
}

func (endpoint *Endpoint) objectToProto(ctx context.Context, object metabase.Object, rs *pb.RedundancyScheme) (*pb.Object, error) {
	expires := time.Time{}
	if object.ExpiresAt != nil {
		expires = *object.ExpiresAt
	}

	// TotalPlainSize != 0 means object was uploaded with newer uplink
	multipartObject := object.TotalPlainSize != 0 && object.FixedSegmentSize <= 0
	streamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:             []byte(object.BucketName),
		EncryptedObjectKey: []byte(object.ObjectKey),
		Version:            int32(object.Version), // TODO incomatible types
		CreationDate:       object.CreatedAt,
		ExpirationDate:     expires,
		StreamId:           object.StreamID[:],
		MultipartObject:    multipartObject,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(object.Encryption.CipherSuite),
			BlockSize:   int64(object.Encryption.BlockSize),
		},
		// TODO: this is the only one place where placement is not added to the StreamID
		// bucket info  would be required to add placement here
	})
	if err != nil {
		return nil, err
	}

	var nonce storj.Nonce
	if len(object.EncryptedMetadataNonce) > 0 {
		nonce, err = storj.NonceFromBytes(object.EncryptedMetadataNonce)
		if err != nil {
			return nil, err
		}
	}

	streamMeta := &pb.StreamMeta{}
	err = pb.Unmarshal(object.EncryptedMetadata, streamMeta)
	if err != nil {
		return nil, err
	}

	// TODO is this enough to handle old uplinks
	if streamMeta.EncryptionBlockSize == 0 {
		streamMeta.EncryptionBlockSize = object.Encryption.BlockSize
	}
	if streamMeta.EncryptionType == 0 {
		streamMeta.EncryptionType = int32(object.Encryption.CipherSuite)
	}
	if streamMeta.NumberOfSegments == 0 {
		streamMeta.NumberOfSegments = int64(object.SegmentCount)
	}
	if streamMeta.LastSegmentMeta == nil {
		streamMeta.LastSegmentMeta = &pb.SegmentMeta{
			EncryptedKey: object.EncryptedMetadataEncryptedKey,
			KeyNonce:     object.EncryptedMetadataNonce,
		}
	}

	metadataBytes, err := pb.Marshal(streamMeta)
	if err != nil {
		return nil, err
	}

	result := &pb.Object{
		Bucket:        []byte(object.BucketName),
		EncryptedPath: []byte(object.ObjectKey),
		Version:       int32(object.Version), // TODO incomatible types
		StreamId:      streamID,
		ExpiresAt:     expires,
		CreatedAt:     object.CreatedAt,

		TotalSize: object.TotalEncryptedSize,
		PlainSize: object.TotalPlainSize,

		EncryptedMetadata:             metadataBytes,
		EncryptedMetadataNonce:        nonce,
		EncryptedMetadataEncryptedKey: object.EncryptedMetadataEncryptedKey,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(object.Encryption.CipherSuite),
			BlockSize:   int64(object.Encryption.BlockSize),
		},

		RedundancyScheme: rs,
	}

	return result, nil
}

func (endpoint *Endpoint) objectEntryToProtoListItem(ctx context.Context, bucket []byte,
	entry metabase.ObjectEntry, prefixToPrependInSatStreamID metabase.ObjectKey,
	includeMetadata bool, placement storj.PlacementConstraint) (item *pb.ObjectListItem, err error) {

	expires := time.Time{}
	if entry.ExpiresAt != nil {
		expires = *entry.ExpiresAt
	}

	item = &pb.ObjectListItem{
		EncryptedPath: []byte(entry.ObjectKey),
		Version:       int32(entry.Version), // TODO incompatible types
		Status:        pb.Object_Status(entry.Status),
		ExpiresAt:     expires,
		CreatedAt:     entry.CreatedAt,
		PlainSize:     entry.TotalPlainSize,
	}

	if includeMetadata {
		var nonce storj.Nonce
		if len(entry.EncryptedMetadataNonce) > 0 {
			nonce, err = storj.NonceFromBytes(entry.EncryptedMetadataNonce)
			if err != nil {
				return nil, err
			}
		}

		streamMeta := &pb.StreamMeta{}
		err = pb.Unmarshal(entry.EncryptedMetadata, streamMeta)
		if err != nil {
			return nil, err
		}

		if entry.Encryption != (storj.EncryptionParameters{}) {
			streamMeta.EncryptionType = int32(entry.Encryption.CipherSuite)
			streamMeta.EncryptionBlockSize = entry.Encryption.BlockSize
		}

		if entry.SegmentCount != 0 {
			streamMeta.NumberOfSegments = int64(entry.SegmentCount)
		}

		if entry.EncryptedMetadataEncryptedKey != nil {
			streamMeta.LastSegmentMeta = &pb.SegmentMeta{
				EncryptedKey: entry.EncryptedMetadataEncryptedKey,
				KeyNonce:     entry.EncryptedMetadataNonce,
			}
		}

		metadataBytes, err := pb.Marshal(streamMeta)
		if err != nil {
			return nil, err
		}

		item.EncryptedMetadata = metadataBytes
		item.EncryptedMetadataNonce = nonce
		item.EncryptedMetadataEncryptedKey = entry.EncryptedMetadataEncryptedKey
	}

	// Add Stream ID to list items if listing is for pending objects.
	// The client requires the Stream ID to use in the MultipartInfo.
	if entry.Status == metabase.Pending {
		satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
			Bucket:             bucket,
			EncryptedObjectKey: append([]byte(prefixToPrependInSatStreamID), item.EncryptedPath...),
			Version:            item.Version,
			CreationDate:       item.CreatedAt,
			ExpirationDate:     item.ExpiresAt,
			StreamId:           entry.StreamID[:],
			MultipartObject:    entry.FixedSegmentSize <= 0,
			EncryptionParameters: &pb.EncryptionParameters{
				CipherSuite: pb.CipherSuite(entry.Encryption.CipherSuite),
				BlockSize:   int64(entry.Encryption.BlockSize),
			},
			Placement: int32(placement),
		})
		if err != nil {
			return nil, err
		}
		item.StreamId = &satStreamID
	}

	return item, nil
}

// groupPiecesByNodeID returns a map that contains pieces with node id as the key.
func groupPiecesByNodeID(segments []metabase.DeletedSegmentInfo) map[storj.NodeID][]storj.PieceID {
	piecesToDelete := map[storj.NodeID][]storj.PieceID{}

	for _, segment := range segments {
		for _, piece := range segment.Pieces {
			pieceID := segment.RootPieceID.Derive(piece.StorageNode, int32(piece.Number))
			piecesToDelete[piece.StorageNode] = append(piecesToDelete[piece.StorageNode], pieceID)
		}
	}

	return piecesToDelete
}

// RevokeAPIKey handles requests to revoke an api key.
func (endpoint *Endpoint) RevokeAPIKey(ctx context.Context, req *pb.RevokeAPIKeyRequest) (resp *pb.RevokeAPIKeyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

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

// Server side move.

// BeginMoveObject begins moving object to different key.
func (endpoint *Endpoint) BeginMoveObject(ctx context.Context, req *pb.ObjectBeginMoveRequest) (resp *pb.ObjectBeginMoveResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	now := time.Now()
	keyInfo, err := endpoint.validateAuthN(ctx, req.Header,
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		verifyPermission{
			action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	for _, bucket := range [][]byte{req.Bucket, req.NewBucket} {
		err = endpoint.validateBucket(ctx, bucket)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
	}

	// we are verifying existence of target bucket only because source bucket
	// will be checked while quering source object
	// TODO this needs to be optimized to avoid DB call on each request
	newBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.NewBucket, keyInfo.ProjectID)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// if source and target buckets are different, we need to check their geofencing configs
	if !bytes.Equal(req.Bucket, req.NewBucket) {
		oldBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
		if err != nil {
			if storj.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
			}
			endpoint.log.Error("unable to check bucket", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		if oldBucketPlacement != newBucketPlacement {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "moving object to bucket with different placement policy is not (yet) supported")
		}
	}

	result, err := endpoint.metabase.BeginMoveObject(ctx, metabase.BeginMoveObject{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
		Version: metabase.DefaultVersion,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	response, err := convertBeginMoveObjectResults(result)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:             req.Bucket,
		EncryptedObjectKey: req.EncryptedObjectKey,
		Version:            int32(metabase.DefaultVersion),
		StreamId:           result.StreamID[:],
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(result.EncryptionParameters.CipherSuite),
			BlockSize:   int64(result.EncryptionParameters.BlockSize),
		},
		Placement: int32(newBucketPlacement),
	})
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	response.StreamId = satStreamID
	return response, nil
}

func convertBeginMoveObjectResults(result metabase.BeginMoveObjectResult) (*pb.ObjectBeginMoveResponse, error) {
	keys := make([]*pb.EncryptedKeyAndNonce, len(result.EncryptedKeysNonces))
	for i, key := range result.EncryptedKeysNonces {
		var nonce storj.Nonce
		var err error
		if len(key.EncryptedKeyNonce) != 0 {
			nonce, err = storj.NonceFromBytes(key.EncryptedKeyNonce)
			if err != nil {
				return nil, err
			}
		}

		keys[i] = &pb.EncryptedKeyAndNonce{
			Position: &pb.SegmentPosition{
				PartNumber: int32(key.Position.Part),
				Index:      int32(key.Position.Index),
			},
			EncryptedKey:      key.EncryptedKey,
			EncryptedKeyNonce: nonce,
		}
	}

	// TODO we need this becase of an uplink issue with how we are storing key and nonce
	if result.EncryptedMetadataKey == nil {
		streamMeta := &pb.StreamMeta{}
		err := pb.Unmarshal(result.EncryptedMetadata, streamMeta)
		if err != nil {
			return nil, err
		}
		if streamMeta.LastSegmentMeta != nil {
			result.EncryptedMetadataKey = streamMeta.LastSegmentMeta.EncryptedKey
			result.EncryptedMetadataKeyNonce = streamMeta.LastSegmentMeta.KeyNonce
		}
	}

	var metadataNonce storj.Nonce
	var err error
	if len(result.EncryptedMetadataKeyNonce) != 0 {
		metadataNonce, err = storj.NonceFromBytes(result.EncryptedMetadataKeyNonce)
		if err != nil {
			return nil, err
		}
	}

	return &pb.ObjectBeginMoveResponse{
		EncryptedMetadataKey:      result.EncryptedMetadataKey,
		EncryptedMetadataKeyNonce: metadataNonce,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(result.EncryptionParameters.CipherSuite),
			BlockSize:   int64(result.EncryptionParameters.BlockSize),
		},
		SegmentKeys: keys,
	}, nil
}

// FinishMoveObject accepts new encryption keys for moved object and updates the corresponding object ObjectKey and segments EncryptedKey.
func (endpoint *Endpoint) FinishMoveObject(ctx context.Context, req *pb.ObjectFinishMoveRequest) (resp *pb.ObjectFinishMoveResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Time:          time.Now(),
		Bucket:        req.NewBucket,
		EncryptedPath: req.NewEncryptedObjectKey,
	})
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	err = endpoint.validateBucket(ctx, req.NewBucket)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	exists, err := endpoint.buckets.HasBucket(ctx, req.NewBucket, keyInfo.ProjectID)
	if err != nil {
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	} else if !exists {
		return nil, rpcstatus.Errorf(rpcstatus.NotFound, "target bucket not found: %s", req.NewBucket)
	}

	streamUUID, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	err = endpoint.metabase.FinishMoveObject(ctx, metabase.FinishMoveObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			Version:    metabase.DefaultVersion,
			StreamID:   streamUUID,
		},
		NewSegmentKeys:               protobufkeysToMetabase(req.NewSegmentKeys),
		NewBucket:                    string(req.NewBucket),
		NewEncryptedObjectKey:        req.NewEncryptedObjectKey,
		NewEncryptedMetadataKeyNonce: req.NewEncryptedMetadataKeyNonce[:],
		NewEncryptedMetadataKey:      req.NewEncryptedMetadataKey,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	return &pb.ObjectFinishMoveResponse{}, nil
}

// convertMetabaseErr converts domain errors from metabase to appropriate rpc statuses errors.
func (endpoint *Endpoint) convertMetabaseErr(err error) error {
	switch {
	case storj.ErrObjectNotFound.Has(err):
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	case metabase.ErrSegmentNotFound.Has(err):
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	case metabase.ErrInvalidRequest.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	default:
		endpoint.log.Error("internal", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
}

// protobufkeysToMetabase converts []*pb.EncryptedKeyAndNonce to []metabase.EncryptedKeyAndNonce.
func protobufkeysToMetabase(protoKeys []*pb.EncryptedKeyAndNonce) []metabase.EncryptedKeyAndNonce {
	keys := make([]metabase.EncryptedKeyAndNonce, len(protoKeys))
	for i, key := range protoKeys {
		position := metabase.SegmentPosition{}

		if key.Position != nil {
			position = metabase.SegmentPosition{
				Part:  uint32(key.Position.PartNumber),
				Index: uint32(key.Position.Index),
			}
		}

		keys[i] = metabase.EncryptedKeyAndNonce{
			EncryptedKeyNonce: key.EncryptedKeyNonce.Bytes(),
			EncryptedKey:      key.EncryptedKey,
			Position:          position,
		}
	}

	return keys
}
