// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"io"
	"reflect"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/minio/minio/pkg/madmin"
	"github.com/minio/minio/pkg/policy"
	"go.uber.org/zap"
)

type gatewayLogging struct {
	gateway minio.Gateway
	log     *zap.Logger
}

// Logging returns a wrapper of minio.Gateway that logs errors before returning them.
func Logging(gateway minio.Gateway, log *zap.Logger) minio.Gateway {
	return &gatewayLogging{gateway, log}
}

func (lg *gatewayLogging) Name() string     { return lg.gateway.Name() }
func (lg *gatewayLogging) Production() bool { return lg.gateway.Production() }
func (lg *gatewayLogging) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
	layer, err := lg.gateway.NewGatewayLayer(creds)
	return &layerLogging{layer: layer, logger: lg.log}, err
}

type layerLogging struct {
	layer  minio.ObjectLayer
	logger *zap.Logger
}

// minioError checks if the given error is a minio error.
func minioError(err error) bool {
	return reflect.TypeOf(err).ConvertibleTo(reflect.TypeOf(minio.GenericError{}))
}

// log unexpected errors, i.e. non-minio errors. It will return the given error
// to allow method chaining.
func (log *layerLogging) log(err error) error {
	if err != nil && !minioError(err) {
		log.logger.Error("gateway error:", zap.Error(err))
	}
	return err
}

func (log *layerLogging) Shutdown(ctx context.Context) error {
	return log.log(log.layer.Shutdown(ctx))
}

func (log *layerLogging) StorageInfo(ctx context.Context) minio.StorageInfo {
	return log.layer.StorageInfo(ctx)
}

func (log *layerLogging) MakeBucketWithLocation(ctx context.Context, bucket string, location string) error {
	return log.log(log.layer.MakeBucketWithLocation(ctx, bucket, location))
}

func (log *layerLogging) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	bucketInfo, err = log.layer.GetBucketInfo(ctx, bucket)
	return bucketInfo, log.log(err)
}

func (log *layerLogging) ListBuckets(ctx context.Context) (buckets []minio.BucketInfo, err error) {
	buckets, err = log.layer.ListBuckets(ctx)
	return buckets, log.log(err)
}

func (log *layerLogging) DeleteBucket(ctx context.Context, bucket string) error {
	return log.log(log.layer.DeleteBucket(ctx, bucket))
}

func (log *layerLogging) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	result, err = log.layer.ListObjects(ctx, bucket, prefix, marker, delimiter,
		maxKeys)
	return result, log.log(err)
}

func (log *layerLogging) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	result, err = log.layer.ListObjectsV2(ctx, bucket, prefix, continuationToken, delimiter, maxKeys, fetchOwner, startAfter)
	return result, log.log(err)
}

func (log *layerLogging) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	return log.log(log.layer.GetObject(ctx, bucket, object, startOffset, length, writer, etag))
}

func (log *layerLogging) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	objInfo, err = log.layer.GetObjectInfo(ctx, bucket, object)
	return objInfo, log.log(err)
}

func (log *layerLogging) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {
	objInfo, err = log.layer.PutObject(ctx, bucket, object, data, metadata)
	return objInfo, log.log(err)
}

func (log *layerLogging) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	objInfo, err = log.layer.CopyObject(ctx, srcBucket, srcObject, destBucket, destObject, srcInfo)
	return objInfo, log.log(err)
}

func (log *layerLogging) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	return log.log(log.layer.DeleteObject(ctx, bucket, object))
}

func (log *layerLogging) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result minio.ListMultipartsInfo, err error) {
	result, err = log.layer.ListMultipartUploads(ctx, bucket, prefix, keyMarker, uploadIDMarker, delimiter, maxUploads)
	return result, log.log(err)
}

func (log *layerLogging) NewMultipartUpload(ctx context.Context, bucket, object string, metadata map[string]string) (uploadID string, err error) {
	uploadID, err = log.layer.NewMultipartUpload(ctx, bucket, object, metadata)
	return uploadID, log.log(err)
}

