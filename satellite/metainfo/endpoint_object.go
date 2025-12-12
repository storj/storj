// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
)

const (
	projectNoLockErrMsg      = "Object Lock is not enabled for this project"
	objectLockDisabledErrMsg = "Object Lock feature is not enabled"
	bucketNoLockErrMsg       = "Object Lock is not enabled for this bucket"
	methodNotAllowedErrMsg   = "method not allowed"
	objectInvalidStateErrMsg = "The operation is not permitted for this object"
)

// BeginObject begins object.
func (endpoint *Endpoint) BeginObject(ctx context.Context, req *pb.ObjectBeginRequest) (resp *pb.ObjectBeginResponse, err error) {
	return endpoint.beginObject(ctx, req, true)
}

func (endpoint *Endpoint) beginObject(ctx context.Context, req *pb.ObjectBeginRequest, multipartUpload bool) (resp *pb.ObjectBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()

	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			Optional: true,
		},
	}

	retention := protobufRetentionToMetabase(req.Retention)

	if retention.Enabled() {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectRetention,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		})
	}
	if req.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if (retention.Enabled() || req.LegalHold) && !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	maxObjectTTL, err := endpoint.getMaxObjectTTL(ctx, req.Header)
	if err != nil {
		return nil, err
	}

	if !req.ExpiresAt.IsZero() {
		if req.ExpiresAt.Before(now) {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "invalid expiration time, cannot be in the past")
		}
		if maxObjectTTL != nil && req.ExpiresAt.After(now.Add(*maxObjectTTL)) {
			return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid expiration time, cannot be longer than %v", maxObjectTTL)
		}
	}

	var expiresAt time.Time
	if !req.ExpiresAt.IsZero() {
		expiresAt = req.ExpiresAt
	} else if maxObjectTTL != nil {
		ttl := now.Add(*maxObjectTTL)
		expiresAt = ttl
	}

	if retention.Enabled() || req.LegalHold {
		switch {
		case maxObjectTTL != nil:
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument,
				"cannot specify Object Lock settings when using an API key that enforces an object expiration time")
		case !req.ExpiresAt.IsZero():
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument,
				"cannot specify Object Lock settings and an object expiration time")
		}
	}

	objectKeyLength := len(req.EncryptedObjectKey)
	if objectKeyLength > endpoint.config.MaxEncryptedObjectKeyLength {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "key length is too big, got %v, maximum allowed is %v", objectKeyLength, endpoint.config.MaxEncryptedObjectKeyLength)
	}

	err = endpoint.checkUploadLimits(ctx, keyInfo)
	if err != nil {
		return nil, err
	}

	if err := endpoint.checkObjectUploadRate(ctx, keyInfo.ProjectPublicID, req.Bucket, req.EncryptedObjectKey); err != nil {
		return nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
	}

	if err = validateBucketObjectLockStatus(bucket, retention, req.LegalHold); err != nil {
		return nil, err
	}

	if bucket.ObjectLock.Enabled && bucket.ObjectLock.DefaultRetentionMode != storj.NoRetention && !retention.Enabled() {
		err = validateObjectRetentionWithTTL(maxObjectTTL, req.ExpiresAt)
		if err != nil {
			return nil, err
		}

		retention = useDefaultBucketRetention(bucket.ObjectLock, now)
	}

	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.Bucket, nil, bucket.Placement, false, false); err != nil {
		return nil, err
	}

	streamID, err := uuid.New()
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create stream id")
	}

	// TODO this will work only with newest uplink
	// figure out what to do with this
	encryptionParameters := storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(req.EncryptionParameters.CipherSuite),
		BlockSize:   int32(req.EncryptionParameters.BlockSize), // TODO check conversion
	}

	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[keyInfo.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

	var object metabase.Object
	if !multipartUpload && endpoint.config.isNoPendingObjectUploadEnabled(keyInfo.ProjectID) {
		object.CreatedAt = time.Now()
	} else {
		objectStream := metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			StreamID:   streamID,
			Version:    metabase.NextVersion,
		}

		encryptedUserData := metabase.EncryptedUserData{
			EncryptedMetadata:             req.EncryptedMetadata,
			EncryptedMetadataEncryptedKey: req.EncryptedMetadataEncryptedKey,
			EncryptedMetadataNonce:        nonceBytes(req.EncryptedMetadataNonce),
			EncryptedETag:                 req.EncryptedEtag,
		}
		if _, ok := endpoint.config.TestingAlternativeBeginObjectProjects[keyInfo.ProjectID]; ok || endpoint.config.TestingAlternativeBeginObject {
			opts := metabase.BeginObjectExactVersion{
				ObjectStream: objectStream,
				Encryption:   encryptionParameters,

				EncryptedUserData: encryptedUserData,

				Retention: retention,
				LegalHold: req.LegalHold,

				MaxCommitDelay: maxCommitDelay,
			}
			if !expiresAt.IsZero() {
				opts.ExpiresAt = &expiresAt
			}

			const maxRetries = 5

			for i := range maxRetries {
				rng := rand.New(rand.NewSource(time.Now().UnixNano()))
				opts.Version = metabase.Version(-1 * rng.Int63())

				object, err = endpoint.metabase.BeginObjectExactVersion(ctx, opts)
				if err != nil {
					if metabase.ErrObjectAlreadyExists.Has(err) && i < maxRetries-1 {
						continue
					}
					return nil, endpoint.ConvertMetabaseErr(err)
				}
				break
			}
		} else {
			opts := metabase.BeginObjectNextVersion{
				ObjectStream: objectStream,
				Encryption:   encryptionParameters,

				EncryptedUserData: encryptedUserData,

				Retention: retention,
				LegalHold: req.LegalHold,

				MaxCommitDelay: maxCommitDelay,
			}
			if !expiresAt.IsZero() {
				opts.ExpiresAt = &expiresAt
			}

			object, err = endpoint.metabase.BeginObjectNextVersion(ctx, opts)
			if err != nil {
				return nil, endpoint.ConvertMetabaseErr(err)
			}
		}
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:               req.Bucket,
		EncryptedObjectKey:   req.EncryptedObjectKey,
		Version:              int64(object.Version),
		CreationDate:         object.CreatedAt,
		ExpirationDate:       expiresAt, // TODO make ExpirationDate nullable
		StreamId:             streamID.Bytes(),
		MultipartObject:      multipartUpload || !endpoint.config.isNoPendingObjectUploadEnabled(keyInfo.ProjectID),
		EncryptionParameters: req.EncryptionParameters,
		Placement:            int32(bucket.Placement),
		Versioned:            bucket.Versioning == buckets.VersioningEnabled,
		Retention:            req.Retention,
		LegalHold:            req.LegalHold,
	})
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create stream id")
	}

	endpoint.log.Debug("Object Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "object"))
	mon.Meter("req_put_object", monkit.NewSeriesTag("multipart", strconv.FormatBool(multipartUpload))).Mark(1)

	return &pb.ObjectBeginResponse{
		Bucket:             req.Bucket,
		EncryptedObjectKey: req.EncryptedObjectKey,
		StreamId:           satStreamID,
		RedundancyScheme:   endpoint.getRSProto(bucket.Placement),
	}, nil
}

func (endpoint *Endpoint) getMaxObjectTTL(ctx context.Context, header *pb.RequestHeader) (_ *time.Duration, err error) {
	key, err := getAPIKey(ctx, header)
	if err != nil {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "Invalid API credentials: %v", err)
	}

	ttl, err := key.GetMaxObjectTTL(ctx)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get max object TTL")
	}

	if ttl != nil && *ttl <= 0 {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid MaxObjectTTL in API key: %v", ttl)
	}

	return ttl, nil
}

