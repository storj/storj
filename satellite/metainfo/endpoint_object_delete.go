// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
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

	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[keyInfo.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

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
				deletedObjects, err = endpoint.DeletePendingObject(ctx, metabase.DeletePendingObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  keyInfo.ProjectID,
						BucketName: metabase.BucketName(pbStreamID.Bucket),
						ObjectKey:  metabase.ObjectKey(pbStreamID.EncryptedObjectKey),
						Version:    metabase.Version(pbStreamID.Version),
						StreamID:   streamID,
					},
					MaxCommitDelay: maxCommitDelay,
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

			TransmitEvent: endpoint.bucketEventing.Enabled(opts.ProjectID, opts.BucketName.String()),
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

			TransmitEvent: endpoint.bucketEventing.Enabled(opts.ProjectID, opts.BucketName.String()),
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
func (endpoint *Endpoint) DeletePendingObject(ctx context.Context, opts metabase.DeletePendingObject) (deletedObjects []*pb.Object, err error) {
	result, err := endpoint.metabase.DeletePendingObject(ctx, opts)
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

// DeleteObjects deletes multiple objects from a bucket.
func (endpoint *Endpoint) DeleteObjects(ctx context.Context, req *pb.DeleteObjectsRequest) (resp *pb.DeleteObjectsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.DeleteObjectsEnabled {
		return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Unimplemented")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	if err = endpoint.validateDeleteObjectsRequestSimple(req); err != nil {
		return nil, err
	}

	key, keyInfo, err := endpoint.validateBasic(ctx, req.Header, console.RateLimitDelete)
	if err != nil {
		return nil, err
	}

	if endpoint.migrationModeFlag.Enabled() {
		if _, found := endpoint.config.TestingSpannerProjects[keyInfo.ProjectID]; !found {
			return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "try again later")
		}
	}

	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	deduplicateDeleteObjectsItems(req)

	resp = &pb.DeleteObjectsResponse{
		Items: make([]*pb.DeleteObjectsResponseItem, 0, len(req.Items)),
	}

	now := time.Now()

	// Return early if the requester has no access to this bucket.
	if key.Check(ctx, keyInfo.Secret, keyInfo.Version, macaroon.Action{
		Op:     macaroon.ActionRead,
		Bucket: req.Bucket,
		Time:   now,
	}, endpoint.revocations) != nil {
		for _, item := range req.Items {
			resp.Items = append(resp.Items, &pb.DeleteObjectsResponseItem{
				EncryptedObjectKey:     item.EncryptedObjectKey,
				RequestedObjectVersion: item.ObjectVersion,
				Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
			})
		}
		return resp, nil
	}

	bucket, err := endpoint.buckets.GetBucket(ctx, req.Bucket, keyInfo.ProjectID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.BucketNotFound, "The specified bucket was not found")
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get bucket state")
	}

	allowedObjectKeys := make(map[string]bool, len(req.Items))
	var numAllowedObjectKeys int
	for _, item := range req.Items {
		objectKey := unsafeBytesToString(item.EncryptedObjectKey)

		allowed, ok := allowedObjectKeys[objectKey]
		if !ok {
			action := macaroon.Action{
				Op:            macaroon.ActionDelete,
				Bucket:        req.Bucket,
				EncryptedPath: item.EncryptedObjectKey,
				Time:          now,
			}

			// TODO: Every invocation of key.Check validates the macaroon, unmarshalls its caveats,
			// and checks the revocation DB. These operations only need to occur once.
			allowed = key.Check(ctx, keyInfo.Secret, keyInfo.Version, action, endpoint.revocations) == nil
			if allowed && req.BypassGovernanceRetention {
				action.Op = macaroon.ActionBypassGovernanceRetention
				allowed = key.Check(ctx, keyInfo.Secret, keyInfo.Version, action, endpoint.revocations) == nil
			}

			allowedObjectKeys[objectKey] = allowed

			if allowed {
				numAllowedObjectKeys++
			}
		}

		if !allowed {
			resp.Items = append(resp.Items, &pb.DeleteObjectsResponseItem{
				EncryptedObjectKey:     item.EncryptedObjectKey,
				RequestedObjectVersion: item.ObjectVersion,
				Status:                 pb.DeleteObjectsResponseItem_UNAUTHORIZED,
			})
		}
	}

	if numAllowedObjectKeys > 0 {
		deleteObjectsOpts := metabase.DeleteObjects{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(req.Bucket),

			Versioned: bucket.Versioning == buckets.VersioningEnabled,
			Suspended: bucket.Versioning == buckets.VersioningSuspended,
			ObjectLock: metabase.ObjectLockDeleteOptions{
				Enabled:          bucket.ObjectLock.Enabled,
				BypassGovernance: req.BypassGovernanceRetention,
			},

			Items: make([]metabase.DeleteObjectsItem, 0, numAllowedObjectKeys),

			TransmitEvent: endpoint.bucketEventing.Enabled(keyInfo.ProjectID, string(req.Bucket)),
		}

		for _, item := range req.Items {
			objectKey := unsafeBytesToString(item.EncryptedObjectKey)
			if !allowedObjectKeys[objectKey] {
				continue
			}

			deleteObjectsItem := metabase.DeleteObjectsItem{
				ObjectKey: metabase.ObjectKey(objectKey),
			}
			if len(item.ObjectVersion) != 0 {
				deleteObjectsItem.StreamVersionID = metabase.StreamVersionID(item.ObjectVersion)
			}
			deleteObjectsOpts.Items = append(deleteObjectsOpts.Items, deleteObjectsItem)
		}

		deleteObjectsResult, err := endpoint.metabase.DeleteObjects(ctx, deleteObjectsOpts)
		if err != nil {
			endpoint.log.Error("error deleting objects",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Stringer("Bucket", metabase.BucketName(req.Bucket)),
				zap.Error(err),
			)
		}

		addDeleteObjectsResultToProto(resp, deleteObjectsResult, req.Quiet)
	}

	return resp, nil
}