func (log *layerLogging) CopyObjectPart(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string, partID int, startOffset int64, length int64, srcInfo minio.ObjectInfo) (info minio.PartInfo, err error) {
	info, err = log.layer.CopyObjectPart(ctx, srcBucket, srcObject, destBucket, destObject, uploadID, partID, startOffset, length, srcInfo)
	return info, log.log(err)
}

func (log *layerLogging) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *hash.Reader) (info minio.PartInfo, err error) {
	info, err = log.layer.PutObjectPart(ctx, bucket, object, uploadID, partID, data)
	return info, log.log(err)
}

func (log *layerLogging) ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int) (result minio.ListPartsInfo, err error) {
	result, err = log.layer.ListObjectParts(ctx, bucket, object, uploadID,
		partNumberMarker, maxParts)
	return result, log.log(err)
}

func (log *layerLogging) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error {
	return log.log(log.layer.AbortMultipartUpload(ctx, bucket, object, uploadID))
}

func (log *layerLogging) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []minio.CompletePart) (objInfo minio.ObjectInfo, err error) {
	objInfo, err = log.layer.CompleteMultipartUpload(ctx, bucket, object, uploadID, uploadedParts)
	return objInfo, log.log(err)
}

func (log *layerLogging) ReloadFormat(ctx context.Context, dryRun bool) error {
	return log.log(log.layer.ReloadFormat(ctx, dryRun))
}

func (log *layerLogging) HealFormat(ctx context.Context, dryRun bool) (madmin.HealResultItem, error) {
	rv, err := log.layer.HealFormat(ctx, dryRun)
	return rv, log.log(err)
}

func (log *layerLogging) HealBucket(ctx context.Context, bucket string, dryRun bool) ([]madmin.HealResultItem, error) {
	rv, err := log.layer.HealBucket(ctx, bucket, dryRun)
	return rv, log.log(err)
}

func (log *layerLogging) HealObject(ctx context.Context, bucket, object string, dryRun bool) (madmin.HealResultItem, error) {
	rv, err := log.layer.HealObject(ctx, bucket, object, dryRun)
	return rv, log.log(err)
}

func (log *layerLogging) ListBucketsHeal(ctx context.Context) (buckets []minio.BucketInfo, err error) {
	buckets, err = log.layer.ListBucketsHeal(ctx)
	return buckets, log.log(err)
}

func (log *layerLogging) ListObjectsHeal(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (minio.ListObjectsInfo, error) {
	rv, err := log.layer.ListObjectsHeal(ctx, bucket, prefix, marker, delimiter, maxKeys)
	return rv, log.log(err)
}

func (log *layerLogging) ListLocks(ctx context.Context, bucket, prefix string, duration time.Duration) ([]minio.VolumeLockInfo, error) {
	rv, err := log.layer.ListLocks(ctx, bucket, prefix, duration)
	return rv, log.log(err)
}

func (log *layerLogging) ClearLocks(ctx context.Context, lockInfos []minio.VolumeLockInfo) error {
	return log.log(log.layer.ClearLocks(ctx, lockInfos))
}

func (log *layerLogging) SetBucketPolicy(ctx context.Context, n string, p *policy.Policy) error {
	return log.log(log.layer.SetBucketPolicy(ctx, n, p))
}

func (log *layerLogging) GetBucketPolicy(ctx context.Context, n string) (*policy.Policy, error) {
	p, err := log.layer.GetBucketPolicy(ctx, n)
	return p, log.log(err)
}

func (log *layerLogging) DeleteBucketPolicy(ctx context.Context, n string) error {
	return log.log(log.layer.DeleteBucketPolicy(ctx, n))
}

func (log *layerLogging) IsNotificationSupported() bool {
	return log.layer.IsNotificationSupported()
}

func (log *layerLogging) IsEncryptionSupported() bool {
	return log.layer.IsEncryptionSupported()
}
