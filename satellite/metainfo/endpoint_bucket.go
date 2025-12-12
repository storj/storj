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
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
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
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	bucket, err := endpoint.buckets.GetMinimalBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket metadata")
	}

	// override RS to fit satellite settings
	convBucket, err := convertMinimalBucketToProto(bucket, endpoint.getRSProto(bucket.Placement), endpoint.config.MaxSegmentSize)
	if err != nil {
		return resp, err
	}

	return &pb.BucketGetResponse{
		Bucket: convBucket,
	}, nil
}

// GetBucketLocation responds with the location that the bucket's placement is
// annotated with (if any) and any error encountered.
func (endpoint *Endpoint) GetBucketLocation(ctx context.Context, req *pb.GetBucketLocationRequest) (resp *pb.GetBucketLocationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	p, err := endpoint.buckets.GetBucketPlacement(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get the bucket's placement")
	}

	return &pb.GetBucketLocationResponse{
		Location: []byte(endpoint.overlay.GetLocationFromPlacement(p)),
	}, nil
}

// SetBucketTagging places a set of tags on a bucket.
func (endpoint *Endpoint) SetBucketTagging(ctx context.Context, req *pb.SetBucketTaggingRequest) (resp *pb.SetBucketTaggingResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.BucketTaggingEnabled {
		return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Unimplemented")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionWrite,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if err = endpoint.validateSetBucketTaggingRequestSimple(req); err != nil {
		return nil, err
	}

	tags := make([]buckets.Tag, 0, len(req.Tags))
	for _, protoTag := range req.Tags {
		tags = append(tags, buckets.Tag{
			Key:   string(protoTag.Key),
			Value: string(protoTag.Value),
		})
	}

	err = endpoint.buckets.SetBucketTagging(ctx, req.GetName(), keyInfo.ProjectID, tags)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket tags")
	}

	return &pb.SetBucketTaggingResponse{}, nil
}