// CommitObject commits an object when all its segments have already been committed.
func (endpoint *Endpoint) CommitObject(ctx context.Context, req *pb.ObjectCommitRequest) (resp *pb.ObjectCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	now := time.Now()
	var allowDelete, canGetRetention, canGetLegalHold bool
	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPutNoError,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        streamID.Bucket,
				EncryptedPath: streamID.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        streamID.Bucket,
				EncryptedPath: streamID.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &allowDelete,
			Optional:        true,
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectRetention,
				Bucket:        streamID.Bucket,
				EncryptedPath: streamID.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetRetention,
			Optional:        true,
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectLegalHold,
				Bucket:        streamID.Bucket,
				EncryptedPath: streamID.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetLegalHold,
			Optional:        true,
		},
	)
	if err != nil {
		return nil, err
	}
	var committedObject *metabase.Object
	defer func() {
		var tags []eventkit.Tag
		if committedObject != nil {
			tags = []eventkit.Tag{
				eventkit.Bool("expires", committedObject.ExpiresAt != nil),
				eventkit.Int64("segment_count", int64(committedObject.SegmentCount)),
				eventkit.Int64("total_plain_size", committedObject.TotalPlainSize),
				eventkit.Int64("total_encrypted_size", committedObject.TotalEncryptedSize),
				eventkit.Int64("fixed_segment_size", int64(committedObject.FixedSegmentSize)),
			}
		}
		endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req), tags...)
	}()

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	// for old uplinks get Encryption from StreamMeta
	streamMeta := &pb.StreamMeta{}
	encryption := storj.EncryptionParameters{}
	err = pb.Unmarshal(req.EncryptedMetadata, streamMeta)
	if err != nil {
		// TODO: what if this is an error we don't expect?
	} else {
		encryption.CipherSuite = storj.CipherSuite(streamMeta.EncryptionType)
		encryption.BlockSize = streamMeta.EncryptionBlockSize
	}
	if encryption.IsZero() {
		encryption = storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(streamID.EncryptionParameters.CipherSuite),
			BlockSize:   int32(streamID.EncryptionParameters.BlockSize), // TODO unsafe conversion
		}
	}

	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[keyInfo.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	request := metabase.CommitObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(streamID.Version),
		},
		Encryption: encryption,
		ExpiresAt:  expiresAt,

		Retention: protobufRetentionToMetabase(streamID.Retention),
		LegalHold: streamID.LegalHold,

		DisallowDelete: !allowDelete,

		Versioned: streamID.Versioned,

		MaxCommitDelay: maxCommitDelay,

		IfNoneMatch: req.IfNoneMatch,

		TransmitEvent: endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(streamID.Bucket)),

		SkipPendingObject: !streamID.MultipartObject && endpoint.config.isNoPendingObjectUploadEnabled(keyInfo.ProjectID),
	}
	// uplink can send empty metadata with not empty key/nonce
	// we need to fix it on uplink side but that part will be
	// needed for backward compatibility
	if len(req.EncryptedMetadata) != 0 {
		request.OverrideEncryptedMetadata = true
		request.EncryptedMetadata = req.EncryptedMetadata
		request.EncryptedETag = req.EncryptedEtag
		request.EncryptedMetadataNonce = nonceBytes(req.EncryptedMetadataNonce)
		request.EncryptedMetadataEncryptedKey = req.EncryptedMetadataEncryptedKey

		// older uplinks might send EncryptedMetadata directly with request but
		// key/nonce will be part of StreamMeta
		if req.EncryptedMetadataNonce.IsZero() && len(req.EncryptedMetadataEncryptedKey) == 0 &&
			streamMeta.LastSegmentMeta != nil {
			request.EncryptedMetadataNonce = streamMeta.LastSegmentMeta.KeyNonce
			request.EncryptedMetadataEncryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
		}
	}

	if err := endpoint.checkEncryptedMetadataSize(request.EncryptedUserData); err != nil {
		return nil, err
	}

	object, err := endpoint.metabase.CommitObject(ctx, request)
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}
	committedObject = &object

	pbObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}
	if !canGetRetention {
		pbObject.Retention = nil
	}
	if !canGetLegalHold {
		pbObject.LegalHold = nil
	}

	mon.Meter("req_commit_object").Mark(1)

	return &pb.ObjectCommitResponse{
		Object: pbObject,
	}, nil
}

// CommitInlineObject commits a full inline object.
func (endpoint *Endpoint) CommitInlineObject(ctx context.Context, beginObjectReq *pb.ObjectBeginRequest, makeInlineSegReq *pb.SegmentMakeInlineRequest, commitObjectReq *pb.ObjectCommitRequest) (
	_ *pb.ObjectBeginResponse,
	_ *pb.SegmentMakeInlineResponse,
	_ *pb.ObjectCommitResponse, err error,
) {
	defer mon.Task()(&ctx)(&err)

	if err = validateRequestSimple(beginObjectReq); err != nil {
		return nil, nil, nil, err
	}

	now := time.Now()
	var allowDelete, canGetRetention, canGetLegalHold bool
	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &allowDelete,
			Optional:        true,
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectRetention,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetRetention,
			Optional:        true,
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectLegalHold,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetLegalHold,
			Optional:        true,
		},
	}

	retention := protobufRetentionToMetabase(beginObjectReq.Retention)
	if retention.Enabled() {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectRetention,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
		})
	}

	if beginObjectReq.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        beginObjectReq.Bucket,
				EncryptedPath: beginObjectReq.EncryptedObjectKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, beginObjectReq.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, nil, nil, err
	}

	// TODO does it make sense to track each request separately
	//
	endpoint.usageTracking(keyInfo, beginObjectReq.Header, fmt.Sprintf("%T", beginObjectReq))
	endpoint.usageTracking(keyInfo, makeInlineSegReq.Header, fmt.Sprintf("%T", makeInlineSegReq))
	endpoint.usageTracking(keyInfo, commitObjectReq.Header, fmt.Sprintf("%T", commitObjectReq))

	if (retention.Enabled() || beginObjectReq.LegalHold) && !endpoint.config.ObjectLockEnabled {
		return nil, nil, nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	maxObjectTTL, err := endpoint.getMaxObjectTTL(ctx, beginObjectReq.Header)
	if err != nil {
		return nil, nil, nil, err
	}

	// TODO unify validation part between other methods to avoid duplication

	if !beginObjectReq.ExpiresAt.IsZero() {
		if beginObjectReq.ExpiresAt.Before(now) {
			return nil, nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, "invalid expiration time, cannot be in the past")
		}
		if maxObjectTTL != nil && beginObjectReq.ExpiresAt.After(now.Add(*maxObjectTTL)) {
			return nil, nil, nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid expiration time, cannot be longer than %v", maxObjectTTL)
		}
	}

	var expiresAt *time.Time
	if !beginObjectReq.ExpiresAt.IsZero() {
		expiresAt = &beginObjectReq.ExpiresAt
	} else if maxObjectTTL != nil {
		ttl := now.Add(*maxObjectTTL)
		expiresAt = &ttl
	}

	if retention.Enabled() || beginObjectReq.LegalHold {
		switch {
		case maxObjectTTL != nil:
			return nil, nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument,
				"cannot specify Object Lock settings when using an API key that enforces an object expiration time")
		case !beginObjectReq.ExpiresAt.IsZero():
			return nil, nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument,
				"cannot specify Object Lock settings and an object expiration time")
		}
	}

	objectKeyLength := len(beginObjectReq.EncryptedObjectKey)
	if objectKeyLength > endpoint.config.MaxEncryptedObjectKeyLength {
		return nil, nil, nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "key length is too big, got %v, maximum allowed is %v", objectKeyLength, endpoint.config.MaxEncryptedObjectKeyLength)
	}

	err = endpoint.checkUploadLimits(ctx, keyInfo)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := endpoint.checkObjectUploadRate(ctx, keyInfo.ProjectPublicID, beginObjectReq.Bucket, beginObjectReq.EncryptedObjectKey); err != nil {
		return nil, nil, nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, beginObjectReq.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, nil, nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", beginObjectReq.Bucket)
		}
		return nil, nil, nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
	}

	if err = validateBucketObjectLockStatus(bucket, retention, beginObjectReq.LegalHold); err != nil {
		return nil, nil, nil, err
	}

	if bucket.ObjectLock.Enabled && bucket.ObjectLock.DefaultRetentionMode != storj.NoRetention && !retention.Enabled() {
		err = validateObjectRetentionWithTTL(maxObjectTTL, beginObjectReq.ExpiresAt)
		if err != nil {
			return nil, nil, nil, err
		}

		retention = useDefaultBucketRetention(bucket.ObjectLock, now)
	}

	if err := endpoint.ensureAttribution(ctx, beginObjectReq.Header, keyInfo, beginObjectReq.Bucket, nil, bucket.Placement, false, false); err != nil {
		return nil, nil, nil, err
	}

	if makeInlineSegReq.Position.Index < 0 {
		return nil, nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	inlineUsed := int64(len(makeInlineSegReq.EncryptedInlineData))
	if inlineUsed > endpoint.encInlineSegmentSize {
		return nil, nil, nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "inline segment size cannot be larger than %s", endpoint.config.MaxInlineSegmentSize)
	}

	var committedObject *metabase.Object
	defer func() {
		var tags []eventkit.Tag
		if committedObject != nil {
			tags = []eventkit.Tag{
				eventkit.Bool("expires", committedObject.ExpiresAt != nil),
				eventkit.Int64("segment_count", int64(committedObject.SegmentCount)),
				eventkit.Int64("total_plain_size", committedObject.TotalPlainSize),
				eventkit.Int64("total_encrypted_size", committedObject.TotalEncryptedSize),
			}
		}
		endpoint.usageTracking(keyInfo, commitObjectReq.Header, fmt.Sprintf("%T", commitObjectReq), tags...)
	}()

	streamID, err := uuid.New()
	if err != nil {
		return nil, nil, nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create stream id")
	}

	encryptionParameters := storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(beginObjectReq.EncryptionParameters.CipherSuite),
		BlockSize:   int32(beginObjectReq.EncryptionParameters.BlockSize), // TODO check conversion
	}

	objectStream := metabase.ObjectStream{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(bucket.Name),
		ObjectKey:  metabase.ObjectKey(beginObjectReq.EncryptedObjectKey),
		StreamID:   streamID,
	}

	object, err := endpoint.metabase.CommitInlineObject(ctx, metabase.CommitInlineObject{
		ObjectStream: objectStream,
		CommitInlineSegment: metabase.CommitInlineSegment{
			ObjectStream: objectStream,

			ExpiresAt:         expiresAt,
			EncryptedKey:      makeInlineSegReq.EncryptedKey,
			EncryptedKeyNonce: makeInlineSegReq.EncryptedKeyNonce.Bytes(),
			Position: metabase.SegmentPosition{
				Part:  uint32(makeInlineSegReq.Position.PartNumber),
				Index: uint32(makeInlineSegReq.Position.Index),
			},
			PlainSize:  int32(makeInlineSegReq.PlainSize), // TODO incompatible types int32 vs int64
			InlineData: makeInlineSegReq.EncryptedInlineData,

			// don't set EncryptedETag as this method won't be used with multipart upload
		},

		ExpiresAt:  expiresAt,
		Encryption: encryptionParameters,

		EncryptedUserData: metabase.EncryptedUserData{
			EncryptedMetadata:             commitObjectReq.EncryptedMetadata,
			EncryptedMetadataEncryptedKey: commitObjectReq.EncryptedMetadataEncryptedKey,
			EncryptedMetadataNonce:        nonceBytes(commitObjectReq.EncryptedMetadataNonce),
			EncryptedETag:                 commitObjectReq.EncryptedEtag,
		},

		Retention: retention,
		LegalHold: beginObjectReq.LegalHold,

		DisallowDelete: !allowDelete,

		Versioned: bucket.Versioning == buckets.VersioningEnabled,

		IfNoneMatch: commitObjectReq.IfNoneMatch,

		TransmitEvent: endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(beginObjectReq.Bucket)),
	})
	if err != nil {
		return nil, nil, nil, endpoint.ConvertMetabaseErr(err)
	}

	err = endpoint.orders.UpdatePutInlineOrder(ctx, metabase.BucketLocation{
		ProjectID: keyInfo.ProjectID, BucketName: metabase.BucketName(beginObjectReq.Bucket),
	}, inlineUsed)
	if err != nil {
		return nil, nil, nil, endpoint.ConvertKnownErrWithMessage(err, "unable to update PUT inline order")
	}

	endpoint.addSegmentToUploadLimits(ctx, keyInfo, inlineUsed)

	pbObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		return nil, nil, nil, endpoint.ConvertKnownErrWithMessage(err, "unable to convert metabase object")
	}
	if !canGetRetention {
		pbObject.Retention = nil
	}
	if !canGetLegalHold {
		pbObject.LegalHold = nil
	}

	endpoint.log.Debug("Object Inline Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "object"))
	mon.Meter("req_put_inline_object").Mark(1)

	return &pb.ObjectBeginResponse{
			StreamId: storj.StreamID{1}, // return dummy stream id as it won't be really used later
		}, &pb.SegmentMakeInlineResponse{}, &pb.ObjectCommitResponse{
			Object: pbObject,
		}, nil
}

