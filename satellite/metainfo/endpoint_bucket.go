// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
)

// GetBucket returns a bucket.
func (endpoint *Endpoint) GetBucket(ctx context.Context, req *pb.BucketGetRequest) (resp *pb.BucketGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	bucket, err := endpoint.buckets.GetMinimalBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// override RS to fit satellite settings
	convBucket, err := convertMinimalBucketToProto(bucket, endpoint.defaultRS, endpoint.config.MaxSegmentSize)
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

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionWrite,
		Bucket: req.Name,
		Time:   time.Now(),
	})
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

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
		if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName(), nil); err != nil {
			return nil, err
		}
		return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "bucket already exists")
	}

	project, err := endpoint.projects.Get(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}
	maxBuckets := project.MaxBuckets
	if maxBuckets == nil {
		defaultMaxBuckets := endpoint.config.ProjectLimits.MaxBuckets
		maxBuckets = &defaultMaxBuckets
	}
	bucketCount, err := endpoint.buckets.CountBuckets(ctx, keyInfo.ProjectID)
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
	bucketReq.Placement = project.DefaultPlacement

	bucket, err := endpoint.buckets.CreateBucket(ctx, bucketReq)
	if err != nil {
		endpoint.log.Error("error while creating bucket", zap.String("bucketName", bucketReq.Name), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create bucket")
	}

	// Once we have created the bucket, we can try setting the attribution.
	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName(), project.UserAgent); err != nil {
		return nil, err
	}

	// override RS to fit satellite settings
	convBucket, err := convertMinimalBucketToProto(buckets.MinimalBucket{
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

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

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
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	err = endpoint.validateBucket(ctx, req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	var (
		bucket     buckets.MinimalBucket
		convBucket *pb.Bucket
	)
	if canRead || canList {
		// Info about deleted bucket is returned only if either Read, or List permission is granted.
		bucket, err = endpoint.buckets.GetMinimalBucket(ctx, req.Name, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
			}
			return nil, err
		}

		convBucket, err = convertMinimalBucketToProto(bucket, endpoint.defaultRS, endpoint.config.MaxSegmentSize)
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
		if buckets.ErrBucketNotFound.Has(err) {
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
		if buckets.ErrBucketNotFound.Has(err) {
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
	})

	return deletedObjects, Error.Wrap(err)
}

// ListBuckets returns buckets in a project where the bucket name matches the request cursor.
func (endpoint *Endpoint) ListBuckets(ctx context.Context, req *pb.BucketListRequest) (resp *pb.BucketListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

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
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	allowedBuckets, err := getAllowedBuckets(ctx, req.Header, action)
	if err != nil {
		return nil, err
	}

	listOpts := buckets.ListOptions{
		Cursor:    string(req.Cursor),
		Limit:     int(req.Limit),
		Direction: req.Direction,
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
			UserAgent: item.UserAgent,
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

func convertProtoToBucket(req *pb.BucketCreateRequest, projectID uuid.UUID) (bucket buckets.Bucket, err error) {
	bucketID, err := uuid.New()
	if err != nil {
		return buckets.Bucket{}, err
	}

	return buckets.Bucket{
		ID:        bucketID,
		Name:      string(req.GetName()),
		ProjectID: projectID,
	}, nil
}

func convertMinimalBucketToProto(bucket buckets.MinimalBucket, rs *pb.RedundancyScheme, maxSegmentSize memory.Size) (pbBucket *pb.Bucket, err error) {
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