// GetBucketTagging returns the set of tags placed on a bucket.
func (endpoint *Endpoint) GetBucketTagging(ctx context.Context, req *pb.GetBucketTaggingRequest) (resp *pb.GetBucketTaggingResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.BucketTaggingEnabled {
		return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Unimplemented")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	bucketNameLen := len(req.Name)
	if bucketNameLen == 0 {
		return nil, rpcstatus.Error(rpcstatus.BucketNameMissing, "A bucket name is required")
	}
	if err := validateBucketNameLength(req.Name); err != nil {
		return nil, rpcstatus.Error(rpcstatus.BucketNameInvalid, err.Error())
	}

	tags, err := endpoint.buckets.GetBucketTagging(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket tags")
	}

	if len(tags) == 0 {
		return nil, rpcstatus.Error(rpcstatus.TagsNotFound, "No tags are set on the bucket")
	}

	pbTags := make([]*pb.BucketTag, 0, len(tags))
	for _, tag := range tags {
		pbTags = append(pbTags, &pb.BucketTag{
			Key:   []byte(tag.Key),
			Value: []byte(tag.Value),
		})
	}

	return &pb.GetBucketTaggingResponse{
		Tags: pbTags,
	}, nil
}

// GetBucketVersioning responds with the versioning state of the bucket and any error encountered.
func (endpoint *Endpoint) GetBucketVersioning(ctx context.Context, req *pb.GetBucketVersioningRequest) (resp *pb.GetBucketVersioningResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	versioning, err := endpoint.buckets.GetBucketVersioningState(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get versioning state for the bucket")
	}

	return &pb.GetBucketVersioningResponse{
		Versioning: int32(versioning),
	}, nil
}

// SetBucketVersioning attempts to enable or disable versioning for a bucket and responds with any error encountered.
func (endpoint *Endpoint) SetBucketVersioning(ctx context.Context, req *pb.SetBucketVersioningRequest) (resp *pb.SetBucketVersioningResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionWrite,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.UseBucketLevelObjectVersioning {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "versioning not allowed")
	}
	if req.Versioning {
		err = endpoint.buckets.EnableBucketVersioning(ctx, req.GetName(), keyInfo.ProjectID)
	} else {
		err = endpoint.buckets.SuspendBucketVersioning(ctx, req.GetName(), keyInfo.ProjectID)
	}
	if err != nil {
		switch {
		case buckets.ErrBucketNotFound.Has(err):
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		case buckets.ErrConflict.Has(err):
			return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		case buckets.ErrLocked.Has(err):
			return nil, rpcstatus.Error(rpcstatus.ObjectLockInvalidBucketState, err.Error())
		case buckets.ErrUnavailable.Has(err):
			return nil, rpcstatus.Error(rpcstatus.Unavailable, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to set versioning state for the bucket")
	}

	return &pb.SetBucketVersioningResponse{}, nil
}

// CreateBucket creates a new bucket.
func (endpoint *Endpoint) CreateBucket(ctx context.Context, req *pb.BucketCreateRequest) (resp *pb.BucketCreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	perms := []VerifyPermission{{
		Action: macaroon.Action{
			Op:     macaroon.ActionWrite,
			Bucket: req.Name,
			Time:   time.Now(),
		},
	}}
	if req.ObjectLockEnabled {
		perms = append(perms, VerifyPermission{
			Action: macaroon.Action{
				Op:     macaroon.ActionPutBucketObjectLockConfiguration,
				Bucket: req.Name,
				Time:   time.Now(),
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, perms...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	err = validateBucketName(req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	project, err := endpoint.projects.Get(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}

	if project.Status != nil && *project.Status == console.ProjectDisabled {
		return nil, rpcstatus.Error(rpcstatus.NotFound, "no such project")
	}

	if req.ObjectLockEnabled && !(endpoint.config.UseBucketLevelObjectVersioning && endpoint.config.ObjectLockEnabled) {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockDisabledForProject, projectNoLockErrMsg)
	}

	bucketReq, err := convertProtoToBucket(req, keyInfo)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	var exists bool
	if endpoint.config.SelfServePlacementSelectEnabled && req.Placement != nil {
		if bucketReq.Placement, exists = endpoint.overlay.GetPlacementConstraintFromName(string(req.Placement)); !exists {
			return nil, rpcstatus.Error(rpcstatus.PlacementInvalidValue, "invalid placement value")
		}
		if err = endpoint.validateSelfServePlacement(ctx, project, bucketReq.Placement); err != nil {
			return nil, err
		}
	} else {
		bucketReq.Placement = project.DefaultPlacement
	}

	// checks if bucket exists before updates it or makes a new entry
	exists, err = endpoint.buckets.HasBucket(ctx, req.GetName(), keyInfo.ProjectID)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to check if bucket exists")
	} else if exists {
		// When the bucket exists, try to set the attribution.
		if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName(), nil, bucketReq.Placement, false, true); err != nil {
			return nil, err
		}
		return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "bucket already exists")
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

	if endpoint.config.UseBucketLevelObjectVersioning {
		if bucketReq.ObjectLock.Enabled {
			bucketReq.Versioning = buckets.VersioningEnabled
		} else {
			defaultVersioning, err := endpoint.projects.GetDefaultVersioning(ctx, keyInfo.ProjectID)
			if err != nil {
				return nil, err
			}
			switch defaultVersioning {
			case console.VersioningUnsupported, console.Unversioned:
				// since bucket level versioning is enabled, projects with
				// unsupported versioning are also allowed to have versioning.
				bucketReq.Versioning = buckets.Unversioned
			case console.VersioningEnabled:
				bucketReq.Versioning = buckets.VersioningEnabled
			}
		}
	}

	if bucketReq.ObjectLock.Enabled && bucketReq.Versioning != buckets.VersioningEnabled {
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, "Object Lock may only be enabled for versioned buckets")
	}

	if attribution, err := endpoint.attributions.Get(ctx, keyInfo.ProjectID, req.GetName()); err == nil {
		if attribution.Placement == nil && bucketReq.Placement != storj.DefaultPlacement {
			return nil, rpcstatus.Errorf(rpcstatus.FailedPrecondition, "bucket %s already attributed to a different placement constraint", bucketReq.Name)
		}
		if attribution.Placement != nil && *attribution.Placement != bucketReq.Placement {
			return nil, rpcstatus.Errorf(rpcstatus.FailedPrecondition, "bucket %s already attributed to a different placement constraint", bucketReq.Name)
		}
	}

	bucket, err := endpoint.buckets.CreateBucket(ctx, bucketReq)
	if err != nil {
		if buckets.ErrBucketAlreadyExists.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "bucket already exists")
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create bucket")
	}

	// Once we have created the bucket, we can try setting the attribution.
	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.GetName(), project.UserAgent, bucket.Placement, true, true); err != nil {
		return nil, err
	}

	// override RS to fit satellite settings
	convBucket, err := convertMinimalBucketToProto(buckets.MinimalBucket{
		Name:      []byte(bucket.Name),
		CreatedAt: bucket.Created,
	}, endpoint.getRSProto(bucket.Placement), endpoint.config.MaxSegmentSize)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create bucket")
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

	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:     macaroon.ActionDelete,
				Bucket: req.Name,
				Time:   now,
			},
		},
		{
			Action: macaroon.Action{
				Op:     macaroon.ActionRead,
				Bucket: req.Name,
				Time:   now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		},
		{
			Action: macaroon.Action{
				Op:     macaroon.ActionList,
				Bucket: req.Name,
				Time:   now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		},
	}

	if req.BypassGovernanceRetention {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:     macaroon.ActionBypassGovernanceRetention,
				Bucket: req.Name,
				Time:   now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitDelete, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	err = validateBucketNameLength(req.Name)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	bucket, err := endpoint.buckets.GetBucket(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket")
	}

	if !keyInfo.CreatedBy.IsZero() {
		member, err := endpoint.projectMembers.GetByMemberIDAndProjectID(ctx, keyInfo.CreatedBy, keyInfo.ProjectID)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
		}

		if member.Role != console.RoleAdmin && bucket.CreatedBy != keyInfo.CreatedBy {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "not enough access to delete this bucket")
		}
	}

	lockEnabled := bucket.ObjectLock.Enabled
	if req.BypassGovernanceRetention {
		lockEnabled = false
	}
	if lockEnabled && req.DeleteAll {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, unauthorizedErrMsg)
	}

	var convBucket *pb.Bucket
	if canRead || canList {
		// Info about deleted bucket is returned only if either Read, or List permission is granted.
		convBucket, err = convertMinimalBucketToProto(
			buckets.MinimalBucket{
				Name:      []byte(bucket.Name),
				Placement: bucket.Placement,
				CreatedBy: bucket.CreatedBy,
				CreatedAt: bucket.Created,
			},
			endpoint.getRSProto(bucket.Placement),
			endpoint.config.MaxSegmentSize,
		)
		if err != nil {
			return nil, err
		}
	}

	err = endpoint.deleteBucket(ctx, bucket)
	if err != nil {
		if !canRead && !canList {
			if !buckets.ErrBucketNotFound.Has(err) && !ErrBucketNotEmpty.Has(err) {
				// We don't want to return an internal error if it doesn't have read and list permissions
				// for not giving any little chance to find out that a bucket with a specific name and with
				// data exists, but we want to log it about it.
				endpoint.log.Error("internal", zap.Error(err))
			}

			// No error info is returned if neither Read, nor List permission is granted.
			return &pb.BucketDeleteResponse{}, nil
		}
		if ErrBucketNotEmpty.Has(err) {
			// List permission is required to delete all objects in a bucket.
			if !req.GetDeleteAll() || !canList {
				return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
			}

			transmitEvent := endpoint.bucketEventing.Enabled(bucket.ProjectID, bucket.Name)
			deletedObjCount, err := endpoint.deleteBucketNotEmpty(ctx, bucket, transmitEvent)
			if err != nil {
				return nil, err
			}

			return &pb.BucketDeleteResponse{Bucket: convBucket, DeletedObjectsCount: deletedObjCount}, nil
		}
		if buckets.ErrBucketNotFound.Has(err) {
			return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to delete bucket")
	}

	return &pb.BucketDeleteResponse{Bucket: convBucket}, nil
}