// GetObject gets single object metadata.
func (endpoint *Endpoint) GetObject(ctx context.Context, req *pb.ObjectGetRequest) (resp *pb.ObjectGetResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	var canGetRetention bool
	var canGetLegalHold bool

	now := time.Now()
	keyInfo, err := endpoint.ValidateAuthAny(ctx, req.Header, console.RateLimitHead,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionList,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectRetention,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetRetention,
			Optional:        true,
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectLegalHold,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetLegalHold,
			Optional:        true,
		},
	)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	objectLocation := metabase.ObjectLocation{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(req.Bucket),
		ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
	}

	if endpoint.config.DownloadLimiter.Enabled {
		if !endpoint.singleObjectDownloadLimitCache.Allow(time.Now(),
			bytes.Join([][]byte{keyInfo.ProjectID[:], req.Bucket, req.EncryptedObjectKey}, []byte{'/'})) {
			return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
		}
	}

	var mbObject metabase.Object
	if len(req.ObjectVersion) == 0 {
		mbObject, err = endpoint.metabase.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
			ObjectLocation: objectLocation,
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		mbObject, err = endpoint.metabase.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: objectLocation,
			Version:        sv.Version(),
		})
	}
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	if mbObject.Status.IsDeleteMarker() {
		return nil, rpcstatus.Error(rpcstatus.MethodNotAllowed, methodNotAllowedErrMsg)
	}

	{
		tags := []eventkit.Tag{
			eventkit.Bool("expires", mbObject.ExpiresAt != nil),
			eventkit.Int64("segment_count", int64(mbObject.SegmentCount)),
			eventkit.Int64("total_plain_size", mbObject.TotalPlainSize),
			eventkit.Int64("total_encrypted_size", mbObject.TotalEncryptedSize),
			eventkit.Int64("fixed_segment_size", int64(mbObject.FixedSegmentSize)),
		}
		endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req), tags...)
	}

	object, err := endpoint.objectToProto(ctx, mbObject)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	// TODO this code is triggered only by very old uplink library (<1.4.2) and we will remove it eventually.
	// note: for non-default RS schema it
	if !req.RedundancySchemePerSegment && mbObject.SegmentCount > 0 {
		segment, err := endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: mbObject.StreamID,
			Position: metabase.SegmentPosition{
				Index: 0,
			},
		})
		if err != nil {
			// add user agent to log entry to figure out tool that is using old uplink
			userAgent := "unknown"
			if req.Header != nil && len(req.Header.UserAgent) != 0 {
				userAgent = string(req.Header.UserAgent)
			}

			// don't fail because its possible that its multipart object
			endpoint.log.Warn("unable to get segment metadata to get object redundancy",
				zap.Stringer("StreamID", mbObject.StreamID),
				zap.Stringer("ProjectID", keyInfo.ProjectID),
				zap.String("User Agent", userAgent),
				zap.Error(err),
			)
		} else {
			object.RedundancyScheme = &pb.RedundancyScheme{
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

	if !canGetRetention {
		object.Retention = nil
	}
	if !canGetLegalHold {
		object.LegalHold = nil
	}

	endpoint.log.Debug("Object Get", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "object"))
	mon.Meter("req_get_object").Mark(1)

	return &pb.ObjectGetResponse{Object: object}, nil
}

