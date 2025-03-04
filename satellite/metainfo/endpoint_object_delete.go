// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
)

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
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket state")
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