// deleteBucket deletes a bucket from the bucekts db.
func (endpoint *Endpoint) deleteBucket(ctx context.Context, bucket buckets.Bucket) (err error) {
	defer mon.Task()(&ctx)(&err)

	nameBytes := []byte(bucket.Name)

	empty, err := endpoint.isBucketEmpty(ctx, bucket.ProjectID, nameBytes)
	if err != nil {
		return err
	}
	if !empty {
		return ErrBucketNotEmpty.New("")
	}

	err = endpoint.buckets.DeleteBucket(ctx, nameBytes, bucket.ProjectID)
	if err != nil {
		return err
	}

	if err = endpoint.ensureAttributionOnBucketDelete(ctx, bucket); err != nil {
		endpoint.log.Error("failed to ensure attribution on bucket delete",
			zap.Error(err),
			zap.String("bucket", bucket.Name),
			zap.String("project_id", bucket.ProjectID.String()),
		)
	}

	return nil
}

// isBucketEmpty returns whether bucket is empty.
func (endpoint *Endpoint) isBucketEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) (bool, error) {
	empty, err := endpoint.metabase.BucketEmpty(ctx, metabase.BucketEmpty{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucketName),
	})
	return empty, Error.Wrap(err)
}

// deleteBucketNotEmpty deletes all objects from bucket and deletes this bucket.
// On success, it returns only the number of deleted objects.
func (endpoint *Endpoint) deleteBucketNotEmpty(ctx context.Context, bucket buckets.Bucket, transmitEvent bool) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[bucket.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

	deletedCount, err := endpoint.metabase.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
		Bucket: metabase.BucketLocation{
			ProjectID:  bucket.ProjectID,
			BucketName: metabase.BucketName(bucket.Name),
		},
		BatchSize:      endpoint.config.TestingDeleteBucketBatchSize,
		MaxCommitDelay: maxCommitDelay,
		TransmitEvent:  transmitEvent,
	})
	if err != nil {
		return 0, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	err = endpoint.deleteBucket(ctx, bucket)
	if err != nil {
		if ErrBucketNotEmpty.Has(err) {
			return deletedCount, rpcstatus.Error(rpcstatus.FailedPrecondition, "cannot delete the bucket because it's being used by another process")
		}
		if buckets.ErrBucketNotFound.Has(err) {
			return 0, nil
		}
		return deletedCount, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	return deletedCount, nil
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
	keyInfo, err := endpoint.validateAuth(ctx, req.Header, action, console.RateLimitList)
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

// GetBucketObjectLockConfiguration returns a bucket's Object Lock configuration.
func (endpoint *Endpoint) GetBucketObjectLockConfiguration(ctx context.Context, req *pb.GetBucketObjectLockConfigurationRequest) (resp *pb.GetBucketObjectLockConfigurationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionGetBucketObjectLockConfiguration,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	settings, err := endpoint.buckets.GetBucketObjectLockSettings(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket's Object Lock configuration")
	}

	if !settings.Enabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, bucketNoLockErrMsg)
	}

	configuration := pb.ObjectLockConfiguration{
		Enabled: true,
	}

	if settings.DefaultRetentionMode != storj.NoRetention {
		defaultRetention := pb.DefaultRetention{
			Mode: pb.Retention_Mode(settings.DefaultRetentionMode),
		}
		switch {
		case settings.DefaultRetentionDays != 0:
			defaultRetention.Duration = &pb.DefaultRetention_Days{Days: int32(settings.DefaultRetentionDays)}
		case settings.DefaultRetentionYears != 0:
			defaultRetention.Duration = &pb.DefaultRetention_Years{Years: int32(settings.DefaultRetentionYears)}
		}
		configuration.DefaultRetention = &defaultRetention
	}

	return &pb.GetBucketObjectLockConfigurationResponse{
		Configuration: &configuration,
	}, nil
}