// DownloadObject gets object information, creates a download for segments and lists the object segments.
func (endpoint *Endpoint) DownloadObject(ctx context.Context, req *pb.ObjectDownloadRequest) (resp *pb.ObjectDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if ctx.Err() != nil {
		return nil, rpcstatus.Error(rpcstatus.Canceled, "client has closed the connection")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	peer, trusted, err := endpoint.uplinkPeer(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, err
	}

	var canGetRetention bool
	var canGetLegalHold bool

	now := time.Now()
	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitGet,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectRetention,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetRetention,
			Optional:        true,
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectLegalHold,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetLegalHold,
			Optional:        true,
		},
	)
	if err != nil {
		return nil, err
	}

	if err := validateServerSideCopyFlag(req.ServerSideCopy, trusted); err != nil {
		return nil, err
	}

	if err := endpoint.checkDownloadLimits(ctx, keyInfo); err != nil {
		return nil, err
	}
	if endpoint.config.DownloadLimiter.Enabled {
		if !endpoint.singleObjectDownloadLimitCache.Allow(endpoint.rateLimiterTime(),
			bytes.Join([][]byte{keyInfo.ProjectID[:], req.Bucket, req.EncryptedObjectKey}, []byte{'/'})) {
			return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
		}
	}

	var object metabase.Object
	if len(req.ObjectVersion) == 0 {
		object, err = endpoint.metabase.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(req.Bucket),
				ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			},
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		object, err = endpoint.metabase.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(req.Bucket),
				ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			},
			Version: sv.Version(),
		})
	}
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	if object.Status.IsDeleteMarker() {
		return nil, rpcstatus.Error(rpcstatus.MethodNotAllowed, methodNotAllowedErrMsg)
	}

	// get the range segments
	streamRange, err := calculateStreamRange(object, req.Range)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	{
		tags := []eventkit.Tag{
			eventkit.Bool("expires", object.ExpiresAt != nil),
			eventkit.Int64("segment_count", int64(object.SegmentCount)),
			eventkit.Int64("total_plain_size", object.TotalPlainSize),
			eventkit.Int64("total_encrypted_size", object.TotalEncryptedSize),
			eventkit.Int64("fixed_segment_size", int64(object.FixedSegmentSize)),
		}
		if streamRange != nil {
			tags = append(tags,
				eventkit.Int64("range_start", streamRange.PlainStart),
				eventkit.Int64("range_end", streamRange.PlainLimit))
		}
		endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req), tags...)
	}

	segments, err := endpoint.metabase.ListSegments(ctx, metabase.ListSegments{
		ProjectID: keyInfo.ProjectID,
		StreamID:  object.StreamID,
		Range:     streamRange,
		Limit:     int(req.Limit),
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	// get the download response for the first segment
	downloadSegments, err := func() ([]*pb.SegmentDownloadResponse, error) {
		if len(segments.Segments) == 0 {
			return nil, nil
		}
		if object.IsMigrated() && streamRange != nil && streamRange.PlainStart > 0 {
			return nil, nil
		}

		segment := segments.Segments[0]
		downloadSizes := endpoint.calculateDownloadSizes(streamRange, segment, object.Encryption)

		// Update the current bandwidth cache value incrementing the SegmentSize.
		err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfoToLimits(keyInfo), downloadSizes.encryptedSize)
		if err != nil {
			// don't log errors if it was user cancellation
			if errors.Is(ctx.Err(), context.Canceled) {
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
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get encryption key nonce from metadata")
		}

		if segment.Inline() {
			// skip egress tracking for server-side copy operation
			if !req.ServerSideCopy {
				if err := endpoint.orders.UpdateGetInlineOrder(ctx, object.Location().Bucket(), downloadSizes.plainSize); err != nil {
					return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to update GET inline order")
				}
			}

			// TODO we may think about fallback to encrypted size
			// as plain size may be empty for old objects
			downloaded := segment.PlainSize
			if streamRange != nil {
				downloaded = int32(streamRange.PlainLimit)
			}
			endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, download, int(downloaded))

			endpoint.log.Debug("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
			mon.Meter("req_get_inline").Mark(1)
			mon.Counter("req_get_inline_bytes").Inc(int64(len(segment.InlineData)))

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

		var (
			limits []*pb.AddressedOrderLimit
			// Only needed for lite requests.
			encodedSegmentID []byte
		)

		bucketLocation := object.Location().Bucket()
		if req.ServerSideCopy {
			// skip egress tracking for server-side copy operation, empty bucket location will
			// be skipped while orders settlement
			bucketLocation = metabase.BucketLocation{}
		}

		var privateKey storj.PiecePrivateKey
		if req.LiteRequest {
			// Lite responses don't have signed orders limits and the piece ID because the lite request are
			// only accepted by trusted clients. Clients will add the signature and reconstruct the piece ID,
			// however, they need the root piece ID, which is provided through the segment ID.
			// Because they don't need anything else from the segment ID, we only set the root piece ID.
			segmentID := internalpb.SegmentID{
				RootPieceId: segment.RootPieceID,
			}
			encodedSegmentID, err = endpoint.packSegmentID(ctx, &segmentID)
			if err != nil {
				return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create segment id")
			}

			limits, privateKey, err = endpoint.orders.CreateLiteGetOrderLimits(ctx, peer, bucketLocation, segment, req.GetDesiredNodes(), downloadSizes.orderLimit)
		} else {
			limits, privateKey, err = endpoint.orders.CreateGetOrderLimits(ctx, peer, bucketLocation, segment, req.GetDesiredNodes(), downloadSizes.orderLimit)
		}
		if err != nil {
			if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
				endpoint.log.Error("Unable to create order limits.",
					zap.Stringer("Project ID", keyInfo.ProjectID),
					zap.Stringer("API Key ID", keyInfo.ID),
					zap.Error(err),
				)
			}
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create order limits")
		}

		// TODO we may think about fallback to encrypted size
		// as plain size may be empty for old objects
		downloaded := segment.PlainSize
		if streamRange != nil {
			downloaded = int32(streamRange.PlainLimit)
		}
		endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, download, int(downloaded))

		endpoint.log.Debug("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
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

			SegmentId: encodedSegmentID,
		}}, nil
	}()
	if err != nil {
		return nil, err
	}

	// convert to response
	protoObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to convert object to proto")
	}
	if !canGetRetention {
		protoObject.Retention = nil
	}
	if !canGetLegalHold {
		protoObject.LegalHold = nil
	}

	segmentList, err := convertSegmentListResults(segments)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to convert stream list")
	}

	endpoint.log.Debug("Object Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "download"), zap.String("type", "object"))
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

func convertSegmentListResults(segments metabase.ListSegmentsResult) (*pb.SegmentListResponse, error) {
	items := make([]*pb.SegmentListItem, len(segments.Segments))
	for i, item := range segments.Segments {
		items[i] = &pb.SegmentListItem{
			Position: &pb.SegmentPosition{
				PartNumber: int32(item.Position.Part),
				Index:      int32(item.Position.Index),
			},
			PlainSize:     int64(item.PlainSize),
			PlainOffset:   item.PlainOffset,
			CreatedAt:     item.CreatedAt,
			EncryptedETag: item.EncryptedETag,
			EncryptedKey:  item.EncryptedKey,
		}
		var err error
		items[i].EncryptedKeyNonce, err = storj.NonceFromBytes(item.EncryptedKeyNonce)
		if err != nil {
			return nil, err
		}
	}
	return &pb.SegmentListResponse{
		Items: items,
		More:  segments.More,
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
		mon.Event("download_range", monkit.NewSeriesTag("type", "empty"))
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

		mon.Event("download_range", monkit.NewSeriesTag("type", "start"))

		return &metabase.StreamRange{
			PlainStart: r.Start.PlainStart,
			PlainLimit: object.TotalPlainSize,
		}, nil
	case *pb.Range_StartLimit:
		if r.StartLimit == nil {
			return nil, Error.New("StartEnd missing for Range_StartEnd")
		}

		mon.Event("download_range", monkit.NewSeriesTag("type", "startlimit"))

		return &metabase.StreamRange{
			PlainStart: r.StartLimit.PlainStart,
			PlainLimit: r.StartLimit.PlainLimit,
		}, nil
	case *pb.Range_Suffix:
		if r.Suffix == nil {
			return nil, Error.New("Suffix missing for Range_Suffix")
		}

		mon.Event("download_range", monkit.NewSeriesTag("type", "suffix"))

		return &metabase.StreamRange{
			PlainStart: object.TotalPlainSize - r.Suffix.PlainSuffix,
			PlainLimit: object.TotalPlainSize,
		}, nil
	}

	mon.Event("download_range", monkit.NewSeriesTag("type", "unsupported"))

	// if it's a new unsupported range type, let's return all data
	return nil, nil
}

// ListObjectsFlags contains flags for tuning the ListObjects query.
type ListObjectsFlags struct {
	VersionSkipRequery        int `default:"1000" help:"versions to skip before requerying"`
	PrefixSkipRequery         int `default:"1000" help:"prefixes to skip before requerying"`
	QueryExtraForNonRecursive int `default:"10" help:"extra items to list for non-recursive queries"`
	MinBatchSize              int `default:"100" help:"minimum number of items to query at a time"`
}

// ensure that ListObjectsParams and ListObjectsFlags are exactly compatible.
var _ = metabase.ListObjectsParams(ListObjectsFlags{})

// ListObjects list objects according to specific parameters.
func (endpoint *Endpoint) ListObjects(ctx context.Context, req *pb.ObjectListRequest) (resp *pb.ObjectListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	defer func() { err = endpoint.ConvertMetabaseErr(err) }()

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedPrefix,
		Time:          time.Now(),
	}, console.RateLimitList)
	if err != nil {
		return nil, err
	}

	defer func() {
		var tags []eventkit.Tag
		if resp != nil {
			tags = []eventkit.Tag{
				eventkit.Int64("listed_rows", int64(len(resp.Items))),
			}
		}
		endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req), tags...)
	}()

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
	}

	limit := int(req.Limit)
	if limit < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "limit is negative")
	}
	metabase.ListLimit.Ensure(&limit)

	delimiter := metabase.ObjectKey(metabase.Delimiter)
	if len(req.Delimiter) > 0 {
		delimiter = metabase.ObjectKey(req.Delimiter)
	}

	var prefix metabase.ObjectKey
	if len(req.EncryptedPrefix) != 0 {
		prefix = metabase.ObjectKey(req.EncryptedPrefix)
		if !req.ArbitraryPrefix && !strings.HasSuffix(string(prefix), string(delimiter)) {
			prefix += delimiter
		}
	}

	// Default to Commmitted status for backward-compatibility with older uplinks.
	status := metabase.CommittedUnversioned
	if req.Status != pb.Object_INVALID {
		status = metabase.ObjectStatus(req.Status)
	}

	cursorKey := metabase.ObjectKey(req.EncryptedCursor)
	cursorVersion := metabase.Version(0)
	if len(cursorKey) != 0 {
		cursorKey = prefix + cursorKey

		// This is a workaround to avoid duplicates while listing objects by libuplink.
		// Because version is not part of cursor yet we need to specify a version that lists the next
		// version from a given key.
		//
		// The meaning of the cursor is to start iterating from the next item.
		// For pending objects we sort items version ascending, for all others descending.
		//
		// So, to skip a given cursorKey we need to select either the last version (depending on the sorting order)
		// Or create a cursor key that's one higher and the first version (depending on the sorting order)

		if endpoint.config.UseListObjectsForListing {
			if status == metabase.Pending || bucket.Versioning.IsUnversioned() {
				// For pending objects it's the maximum version.
				cursorVersion = metabase.MaxVersion
			} else {
				// For non-pending objects it's 0. (Because they are sorted in descending order)
				cursorVersion = 0
			}
		} else {
			// TODO: for some reason the old codepath always set it to MaxVersion.
			// Let's keep it as such.
			//
			// I'm guessing IterateObjectsAllVersionsWithStatusAscending handles this logic internally.
			cursorVersion = metabase.MaxVersion
		}
	}

	if len(req.VersionCursor) != 0 {
		sv, err := metabase.StreamVersionIDFromBytes(req.VersionCursor)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		cursorVersion = sv.Version()
	}

	var include = includeForObjectEntry{
		SystemMetadata:       true,
		CustomMetadata:       true,
		ETag:                 true,
		ETagOrCustomMetadata: false,
	}

	if req.UseObjectIncludes {
		include.CustomMetadata = req.ObjectIncludes.Metadata
		// because multipart upload UploadID depends on some System metadata fields we need
		// to force reading it for listing pending object when its not included in options.
		// This is used by libuplink ListUploads method.
		include.SystemMetadata = status == metabase.Pending || !req.ObjectIncludes.ExcludeSystemMetadata
		include.ETag = req.ObjectIncludes.IncludeEtag
		include.ETagOrCustomMetadata = req.ObjectIncludes.IncludeEtagOrCustomMetadata
	}

	resp = &pb.ObjectListResponse{}

	// Currently for old and new uplinks both we iterate only the latest version.
	//
	// Old clients always have IncludeAllVersions = False.
	// We need to only list the latest version for old clients because
	// they do not have the necessary cursor logic to iterate over versions.
	//
	// New clients specify what they need.
	if endpoint.config.UseListObjectsForListing {
		result, err := endpoint.metabase.ListObjects(ctx,
			metabase.ListObjects{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(req.Bucket),
				Prefix:     prefix,
				Delimiter:  delimiter,
				Cursor: metabase.ListObjectsCursor{
					Key:     cursorKey,
					Version: cursorVersion,
				},
				Pending: status == metabase.Pending,
				// for pending, we always need all versions
				// when bucket is unversioned, then requesting all versions is slightly faster
				AllVersions: req.IncludeAllVersions || status == metabase.Pending,
				Recursive:   req.Recursive,
				Limit:       limit,

				IncludeCustomMetadata:       include.CustomMetadata,
				IncludeSystemMetadata:       include.SystemMetadata,
				IncludeETag:                 include.ETag,
				IncludeETagOrCustomMetadata: include.ETagOrCustomMetadata,

				Unversioned: bucket.Versioning.IsUnversioned(),
				Params:      metabase.ListObjectsParams(endpoint.config.ListObjects),
			})
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}

		for _, entry := range result.Objects {
			item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, include, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
			if err != nil {
				return nil, endpoint.ConvertMetabaseErr(err)
			}
			resp.Items = append(resp.Items, item)
		}
		resp.More = result.More
	} else {
		// For pending objects, we always need to list the versions.
		if status == metabase.Pending {
			// handles listing pending objects for all types of buckets
			err = endpoint.metabase.IterateObjectsAllVersionsWithStatusAscending(ctx,
				metabase.IterateObjectsWithStatus{
					ProjectID:  keyInfo.ProjectID,
					BucketName: metabase.BucketName(req.Bucket),
					Prefix:     prefix,
					Delimiter:  delimiter,
					Cursor: metabase.IterateCursor{
						Key:     cursorKey,
						Version: cursorVersion,
					},
					Recursive: req.Recursive,
					BatchSize: limit + 1,
					Pending:   true,

					IncludeCustomMetadata:       include.CustomMetadata,
					IncludeSystemMetadata:       include.SystemMetadata,
					IncludeETag:                 include.ETag,
					IncludeETagOrCustomMetadata: include.ETagOrCustomMetadata,
				}, func(ctx context.Context, it metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for len(resp.Items) < limit && it.Next(ctx, &entry) {
						item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, include, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
				return nil, endpoint.ConvertMetabaseErr(err)
			}
		} else if !req.IncludeAllVersions {
			if bucket.Versioning.IsUnversioned() {
				// handles listing for VersioningUnsupported and Unversioned buckets
				err = endpoint.metabase.IterateObjectsAllVersionsWithStatusAscending(ctx,
					metabase.IterateObjectsWithStatus{
						ProjectID:  keyInfo.ProjectID,
						BucketName: metabase.BucketName(req.Bucket),
						Prefix:     prefix,
						Delimiter:  delimiter,
						Cursor: metabase.IterateCursor{
							Key:     cursorKey,
							Version: cursorVersion,
						},
						Recursive: req.Recursive,
						BatchSize: limit + 1,
						Pending:   false,

						IncludeCustomMetadata:       include.CustomMetadata,
						IncludeSystemMetadata:       include.SystemMetadata,
						IncludeETag:                 include.ETag,
						IncludeETagOrCustomMetadata: include.ETagOrCustomMetadata,
					}, func(ctx context.Context, it metabase.ObjectsIterator) error {
						entry := metabase.ObjectEntry{}
						for len(resp.Items) < limit && it.Next(ctx, &entry) {
							item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, include, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
					return nil, endpoint.ConvertMetabaseErr(err)
				}
			} else {
				result, err := endpoint.metabase.ListObjects(ctx,
					metabase.ListObjects{
						ProjectID:  keyInfo.ProjectID,
						BucketName: metabase.BucketName(req.Bucket),
						Prefix:     prefix,
						Delimiter:  delimiter,
						Cursor: metabase.ListObjectsCursor{
							Key:     cursorKey,
							Version: cursorVersion,
						},
						Pending:     false,
						AllVersions: false,
						Recursive:   req.Recursive,
						Limit:       limit,

						IncludeCustomMetadata:       include.CustomMetadata,
						IncludeSystemMetadata:       include.SystemMetadata,
						IncludeETag:                 include.ETag,
						IncludeETagOrCustomMetadata: include.ETagOrCustomMetadata,
					})
				if err != nil {
					return nil, endpoint.ConvertMetabaseErr(err)
				}

				for _, entry := range result.Objects {
					item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, include, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
					if err != nil {
						return nil, endpoint.ConvertMetabaseErr(err)
					}
					resp.Items = append(resp.Items, item)
				}
				resp.More = result.More
			}
		} else {
			// handles listing all versions
			err = endpoint.metabase.IterateObjectsAllVersionsWithStatus(ctx,
				metabase.IterateObjectsWithStatus{
					ProjectID:  keyInfo.ProjectID,
					BucketName: metabase.BucketName(req.Bucket),
					Prefix:     prefix,
					Delimiter:  delimiter,
					Cursor: metabase.IterateCursor{
						Key:     cursorKey,
						Version: cursorVersion,
					},
					Recursive: req.Recursive,
					BatchSize: limit + 1,
					Pending:   false,

					IncludeCustomMetadata:       include.CustomMetadata,
					IncludeSystemMetadata:       include.SystemMetadata,
					IncludeETag:                 include.ETag,
					IncludeETagOrCustomMetadata: include.ETagOrCustomMetadata,
				}, func(ctx context.Context, it metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for len(resp.Items) < limit && it.Next(ctx, &entry) {
						item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, include, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
				return nil, endpoint.ConvertMetabaseErr(err)
			}
		}
	}

	endpoint.log.Debug("Object List", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "list"), zap.String("type", "object"))
	mon.Meter("req_list_object").Mark(1)

	return resp, nil
}

// ListPendingObjectStreams list pending objects according to specific parameters.
func (endpoint *Endpoint) ListPendingObjectStreams(ctx context.Context, req *pb.ObjectListPendingStreamsRequest) (resp *pb.ObjectListPendingStreamsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionList,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitList)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
	}

	cursor := metabase.StreamIDCursor{}
	if req.StreamIdCursor != nil {
		streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamIdCursor)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
		}
		cursor.StreamID, err = uuid.FromBytes(streamID.StreamId)
		if err != nil {
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
		}
	}

	limit := int(req.Limit)
	if limit < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "limit is negative")
	}
	metabase.ListLimit.Ensure(&limit)

	options := metabase.IteratePendingObjectsByKey{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
		BatchSize: limit + 1,
		Cursor:    cursor,
	}

	objectsEntries := make([]*pb.ObjectListItem, 0, limit)
	err = endpoint.metabase.IteratePendingObjectsByKey(ctx,
		options, func(ctx context.Context, it metabase.ObjectsIterator) error {
			entry := metabase.ObjectEntry{}
			for it.Next(ctx, &entry) {
				item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, "", includeAllForObjectEntry(), bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
				if err != nil {
					return err
				}
				objectsEntries = append(objectsEntries, item)
			}
			return nil
		},
	)
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	resp = &pb.ObjectListPendingStreamsResponse{}
	resp.Items = []*pb.ObjectListItem{}

	// TODO currently this request have a bug if we would like to list all pending objects
	// with the same name if we have more than single page of them (1000) because protobuf
	// cursor doesn't include additional things like StreamID so it's a bit useless to do
	// anything else than just combine results
	resp.Items = append(resp.Items, objectsEntries...)
	if len(resp.Items) >= limit {
		resp.More = len(resp.Items) > limit
		resp.Items = resp.Items[:limit]
	}

	endpoint.log.Debug("List pending object streams", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "list"), zap.String("type", "object"))

	mon.Meter("req_list_pending_object_streams").Mark(1)

	return resp, nil
}

