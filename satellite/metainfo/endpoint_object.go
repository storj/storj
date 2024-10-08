// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/identity"
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

	if err := endpoint.checkObjectUploadRate(ctx, keyInfo.ProjectID, req.Bucket, req.EncryptedObjectKey); err != nil {
		return nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
	}

	if retention.Enabled() || req.LegalHold {
		if bucket.Versioning != buckets.VersioningEnabled {
			return nil, rpcstatus.Errorf(rpcstatus.ObjectLockInvalidBucketState, "cannot specify Object Lock settings when uploading into a bucket without Versioning enabled")
		} else if !bucket.ObjectLock.Enabled {
			return nil, rpcstatus.Errorf(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "cannot specify Object Lock settings when uploading into a bucket without Object Lock enabled")
		}
	}

	if err := endpoint.ensureAttribution(ctx, req.Header, keyInfo, req.Bucket, nil, false); err != nil {
		return nil, err
	}

	streamID, err := uuid.New()
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream id")
	}

	// TODO this will work only with newest uplink
	// figure out what to do with this
	encryptionParameters := storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(req.EncryptionParameters.CipherSuite),
		BlockSize:   int32(req.EncryptionParameters.BlockSize), // TODO check conversion
	}

	var nonce []byte
	if !req.EncryptedMetadataNonce.IsZero() {
		nonce = req.EncryptedMetadataNonce[:]
	}

	opts := metabase.BeginObjectNextVersion{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			StreamID:   streamID,
			Version:    metabase.NextVersion,
		},
		Encryption: encryptionParameters,

		EncryptedMetadata:             req.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: req.EncryptedMetadataEncryptedKey,
		EncryptedMetadataNonce:        nonce,

		Retention: retention,
		LegalHold: req.LegalHold,
	}
	if !expiresAt.IsZero() {
		opts.ExpiresAt = &expiresAt
	}

	object, err := endpoint.metabase.BeginObjectNextVersion(ctx, opts)
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	satStreamID, err := endpoint.packStreamID(ctx, &internalpb.StreamID{
		Bucket:               []byte(object.BucketName),
		EncryptedObjectKey:   []byte(object.ObjectKey),
		Version:              int64(object.Version),
		CreationDate:         object.CreatedAt,
		ExpirationDate:       expiresAt, // TODO make ExpirationDate nullable
		StreamId:             object.StreamID[:],
		MultipartObject:      object.FixedSegmentSize <= 0,
		EncryptionParameters: req.EncryptionParameters,
		Placement:            int32(bucket.Placement),
		Versioned:            bucket.Versioning == buckets.VersioningEnabled,
	})
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream id")
	}

	endpoint.log.Debug("Object Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "object"))
	mon.Meter("req_put_object").Mark(1)

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
		endpoint.log.Error("unable to get max object TTL", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get max object TTL")
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
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	now := time.Now()
	var allowDelete, canGetRetention, canGetLegalHold bool
	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut,
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to parse stream id")
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

	request := metabase.CommitObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(streamID.Version),
		},
		Encryption: encryption,

		DisallowDelete: !allowDelete,

		Versioned: streamID.Versioned,
	}
	// uplink can send empty metadata with not empty key/nonce
	// we need to fix it on uplink side but that part will be
	// needed for backward compatibility
	if len(req.EncryptedMetadata) != 0 {
		request.OverrideEncryptedMetadata = true
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

	if err := endpoint.checkEncryptedMetadataSize(request.EncryptedMetadata, request.EncryptedMetadataEncryptedKey); err != nil {
		return nil, err
	}

	object, err := endpoint.metabase.CommitObject(ctx, request)
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}
	committedObject = &object

	pbObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		endpoint.log.Error("unable to convert metabase object", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
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

	if err := endpoint.checkObjectUploadRate(ctx, keyInfo.ProjectID, beginObjectReq.Bucket, beginObjectReq.EncryptedObjectKey); err != nil {
		return nil, nil, nil, err
	}

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, beginObjectReq.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, nil, nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", beginObjectReq.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, nil, nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
	}

	if (retention.Enabled() || beginObjectReq.LegalHold) && !bucket.ObjectLock.Enabled {
		return nil, nil, nil, rpcstatus.Errorf(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "cannot specify Object Lock settings when uploading into a bucket without Object Lock enabled")
	}

	if err := endpoint.ensureAttribution(ctx, beginObjectReq.Header, keyInfo, beginObjectReq.Bucket, nil, false); err != nil {
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, nil, nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream id")
	}

	encryptionParameters := storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(beginObjectReq.EncryptionParameters.CipherSuite),
		BlockSize:   int32(beginObjectReq.EncryptionParameters.BlockSize), // TODO check conversion
	}

	var metadataNonce []byte
	if !commitObjectReq.EncryptedMetadataNonce.IsZero() {
		metadataNonce = commitObjectReq.EncryptedMetadataNonce[:]
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

		EncryptedMetadata:             commitObjectReq.EncryptedMetadata,
		EncryptedMetadataEncryptedKey: commitObjectReq.EncryptedMetadataEncryptedKey,
		EncryptedMetadataNonce:        metadataNonce,

		Retention: retention,
		LegalHold: beginObjectReq.LegalHold,

		DisallowDelete: !allowDelete,

		Versioned: bucket.Versioning == buckets.VersioningEnabled,
	})
	if err != nil {
		return nil, nil, nil, endpoint.ConvertMetabaseErr(err)
	}

	err = endpoint.orders.UpdatePutInlineOrder(ctx, metabase.BucketLocation{
		ProjectID: keyInfo.ProjectID, BucketName: metabase.BucketName(beginObjectReq.Bucket),
	}, inlineUsed)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, nil, nil, rpcstatus.Error(rpcstatus.Internal, "unable to update PUT inline order")
	}

	if err := endpoint.addSegmentToUploadLimits(ctx, keyInfo, inlineUsed); err != nil {
		return nil, nil, nil, err
	}

	pbObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		endpoint.log.Error("unable to convert metabase object", zap.Error(err))
		return nil, nil, nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
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

	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
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

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, rpcstatus.Errorf(rpcstatus.Unauthenticated, "unable to get peer identity: %w", err)
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

	if err := endpoint.checkDownloadLimits(ctx, keyInfo); err != nil {
		return nil, err
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
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
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
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get encryption key nonce from metadata")
		}

		if segment.Inline() {
			err := endpoint.orders.UpdateGetInlineOrder(ctx, object.Location().Bucket(), downloadSizes.plainSize)
			if err != nil {
				endpoint.log.Error("internal", zap.Error(err))
				return nil, rpcstatus.Error(rpcstatus.Internal, "unable to update GET inline order")
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

		limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, peer, object.Location().Bucket(), segment, req.GetDesiredNodes(), downloadSizes.orderLimit)
		if err != nil {
			if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
				endpoint.log.Error("Unable to create order limits.",
					zap.Stringer("Project ID", keyInfo.ProjectID),
					zap.Stringer("API Key ID", keyInfo.ID),
					zap.Error(err),
				)
			}
			endpoint.log.Error("internal", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create order limits")
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
		}}, nil
	}()
	if err != nil {
		return nil, err
	}

	// convert to response
	protoObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		endpoint.log.Error("unable to convert object to proto", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
	}
	if !canGetRetention {
		protoObject.Retention = nil
	}
	if !canGetLegalHold {
		protoObject.LegalHold = nil
	}

	segmentList, err := convertSegmentListResults(segments)
	if err != nil {
		endpoint.log.Error("unable to convert stream list", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to convert stream list")
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

// ListObjects list objects according to specific parameters.
func (endpoint *Endpoint) ListObjects(ctx context.Context, req *pb.ObjectListRequest) (resp *pb.ObjectListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

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
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// TODO this needs to be optimized to avoid DB call on each request
	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.Bucket)
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
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
	status := metabase.CommittedUnversioned
	if req.Status != pb.Object_INVALID {
		status = metabase.ObjectStatus(req.Status)
	}

	cursorKey := metabase.ObjectKey(req.EncryptedCursor)
	cursorVersion := metabase.Version(0)
	if len(cursorKey) != 0 {
		cursorKey = prefix + cursorKey

		// TODO this is a workaround to avoid duplicates while listing objects by libuplink.
		// because version is not part of cursor yet and we can have object with version higher
		// than 1 we cannot use hardcoded version 1 as default.
		// This workaround should be in place for a longer time even if metainfo protocol will be
		// fix as we still want to avoid this problem for older libuplink versions.
		//
		// it should be set in case of pending and committed objects
		cursorVersion = metabase.MaxVersion
	}

	if len(req.VersionCursor) != 0 {
		sv, err := metabase.StreamVersionIDFromBytes(req.VersionCursor)
		if err != nil {
			return nil, endpoint.ConvertMetabaseErr(err)
		}
		cursorVersion = sv.Version()
	}

	includeCustomMetadata := true
	includeSystemMetadata := true
	if req.UseObjectIncludes {
		includeCustomMetadata = req.ObjectIncludes.Metadata
		// because multipart upload UploadID depends on some System metadata fields we need
		// to force reading it for listing pending object when its not included in options.
		// This is used by libuplink ListUploads method.
		includeSystemMetadata = status == metabase.Pending || !req.ObjectIncludes.ExcludeSystemMetadata
	}

	resp = &pb.ObjectListResponse{}

	// Currently for old and new uplinks both we iterate only the latest version.
	//
	// Old clients always have IncludeAllVersions = False.
	// We need to only list the latest version for old clients because
	// they do not have the necessary cursor logic to iterate over versions.
	//
	// New clients specify what they need.

	// For pending objects, we always need to list the versions.
	if status == metabase.Pending {
		// handles listing pending objects for all types of buckets
		err = endpoint.metabase.IterateObjectsAllVersionsWithStatusAscending(ctx,
			metabase.IterateObjectsWithStatus{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(req.Bucket),
				Prefix:     prefix,
				Cursor: metabase.IterateCursor{
					Key:     cursorKey,
					Version: cursorVersion,
				},
				Recursive:             req.Recursive,
				BatchSize:             limit + 1,
				Pending:               true,
				IncludeCustomMetadata: includeCustomMetadata,
				IncludeSystemMetadata: includeSystemMetadata,
			}, func(ctx context.Context, it metabase.ObjectsIterator) error {
				entry := metabase.ObjectEntry{}
				for len(resp.Items) < limit && it.Next(ctx, &entry) {
					item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, includeSystemMetadata, includeCustomMetadata, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
					Cursor: metabase.IterateCursor{
						Key:     cursorKey,
						Version: cursorVersion,
					},
					Recursive:             req.Recursive,
					BatchSize:             limit + 1,
					Pending:               false,
					IncludeCustomMetadata: includeCustomMetadata,
					IncludeSystemMetadata: includeSystemMetadata,
				}, func(ctx context.Context, it metabase.ObjectsIterator) error {
					entry := metabase.ObjectEntry{}
					for len(resp.Items) < limit && it.Next(ctx, &entry) {
						item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, includeSystemMetadata, includeCustomMetadata, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
					Cursor: metabase.ListObjectsCursor{
						Key:     cursorKey,
						Version: cursorVersion,
					},
					Pending:     false,
					AllVersions: false,
					Recursive:   req.Recursive,
					Limit:       limit,

					IncludeCustomMetadata: includeCustomMetadata,
					IncludeSystemMetadata: includeSystemMetadata,
				})
			if err != nil {
				return nil, endpoint.ConvertMetabaseErr(err)
			}

			for _, entry := range result.Objects {
				item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, includeSystemMetadata, includeCustomMetadata, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
				Cursor: metabase.IterateCursor{
					Key:     cursorKey,
					Version: cursorVersion,
				},
				Recursive:             req.Recursive,
				BatchSize:             limit + 1,
				Pending:               false,
				IncludeCustomMetadata: includeCustomMetadata,
				IncludeSystemMetadata: includeSystemMetadata,
			}, func(ctx context.Context, it metabase.ObjectsIterator) error {
				entry := metabase.ObjectEntry{}
				for len(resp.Items) < limit && it.Next(ctx, &entry) {
					item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, prefix, includeSystemMetadata, includeCustomMetadata, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
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
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to parse stream id")
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
				item, err := endpoint.objectEntryToProtoListItem(ctx, req.Bucket, entry, "", true, true, bucket.Placement, bucket.Versioning == buckets.VersioningEnabled)
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

// BeginDeleteObject begins object deletion process.
func (endpoint *Endpoint) BeginDeleteObject(ctx context.Context, req *pb.ObjectBeginDeleteRequest) (resp *pb.ObjectBeginDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = validateRequestSimple(req); err != nil {
		return nil, err
	}

	now := time.Now()

	var canRead, canList, canGetRetention bool

	actions := []VerifyPermission{
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionRead,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionList,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		}, {
			Action: macaroon.Action{
				Op:            macaroon.ActionGetObjectRetention,
				Bucket:        req.Bucket,
				EncryptedPath: req.EncryptedObjectKey,
				Time:          now,
			},
			ActionPermitted: &canGetRetention,
			Optional:        true,
		},
	}

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

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitDelete, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

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
						BucketName: metabase.BucketName(pbStreamID.Bucket),
						ObjectKey:  metabase.ObjectKey(pbStreamID.EncryptedObjectKey),
						Version:    metabase.Version(pbStreamID.Version),
						StreamID:   streamID,
					})
			}
		}
	} else {
		deletedObjects, err = endpoint.DeleteCommittedObject(ctx, DeleteCommittedObject{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(req.Bucket),
				ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
			},
			Version:          req.ObjectVersion,
			BypassGovernance: req.BypassGovernanceRetention,
		})
	}
	if err != nil {
		if !canRead && !canList {
			// No error info is returned if neither Read, nor List permission is granted
			return &pb.ObjectBeginDeleteResponse{}, nil
		}
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	var object *pb.Object
	if canRead || canList {
		// Info about deleted object is returned only if either Read, or List permission is granted
		if err != nil {
			endpoint.log.Error("failed to construct deleted object information",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Stringer("Bucket", metabase.BucketName(req.Bucket)),
				zap.String("Encrypted Path", string(req.EncryptedObjectKey)),
				zap.Error(err),
			)
		}
		if len(deletedObjects) > 0 {
			object = deletedObjects[0]
			if !canGetRetention {
				object.Retention = nil
			}
		}
	}

	endpoint.log.Debug("Object Delete", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "delete"), zap.String("type", "object"))
	mon.Meter("req_delete_object").Mark(1)

	return &pb.ObjectBeginDeleteResponse{
		Object: object,
	}, nil
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get node IPs from placement")
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
		Ips:                nodeIPs,
		SegmentCount:       int64(object.SegmentCount),
		ReliablePieceCount: reliablePieceCount,
		PieceCount:         pieceCount,
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

	if err := endpoint.checkEncryptedMetadataSize(req.EncryptedMetadata, req.EncryptedMetadataEncryptedKey); err != nil {
		return nil, err
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to parse stream id")
	}

	var encryptedMetadataNonce []byte
	if !req.EncryptedMetadataNonce.IsZero() {
		encryptedMetadataNonce = req.EncryptedMetadataNonce[:]
	}

	err = endpoint.metabase.UpdateObjectLastCommittedMetadata(ctx, metabase.UpdateObjectLastCommittedMetadata{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),
			ObjectKey:  metabase.ObjectKey(req.EncryptedObjectKey),
		},
		StreamID:                      id,
		EncryptedMetadata:             req.EncryptedMetadata,
		EncryptedMetadataNonce:        encryptedMetadataNonce,
		EncryptedMetadataEncryptedKey: req.EncryptedMetadataEncryptedKey,
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
		endpoint.log.Error("unable to get bucket's Object Lock configuration", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket's Object Lock configuration")
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
		endpoint.log.Error("unable to get bucket's Object Lock configuration", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket's Object Lock configuration")
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
		endpoint.log.Error("unable to get bucket's Object Lock configuration", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket's Object Lock configuration")
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
		endpoint.log.Error("unable to get bucket's Object Lock configuration", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket's Object Lock configuration")
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

	if metabase.Version(int32(object.Version)) != object.Version {
		return nil, errs.New("unable to convert version for protobuf object")
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

func (endpoint *Endpoint) objectEntryToProtoListItem(ctx context.Context, bucket []byte,
	entry metabase.ObjectEntry, prefixToPrependInSatStreamID metabase.ObjectKey,
	includeSystem, includeMetadata bool, placement storj.PlacementConstraint, versioned bool) (item *pb.ObjectListItem, err error) {

	item = &pb.ObjectListItem{
		EncryptedObjectKey: []byte(entry.ObjectKey),
		Status:             pb.Object_Status(entry.Status),
		ObjectVersion:      entry.StreamVersionID().Bytes(),
	}

	expiresAt := time.Time{}
	if entry.ExpiresAt != nil {
		expiresAt = *entry.ExpiresAt
	}

	if includeSystem {
		item.ExpiresAt = expiresAt
		item.CreatedAt = entry.CreatedAt
		item.PlainSize = entry.TotalPlainSize
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

// DeleteCommittedObject contains arguments necessary for deleting a committed version
// of an object via the (*Endpoint).DeleteCommittedObject method.
type DeleteCommittedObject struct {
	metabase.ObjectLocation
	Version          []byte
	BypassGovernance bool
}

// DeleteCommittedObject deletes all the pieces of the storage nodes that belongs
// to the specified object.
//
// NOTE: this method is exported for being able to individually test it without
// having import cycles.
func (endpoint *Endpoint) DeleteCommittedObject(ctx context.Context, opts DeleteCommittedObject) (deletedObjects []*pb.Object, err error) {
	defer mon.Task()(&ctx, opts.ProjectID.String(), opts.BucketName, opts.ObjectKey)(&err)

	req := metabase.ObjectLocation{
		ProjectID:  opts.ProjectID,
		BucketName: opts.BucketName,
		ObjectKey:  opts.ObjectKey,
	}

	// TODO(ver): for production we need to avoid somehow additional GetBucket call
	bucketData, err := endpoint.buckets.GetBucket(ctx, []byte(opts.BucketName), opts.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, nil
		}
		endpoint.log.Error("unable to check bucket", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket state")
	}

	var result metabase.DeleteObjectResult
	if len(opts.Version) == 0 {
		versioned := bucketData.Versioning == buckets.VersioningEnabled
		suspended := bucketData.Versioning == buckets.VersioningSuspended

		result, err = endpoint.metabase.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
			ObjectLocation: req,
			Versioned:      versioned,
			Suspended:      suspended,

			ObjectLock: metabase.ObjectLockDeleteOptions{
				Enabled:          bucketData.ObjectLock.Enabled,
				BypassGovernance: opts.BypassGovernance,
			},
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	} else {
		var sv metabase.StreamVersionID
		sv, err = metabase.StreamVersionIDFromBytes(opts.Version)
		if err != nil {
			return nil, err
		}
		result, err = endpoint.metabase.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
			ObjectLocation: req,
			Version:        sv.Version(),

			ObjectLock: metabase.ObjectLockDeleteOptions{
				Enabled:          bucketData.ObjectLock.Enabled,
				BypassGovernance: opts.BypassGovernance,
			},
		})
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	deletedObjects, err = endpoint.deleteObjectResultToProto(ctx, result)
	if err != nil {
		endpoint.log.Error("failed to convert delete object result",
			zap.Stringer("project", opts.ProjectID),
			zap.String("bucket", opts.BucketName.String()),
			zap.Binary("object", []byte(opts.ObjectKey)),
			zap.Error(err),
		)
		return nil, Error.Wrap(err)
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

	return endpoint.deleteObjectResultToProto(ctx, result)
}

func (endpoint *Endpoint) deleteObjectResultToProto(ctx context.Context, result metabase.DeleteObjectResult) (deletedObjects []*pb.Object, err error) {
	deletedObjects = make([]*pb.Object, 0, len(result.Removed)+len(result.Markers))
	for _, object := range result.Removed {
		deletedObject, err := endpoint.objectToProto(ctx, object)
		if err != nil {
			return nil, err
		}
		deletedObjects = append(deletedObjects, deletedObject)
	}
	for _, object := range result.Markers {
		deletedObject, err := endpoint.objectToProto(ctx, object)
		if err != nil {
			return nil, err
		}
		deletedObjects = append(deletedObjects, deletedObject)
	}

	return deletedObjects, nil
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
			endpoint.log.Error("unable to check bucket", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
		}
		newBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check bucket", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
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
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	response, err := convertBeginMoveObjectResults(result)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream id")
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
				EncryptedPath: req.NewEncryptedMetadataKey,
				Time:          now,
			},
		},
		{
			Action: macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedMetadataKey,
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
				EncryptedPath: req.NewEncryptedMetadataKey,
				Time:          now,
			},
		})
	}
	if req.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedMetadataKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	objectLockRequested := retention.Enabled() || req.LegalHold
	if objectLockRequested && !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.ObjectLockEndpointsDisabled, objectLockDisabledErrMsg)
	}

	var versioningEnabled bool

	if objectLockRequested {
		enabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check whether bucket has Object Lock enabled", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to copy object")
		}
		if !enabled {
			return nil, rpcstatus.Errorf(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "cannot specify Object Lock settings when uploading into a bucket without Object Lock enabled")
		}
		versioningEnabled = true
	} else {
		state, err := endpoint.buckets.GetBucketVersioningState(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check bucket versioning state", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to copy object")
		}
		versioningEnabled = state == buckets.VersioningEnabled
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamUUID, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	err = endpoint.metabase.FinishMoveObject(ctx, metabase.FinishMoveObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			Version:    metabase.Version(streamID.Version),
			StreamID:   streamUUID,
		},
		NewSegmentKeys:               protobufkeysToMetabase(req.NewSegmentKeys),
		NewBucket:                    metabase.BucketName(req.NewBucket),
		NewEncryptedObjectKey:        metabase.ObjectKey(req.NewEncryptedObjectKey),
		NewEncryptedMetadataKeyNonce: req.NewEncryptedMetadataKeyNonce,
		NewEncryptedMetadataKey:      req.NewEncryptedMetadataKey,

		NewDisallowDelete: !canDelete,

		NewVersioned: versioningEnabled,

		Retention: retention,
		LegalHold: req.LegalHold,
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
			endpoint.log.Error("unable to check bucket", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
		}
		newBucketPlacement, err := endpoint.buckets.GetBucketPlacement(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check bucket", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to get bucket placement")
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
		Version: version,
		VerifyLimits: func(encryptedObjectSize int64, nSegments int64) error {
			return endpoint.checkUploadLimitsForNewObject(ctx, keyInfo, encryptedObjectSize, nSegments)
		},
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	response, err := convertBeginCopyObjectResults(result)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream ID")
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
				EncryptedPath: req.NewEncryptedMetadataKey,
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
				EncryptedPath: req.NewEncryptedMetadataKey,
				Time:          now,
			},
		})
	}
	if req.LegalHold {
		actions = append(actions, VerifyPermission{
			Action: macaroon.Action{
				Op:            macaroon.ActionPutObjectLegalHold,
				Bucket:        req.NewBucket,
				EncryptedPath: req.NewEncryptedMetadataKey,
				Time:          now,
			},
		})
	}

	keyInfo, err := endpoint.ValidateAuthN(ctx, req.Header, console.RateLimitPut, actions...)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	objectLockRequested := retention.Enabled() || req.LegalHold
	if objectLockRequested && !endpoint.config.ObjectLockEnabled {
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, objectLockDisabledErrMsg)
	}

	if err := endpoint.checkEncryptedMetadataSize(req.NewEncryptedMetadata, req.NewEncryptedMetadataKey); err != nil {
		return nil, err
	}

	var versioningEnabled bool

	if objectLockRequested {
		enabled, err := endpoint.buckets.GetBucketObjectLockEnabled(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check whether bucket has Object Lock enabled", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to copy object")
		}
		if !enabled {
			return nil, rpcstatus.Errorf(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "cannot specify Object Lock settings when uploading into a bucket without Object Lock enabled")
		}
		versioningEnabled = true
	} else {
		state, err := endpoint.buckets.GetBucketVersioningState(ctx, req.NewBucket, keyInfo.ProjectID)
		if err != nil {
			if buckets.ErrBucketNotFound.Has(err) {
				return nil, rpcstatus.Errorf(rpcstatus.NotFound, "bucket not found: %s", req.NewBucket)
			}
			endpoint.log.Error("unable to check bucket versioning state", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, "unable to copy object")
		}
		versioningEnabled = state == buckets.VersioningEnabled
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamUUID, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	newStreamID, err := uuid.New()
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	object, err := endpoint.metabase.FinishCopyObject(ctx, metabase.FinishCopyObject{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			Version:    metabase.Version(streamID.Version),
			StreamID:   streamUUID,
		},
		NewStreamID:                  newStreamID,
		NewSegmentKeys:               protobufkeysToMetabase(req.NewSegmentKeys),
		NewBucket:                    metabase.BucketName(req.NewBucket),
		NewEncryptedObjectKey:        metabase.ObjectKey(req.NewEncryptedObjectKey),
		OverrideMetadata:             req.OverrideMetadata,
		NewEncryptedMetadata:         req.NewEncryptedMetadata,
		NewEncryptedMetadataKeyNonce: req.NewEncryptedMetadataKeyNonce,
		NewEncryptedMetadataKey:      req.NewEncryptedMetadataKey,

		// TODO(ver): currently we always allow deletion, to not change behaviour.
		NewDisallowDelete: false,

		NewVersioned: versioningEnabled,

		Retention: retention,
		LegalHold: req.LegalHold,

		VerifyLimits: func(encryptedObjectSize int64, nSegments int64) error {
			return endpoint.addStorageUsageUpToLimit(ctx, keyInfo, encryptedObjectSize, nSegments)
		},
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	// we can return nil redundancy because this request won't be used for downloading
	protoObject, err := endpoint.objectToProto(ctx, object)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "internal error")
	}

	endpoint.log.Debug("Object Copy Finished", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "copy"), zap.String("type", "object"))
	mon.Meter("req_copy_object_finished").Mark(1)

	return &pb.ObjectFinishCopyResponse{
		Object: protoObject,
	}, nil
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