// SetBucketObjectLockConfiguration updates a bucket's Object Lock configuration.
func (endpoint *Endpoint) SetBucketObjectLockConfiguration(ctx context.Context, req *pb.SetBucketObjectLockConfigurationRequest) (resp *pb.SetBucketObjectLockConfigurationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:     macaroon.ActionPutBucketObjectLockConfiguration,
		Bucket: req.Name,
		Time:   time.Now(),
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	bucket, err := endpoint.buckets.GetBucket(ctx, req.Name, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Name)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket's Object Lock configuration")
	}

	if bucket.Versioning != buckets.VersioningEnabled {
		return nil, rpcstatus.Errorf(rpcstatus.ObjectLockInvalidBucketState, "cannot specify Object Lock configuration for a bucket without Versioning enabled")
	}

	updateParams, err := convertProtobufObjectLockConfig(req.Configuration)
	if err != nil {
		return nil, err
	}

	updateParams.ProjectID = keyInfo.ProjectID
	updateParams.Name = string(req.Name)

	_, err = endpoint.buckets.UpdateBucketObjectLockSettings(ctx, updateParams)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Name)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to set bucket's Object Lock configuration")
	}

	return &pb.SetBucketObjectLockConfigurationResponse{}, nil
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

func convertProtoToBucket(req *pb.BucketCreateRequest, keyInfo *console.APIKeyInfo) (bucket buckets.Bucket, err error) {
	bucketID, err := uuid.New()
	if err != nil {
		return buckets.Bucket{}, err
	}

	return buckets.Bucket{
		ID:        bucketID,
		Name:      string(req.GetName()),
		ProjectID: keyInfo.ProjectID,
		CreatedBy: keyInfo.CreatedBy,
		ObjectLock: buckets.ObjectLockSettings{
			Enabled: req.GetObjectLockEnabled(),
		},
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