// GetObjectIPs returns the IP addresses of the nodes holding the pieces for
// the provided object. This is useful for knowing the locations of the pieces.
func (endpoint *Endpoint) GetObjectIPs(ctx context.Context, req *pb.ObjectGetIPsRequest) (resp *pb.ObjectGetIPsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.ValidateAuthAny(ctx, req.Header, console.RateLimitHead,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionList,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// TODO we may need custom metabase request to avoid two DB calls
	object, err := endpoint.metabase.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	var pieceCountByNodeID map[storj.NodeID]int64
	var placement storj.PlacementConstraint

	// TODO this is short term fix to easily filter out IPs out of bucket/object placement
	// this request is not heavily used so it should be fine to add additional request to DB for now.
	var group errgroup.Group
	group.Go(func() error {
		placement, err = endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
		return err
	})
	group.Go(func() (err error) {
		pieceCountByNodeID, err = endpoint.metabase.GetStreamPieceCountByNodeID(ctx,
			metabase.GetStreamPieceCountByNodeID{
				ProjectID: keyInfo.ProjectID,
				StreamID:  object.StreamID,
			})
		return err
	})
	err = group.Wait()
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	nodeIDs := make([]storj.NodeID, 0, len(pieceCountByNodeID))
	for nodeID := range pieceCountByNodeID {
		nodeIDs = append(nodeIDs, nodeID)
	}

	nodeIPMap, err := endpoint.overlay.GetNodeIPsFromPlacement(ctx, nodeIDs, placement)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get node IPs from placement")
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

	mon.Meter("req_get_object_ips").Mark(1)

	return &pb.ObjectGetIPsResponse{
		Ips:                 nodeIPs,
		SegmentCount:        int64(object.SegmentCount),
		ReliablePieceCount:  reliablePieceCount,
		PieceCount:          pieceCount,
		PlacementConstraint: uint32(placement),
	}, nil
}

// UpdateObjectMetadata replaces object metadata.
func (endpoint *Endpoint) UpdateObjectMetadata(ctx context.Context, req *pb.ObjectUpdateMetadataRequest) (resp *pb.ObjectUpdateMetadataResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	encryptedUserData := metabase.EncryptedUserData{
		EncryptedMetadata:             req.EncryptedMetadata,
		EncryptedMetadataNonce:        nonceBytes(req.EncryptedMetadataNonce),
		EncryptedMetadataEncryptedKey: req.EncryptedMetadataEncryptedKey,
		EncryptedETag:                 req.EncryptedEtag,
	}

	if err := endpoint.checkEncryptedMetadataSize(encryptedUserData); err != nil {
		return nil, err
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	err = endpoint.metabase.UpdateObjectLastCommittedMetadata(ctx, metabase.UpdateObjectLastCommittedMetadata{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
		StreamID:          id,
		EncryptedUserData: encryptedUserData,
		SetEncryptedETag:  req.SetEncryptedEtag,
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	mon.Meter("req_update_object_metadata").Mark(1)

	return &pb.ObjectUpdateMetadataResponse{}, nil
}

// GetObjectLegalHold returns an object's Object Lock legal hold configuration.
func (endpoint *Endpoint) GetObjectLegalHold(ctx context.Context, req *pb.GetObjectLegalHoldRequest) (_ *pb.GetObjectLegalHoldResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionGetObjectLegalHold,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          now,
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	bucketLockEnabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", string(req.Bucket))
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket's Object Lock configuration")
	}
	if !bucketLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, bucketNoLockErrMsg)
	}

	loc := metabase.ObjectLocation{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(req.Bucket),
		ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
	}

	var enabled bool
	if len(req.ObjectVersion) == 0 {
		enabled, err = endpoint.metabase.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
			ObjectLocation: loc,
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		enabled, err = endpoint.metabase.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
			ObjectLocation: loc,
			Version:        sv.Version(),
		})
	}
	if err != nil {
		if metabase.ErrMethodNotAllowed.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		}
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	return &pb.GetObjectLegalHoldResponse{
		Enabled: enabled,
	}, nil
}