func addDeleteObjectsResultToProto(pbResult *pb.DeleteObjectsResponse, metabaseResult metabase.DeleteObjectsResult, quiet bool) {
	for _, metabaseItem := range metabaseResult.Items {
		if metabaseItem.Status == storj.DeleteObjectsStatusOK && quiet {
			continue
		}

		pbResponseItem := &pb.DeleteObjectsResponseItem{
			EncryptedObjectKey: unsafeStringToBytes(string(metabaseItem.ObjectKey)),
			Status:             pb.DeleteObjectsResponseItem_Status(metabaseItem.Status),
		}

		if !metabaseItem.RequestedStreamVersionID.IsZero() {
			pbResponseItem.RequestedObjectVersion = metabaseItem.RequestedStreamVersionID.Bytes()
		}

		if metabaseItem.Removed != nil {
			pbResponseItem.Removed = &pb.DeleteObjectsResponseItemInfo{
				ObjectVersion: metabaseItem.Removed.StreamVersionID.Bytes(),
				Status:        pb.Object_Status(metabaseItem.Removed.Status),
			}
		}

		if metabaseItem.Marker != nil {
			pbResponseItem.Marker = &pb.DeleteObjectsResponseItemInfo{
				ObjectVersion: metabaseItem.Marker.StreamVersionID.Bytes(),
				Status:        pb.Object_Status(metabaseItem.Marker.Status),
			}
		}

		pbResult.Items = append(pbResult.Items, pbResponseItem)
	}
}

func deduplicateDeleteObjectsItems(req *pb.DeleteObjectsRequest) {
	slices.SortStableFunc(req.Items, cmpDeleteObjectsRequestItem)

	compacted := slices.CompactFunc(req.Items, func(a, b *pb.DeleteObjectsRequestItem) bool {
		return cmpDeleteObjectsRequestItem(a, b) == 0
	})

	// Zero the pointers to the discarded items so that they can be garbage collected
	for i := len(compacted); i < len(req.Items); i++ {
		req.Items[i] = nil
	}

	req.Items = compacted
}

func cmpDeleteObjectsRequestItem(a, b *pb.DeleteObjectsRequestItem) int {
	if cmp := cmpBytes(a.EncryptedObjectKey, b.EncryptedObjectKey); cmp != 0 {
		return cmp
	}
	return cmpBytes(a.ObjectVersion, b.ObjectVersion)
}

// unsafeBytesToString returns a string backed by the given byte slice.
// It can be used in cases where allocations should be minimized.
// The byte slice is expected to remain constant throughout the string's lifetime.
func unsafeBytesToString(b []byte) string {
	numBytes := len(b)
	if numBytes == 0 {
		return ""
	}
	return unsafe.String(&b[0], numBytes)
}

// unsafeStringToBytes returns a byte slice aliasing the given string.
// It can be used in cases where allocations should be minimized.
// The byte slice should not be modified.
func unsafeStringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// cmpBytes performs a shortlex comparison of the provided byte slices, returning -1, 0, or 1
// if the first slice is less than, equal to, or greater than the second, respectively.
func cmpBytes(a, b []byte) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}

	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	return 0
}