// SetObjectLegalHold sets an object's Object Lock legal hold configuration.
func (endpoint *Endpoint) SetObjectLegalHold(ctx context.Context, req *pb.SetObjectLegalHoldRequest) (_ *pb.SetObjectLegalHoldResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionPutObjectLegalHold,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          now,
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	bucketLockEnabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", string(req.Bucket))
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket's Object Lock configuration")
	}
	if !bucketLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, bucketNoLockErrMsg)
	}

	loc := metabase.ObjectLocation{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(req.Bucket),
		ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
	}

	if len(req.ObjectVersion) == 0 {
		err = endpoint.metabase.SetObjectLastCommittedLegalHold(ctx, metabase.SetObjectLastCommittedLegalHold{
			ObjectLocation: loc,
			Enabled:        req.Enabled,
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		err = endpoint.metabase.SetObjectExactVersionLegalHold(ctx, metabase.SetObjectExactVersionLegalHold{
			ObjectLocation: loc,
			Version:        sv.Version(),
			Enabled:        req.Enabled,
		})
	}
	if err != nil {
		if metabase.ErrObjectStatus.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		}
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	return &pb.SetObjectLegalHoldResponse{}, nil
}

// GetObjectRetention returns an object's Object Lock retention configuration.
func (endpoint *Endpoint) GetObjectRetention(ctx context.Context, req *pb.GetObjectRetentionRequest) (_ *pb.GetObjectRetentionResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionGetObjectRetention,
		Bucket:        req.Bucket,
		EncryptedPath: req.EncryptedObjectKey,
		Time:          now,
	}, console.RateLimitHead)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	bucketLockEnabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", string(req.Bucket))
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket's Object Lock configuration")
	}
	if !bucketLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, bucketNoLockErrMsg)
	}

	loc := metabase.ObjectLocation{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(req.Bucket),
		ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
	}

	var retention metabase.Retention
	if len(req.ObjectVersion) == 0 {
		retention, err = endpoint.metabase.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
			ObjectLocation: loc,
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		retention, err = endpoint.metabase.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
			ObjectLocation: loc,
			Version:        sv.Version(),
		})
	}
	if err != nil {
		if metabase.ErrMethodNotAllowed.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		}
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	if !retention.Enabled() {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockObjectRetentionConfigurationMissing, "object does not have a retention configuration")
	}

	return &pb.GetObjectRetentionResponse{
		Retention: &pb.Retention{
			Mode:        pb.Retention_Mode(retention.Mode),
			RetainUntil: retention.RetainUntil,
		},
	}, nil
}

// SetObjectRetention sets an object's Object Lock retention configuration.
func (endpoint *Endpoint) SetObjectRetention(ctx context.Context, req *pb.SetObjectRetentionRequest) (_ *pb.SetObjectRetentionResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()

	actions := []VerifyPermission{{
		Action: macaroon.Action{
			Op:            macaroon.ActionPutObjectRetention,
			Bucket:        req.Bucket,
			EncryptedPath: req.EncryptedObjectKey,
			Time:          now,
		},
	}}

	if req.BypassGovernanceRetention {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionBypassGovernanceRetention,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	retention := protobufRetentionToMetabase(req.Retention)

	bucketLockEnabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", string(req.Bucket))
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket's Object Lock configuration")
	}
	if !bucketLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, bucketNoLockErrMsg)
	}

	loc := metabase.ObjectLocation{
		ProjectID:  keyInfo.ProjectID,
		BucketName: metabase.BucketName(req.Bucket),
		ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
	}

	if len(req.ObjectVersion) == 0 {
		err = endpoint.metabase.SetObjectLastCommittedRetention(ctx, metabase.SetObjectLastCommittedRetention{
			ObjectLocation:   loc,
			Retention:        retention,
			BypassGovernance: req.BypassGovernanceRetention,
		})
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		err = endpoint.metabase.SetObjectExactVersionRetention(ctx, metabase.SetObjectExactVersionRetention{
			ObjectLocation:   loc,
			Version:          sv.Version(),
			Retention:        retention,
			BypassGovernance: req.BypassGovernanceRetention,
		})
	}
	if err != nil {
		if metabase.ErrObjectStatus.Has(err) || metabase.ErrObjectExpiration.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		}
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	return &pb.SetObjectRetentionResponse{}, nil
}

func (endpoint *Endpoint) objectToProto(ctx context.Context, object metabase.Object) (*pb.Object, error) {
	expires := time.Time{}
	if object.ExpiresAt != nil {
		expires = *object.ExpiresAt
	}

	// TotalPlainSize != 0 means object was uploaded with newer uplink
	multipartObject := object.TotalPlainSize != 0 && object.FixedSegmentSize <= 0
	streamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:             []byte(object.BucketName),
		EncryptedObjectKey: []byte(object.ObjectKey),
		Version:            int64(object.Version),
		CreationDate:       object.CreatedAt,
		ExpirationDate:     expires,
		StreamId:           object.StreamID[:],
		MultipartObject:    multipartObject,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(object.Encryption.CipherSuite),
			BlockSize:   int64(object.Encryption.BlockSize),
		},
		Retention: &pb.Retention{
			Mode:        pb.Retention_Mode(object.Retention.Mode),
			RetainUntil: object.Retention.RetainUntil,
		},
		LegalHold: object.LegalHold,
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

	var retention *pb.Retention
	if object.Retention.Enabled() {
		retention = &pb.Retention{
			Mode:        pb.Retention_Mode(object.Retention.Mode),
			RetainUntil: object.Retention.RetainUntil,
		}
	}

	result := &pb.Object{
		Bucket:             []byte(object.BucketName),
		EncryptedObjectKey: []byte(object.ObjectKey),
		ObjectVersion:      object.StreamVersionID().Bytes(),
		StreamId:           streamID,
		Status:             pb.Object_Status(object.Status),
		ExpiresAt:          expires,
		CreatedAt:          object.CreatedAt,

		TotalSize: object.TotalEncryptedSize,
		PlainSize: object.TotalPlainSize,

		EncryptedMetadata:             metadataBytes,
		EncryptedMetadataNonce:        nonce,
		EncryptedMetadataEncryptedKey: object.EncryptedMetadataEncryptedKey,
		EncryptedEtag:                 object.EncryptedETag,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(object.Encryption.CipherSuite),
			BlockSize:   int64(object.Encryption.BlockSize),
		},

		Retention: retention,
		LegalHold: &types.BoolValue{
			Value: object.LegalHold,
		},
	}

	return result, nil
}

type includeForObjectEntry struct {
	SystemMetadata       bool
	CustomMetadata       bool
	ETag                 bool
	ETagOrCustomMetadata bool
}

func includeAllForObjectEntry() includeForObjectEntry {
	return includeForObjectEntry{
		SystemMetadata:       true,
		CustomMetadata:       true,
		ETag:                 true,
		ETagOrCustomMetadata: false, // implied by CustomMetadata and ETag
	}
}

// keyAndNonce returns whether we should include information related to encryption key
// and cipher format (e.g. encryption type, block size, number of segments).
func (include *includeForObjectEntry) keyAndNonce(entry *metabase.ObjectEntry) bool {
	// When there's an explicit request for CustomMetadata it's not quite clear whether the client
	// wants the stream metadata, so we'll ensure it's always included in that scenario.
	return include.CustomMetadata || include.customMetadata(entry) || include.etag(entry)
}

// customMetadata checks whether we should include user defined custom metadata.
func (include *includeForObjectEntry) customMetadata(entry *metabase.ObjectEntry) bool {
	// We should only include custom metadata when it has been requested and it exists.
	// this affects when we should also include key and nonce.
	return (include.CustomMetadata || include.ETagOrCustomMetadata) && len(entry.EncryptedMetadata) > 0
}

func (include *includeForObjectEntry) etag(entry *metabase.ObjectEntry) bool {
	// We should only include etag when it has been requested and it exists.
	// this affects when we should also include key and nonce.
	return (include.ETag || include.ETagOrCustomMetadata) && len(entry.EncryptedETag) > 0
}

func (endpoint *Endpoint) objectEntryToProtoListItem(ctx context.Context, bucket []byte,
	entry metabase.ObjectEntry, prefixToPrependInSatStreamID metabase.ObjectKey,
	include includeForObjectEntry, placement storj.PlacementConstraint, versioned bool) (item *pb.ObjectListItem, err error) {

	item = &pb.ObjectListItem{
		EncryptedObjectKey: []byte(entry.ObjectKey),
		Status:             pb.Object_Status(entry.Status),
		ObjectVersion:      entry.StreamVersionID().Bytes(),
		IsLatest:           entry.IsLatest,
	}

	expiresAt := time.Time{}
	if entry.ExpiresAt != nil {
		expiresAt = *entry.ExpiresAt
	}

	if include.SystemMetadata {
		item.ExpiresAt = expiresAt
		item.CreatedAt = entry.CreatedAt
		item.PlainSize = entry.TotalPlainSize
	}

	if include.keyAndNonce(&entry) {
		var nonce storj.Nonce
		if len(entry.EncryptedMetadataNonce) > 0 {
			nonce, err = storj.NonceFromBytes(entry.EncryptedMetadataNonce)
			if err != nil {
				return nil, err
			}
		}
		item.EncryptedMetadataNonce = nonce
		item.EncryptedMetadataEncryptedKey = entry.EncryptedMetadataEncryptedKey

		streamMeta := &pb.StreamMeta{}

		if include.customMetadata(&entry) {
			err = pb.Unmarshal(entry.EncryptedMetadata, streamMeta)
			if err != nil {
				return nil, err
			}
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
	}

	if include.etag(&entry) {
		item.EncryptedEtag = entry.EncryptedETag
	}

	// Add Stream ID to list items if listing is for pending objects.
	// The client requires the Stream ID to use in the MultipartInfo.
	if entry.Status == metabase.Pending {
		satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
			Bucket:             bucket,
			EncryptedObjectKey: append([]byte(prefixToPrependInSatStreamID), []byte(entry.ObjectKey)...),
			Version:            int64(entry.Version),
			CreationDate:       entry.CreatedAt,
			ExpirationDate:     expiresAt,
			StreamId:           entry.StreamID[:],
			MultipartObject:    entry.FixedSegmentSize <= 0,
			EncryptionParameters: &pb.EncryptionParameters{
				CipherSuite: pb.CipherSuite(entry.Encryption.CipherSuite),
				BlockSize:   int64(entry.Encryption.BlockSize),
			},
			Placement: int32(placement),
			Versioned: versioned,
		})
		if err != nil {
			return nil, err
		}
		item.StreamId = &satStreamID
	}

	return item, nil
}

// Server side move.

// BeginMoveObject begins moving object to different key.
func (endpoint *Endpoint) BeginMoveObject(ctx context.Context, req *pb.ObjectBeginMoveRequest) (resp *pb.ObjectBeginMoveResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
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
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// if source and target buckets are different, we need to check their geofencing configs
	if !bytes.Equal(req.Bucket, req.NewBucket) {
		// TODO we may try to combine those two DB calls into single one
		oldBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
			}
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
		}
		newBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
		}
		if oldBucketPlacement != newBucketPlacement {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "copying object to bucket with different placement policy is not (yet) supported")
		}
	}

	result, err := endpoint.metabase.BeginMoveObject(ctx, metabase.BeginMoveObject{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},

		SegmentLimit: endpoint.config.CopyMoveSegmentLimit,
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	response, err := convertBeginMoveObjectResults(result)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:             req.Bucket,
		EncryptedObjectKey: req.EncryptedObjectKey,
		Version:            int64(result.Version),
		StreamId:           result.StreamID[:],
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(result.EncryptionParameters.CipherSuite),
			BlockSize:   int64(result.EncryptionParameters.BlockSize),
		},
	})
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create stream id")
	}

	endpoint.log.Debug("Object Move Begins", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "move"), zap.String("type", "object"))
	mon.Meter("req_move_object_begins").Mark(1)

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

	// TODO we need this because of an uplink issue with how we are storing key and nonce
	if result.EncryptedMetadataEncryptedKey == nil {
		streamMeta := &pb.StreamMeta{}
		err := pb.Unmarshal(result.EncryptedMetadata, streamMeta)
		if err != nil {
			return nil, err
		}
		if streamMeta.LastSegmentMeta != nil {
			result.EncryptedMetadataEncryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
			result.EncryptedMetadataNonce = streamMeta.LastSegmentMeta.KeyNonce
		}
	}

	var metadataNonce storj.Nonce
	var err error
	if len(result.EncryptedMetadataNonce) != 0 {
		metadataNonce, err = storj.NonceFromBytes(result.EncryptedMetadataNonce)
		if err != nil {
			return nil, err
		}
	}

	return &pb.ObjectBeginMoveResponse{
		EncryptedMetadataKey:      result.EncryptedMetadataEncryptedKey,
		EncryptedMetadataKeyNonce: metadataNonce,
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(result.EncryptionParameters.CipherSuite),
			BlockSize:   int64(result.EncryptionParameters.BlockSize),
		},
		SegmentKeys: keys,
	}, nil
}

// FinishMoveObject accepts new encryption keys for moved object and
// updates the corresponding object ObjectKey and segments EncryptedKey.
// It optionally sets retention mode and period on the new object.
func (endpoint *Endpoint) FinishMoveObject(ctx context.Context, req *pb.ObjectFinishMoveRequest) (resp *pb.ObjectFinishMoveResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	var (
		now       = time.Now()
		canDelete bool
	)

	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		},
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canDelete,
			Optional:        true,
		},
	}

	retention := protobufRetentionToMetabase(req.Retention)
	if retention.Enabled() {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectRetention,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		})
	}
	if req.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if (retention.Enabled() || req.LegalHold) && !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.NewBucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket state")
	}

	if err = validateBucketObjectLockStatus(bucket, retention, req.LegalHold); err != nil {
		return nil, err
	}

	if bucket.ObjectLock.Enabled && bucket.ObjectLock.DefaultRetentionMode != storj.NoRetention && !retention.Enabled() {
		retention = useDefaultBucketRetention(bucket.ObjectLock, now)
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	streamUUID, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	err = endpoint.metabase.FinishMoveObject(ctx, metabase.FinishMoveObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			Version:    metabase.Version(streamID.Version),
			StreamID:   streamUUID,
		},
		NewSegmentKeys:                   protobufkeysToMetabase(req.NewSegmentKeys),
		NewBucket:                        metabase.BucketName(req.NewBucket),
		NewEncryptedObjectKey:            metabase.ObjectKey(req.NewEncryptedObjectKey),
		NewEncryptedMetadataNonce:        req.NewEncryptedMetadataKeyNonce,
		NewEncryptedMetadataEncryptedKey: req.NewEncryptedMetadataKey,

		NewDisallowDelete: !canDelete,

		NewVersioned: bucket.Versioning == buckets.VersioningEnabled,

		Retention: retention,
		LegalHold: req.LegalHold,

		TransmitEvent: endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(streamID.Bucket)) ||
			endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(req.NewBucket)),
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	endpoint.log.Debug("Object Move Finished", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "move"), zap.String("type", "object"))
	mon.Meter("req_move_object_finished").Mark(1)

	return &pb.ObjectFinishMoveResponse{}, nil
}

// Server side copy.

// BeginCopyObject begins copying object to different key.
func (endpoint *Endpoint) BeginCopyObject(ctx context.Context, req *pb.ObjectBeginCopyRequest) (resp *pb.ObjectBeginCopyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.ServerSideCopy || endpoint.config.ServerSideCopyDisabled {
		return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Unimplemented")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()
	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut,
		VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		},
		VerifyPermission{
			Action: macaroon.Action{
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
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// if source and target buckets are different, we need to check their geofencing configs
	if !bytes.Equal(req.Bucket, req.NewBucket) {
		// TODO we may try to combine those two DB calls into single one
		oldBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.Bucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
			}
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
		}
		newBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket placement")
		}
		if oldBucketPlacement != newBucketPlacement {
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "copying object to bucket with different placement policy is not (yet) supported")
		}
	}

	var version metabase.Version
	if len(req.ObjectVersion) != 0 {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(req.ObjectVersion)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		version = sv.Version()
	}

	result, err := endpoint.metabase.BeginCopyObject(ctx, metabase.BeginCopyObject{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
		Version:      version,
		SegmentLimit: endpoint.config.CopyMoveSegmentLimit,
		VerifyLimits: func(encryptedObjectSize int64, nSegments int64) error {
			return endpoint.checkUploadLimitsForNewObject(ctx, keyInfo, encryptedObjectSize, nSegments)
		},
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	response, err := convertBeginCopyObjectResults(result)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:             req.Bucket,
		EncryptedObjectKey: req.EncryptedObjectKey,
		Version:            int64(result.Version),
		StreamId:           result.StreamID[:],
		EncryptionParameters: &pb.EncryptionParameters{
			CipherSuite: pb.CipherSuite(result.EncryptionParameters.CipherSuite),
			BlockSize:   int64(result.EncryptionParameters.BlockSize),
		},
	})
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create stream ID")
	}

	endpoint.log.Debug("Object Copy Begins", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "copy"), zap.String("type", "object"))
	mon.Meter("req_copy_object_begins").Mark(1)

	response.StreamId = satStreamID
	return response, nil
}

func convertBeginCopyObjectResults(result metabase.BeginCopyObjectResult) (*pb.ObjectBeginCopyResponse, error) {
	beginMoveObjectResult, err := convertBeginMoveObjectResults(metabase.BeginMoveObjectResult(result))
	if err != nil {
		return nil, err
	}

	return &pb.ObjectBeginCopyResponse{
		EncryptedMetadataKeyNonce: beginMoveObjectResult.EncryptedMetadataKeyNonce,
		EncryptedMetadataKey:      beginMoveObjectResult.EncryptedMetadataKey,
		SegmentKeys:               beginMoveObjectResult.SegmentKeys,
		EncryptionParameters:      beginMoveObjectResult.EncryptionParameters,
	}, nil
}

func useDefaultBucketRetention(bucketConfig buckets.ObjectLockSettings, timeNow time.Time) (retention metabase.Retention) {
	retention.Mode = bucketConfig.DefaultRetentionMode
	switch {
	case bucketConfig.DefaultRetentionDays != 0:
		retention.RetainUntil = timeNow.AddDate(0, 0, bucketConfig.DefaultRetentionDays)
	case bucketConfig.DefaultRetentionYears != 0:
		retention.RetainUntil = timeNow.AddDate(bucketConfig.DefaultRetentionYears, 0, 0)
	}

	return retention
}

func validateObjectRetentionWithTTL(maxObjectTTL *time.Duration, expiresAt time.Time) error {
	switch {
	case maxObjectTTL != nil:
		return rpcstatus.Error(rpcstatus.ObjectLockUploadWithTTLAPIKeyAndDefaultRetention,
			"cannot upload into a bucket with default retention settings using an API key that enforces an object expiration time")
	case !expiresAt.IsZero():
		return rpcstatus.Error(rpcstatus.ObjectLockUploadWithTTLAndDefaultRetention,
			"cannot specify an object expiration time when uploading into a bucket with default retention settings")
	}
	return nil
}

// FinishCopyObject accepts new encryption keys for object copy and
// updates the corresponding object ObjectKey and segments EncryptedKey.
// It optionally sets retention mode and period on the new object.
func (endpoint *Endpoint) FinishCopyObject(ctx context.Context, req *pb.ObjectFinishCopyRequest) (resp *pb.ObjectFinishCopyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.ServerSideCopy || endpoint.config.ServerSideCopyDisabled {
		return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Unimplemented")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()

	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionWrite,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		},
	}

	retention := protobufRetentionToMetabase(req.Retention)
	if retention.Enabled() {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectRetention,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		})
	}
	if req.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedObjectKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if (retention.Enabled() || req.LegalHold) && !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	encryptedUserData := metabase.EncryptedUserData{
		EncryptedMetadata:             req.NewEncryptedMetadata,
		EncryptedMetadataNonce:        nonceBytes(req.NewEncryptedMetadataKeyNonce),
		EncryptedMetadataEncryptedKey: req.NewEncryptedMetadataKey,
		EncryptedETag:                 req.NewEncryptedEtag,
	}
	if err := endpoint.checkEncryptedMetadataSize(encryptedUserData); err != nil {
		return nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.NewBucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket state")
	}

	if err = validateBucketObjectLockStatus(bucket, retention, req.LegalHold); err != nil {
		return nil, err
	}

	if bucket.ObjectLock.Enabled && bucket.ObjectLock.DefaultRetentionMode != storj.NoRetention && !retention.Enabled() {
		retention = useDefaultBucketRetention(bucket.ObjectLock, now)
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	streamUUID, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	newStreamID, err := uuid.New()
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	object, err := endpoint.metabase.FinishCopyObject(ctx, metabase.FinishCopyObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			Version:    metabase.Version(streamID.Version),
			StreamID:   streamUUID,
		},
		NewStreamID:           newStreamID,
		NewSegmentKeys:        protobufkeysToMetabase(req.NewSegmentKeys),
		NewBucket:             metabase.BucketName(req.NewBucket),
		NewEncryptedObjectKey: metabase.ObjectKey(req.NewEncryptedObjectKey),
		OverrideMetadata:      req.OverrideMetadata,

		NewEncryptedUserData: encryptedUserData,

		// TODO(ver): currently we always allow deletion, to not change behaviour.
		NewDisallowDelete: false,

		NewVersioned: bucket.Versioning == buckets.VersioningEnabled,

		Retention: retention,
		LegalHold: req.LegalHold,

		VerifyLimits: func(encryptedObjectSize int64, nSegments int64) error {
			return endpoint.addStorageUsageUpToLimit(ctx, keyInfo, encryptedObjectSize, nSegments)
		},

		IfNoneMatch: req.IfNoneMatch,

		TransmitEvent: endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(req.NewBucket)),
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	// we can return nil redundancy because this request won't be used for downloading
	protoObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	endpoint.log.Debug("Object Copy Finished", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "copy"), zap.String("type", "object"))
	mon.Meter("req_copy_object_finished").Mark(1)

	return &pb.ObjectFinishCopyResponse{
		Object: protoObject,
	}, nil
}

func nonceBytes(nonce storj.Nonce) []byte {
	if nonce.IsZero() {
		return nil
	}
	return nonce.Bytes()
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
