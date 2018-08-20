// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package logging

import (
	context "context"
	io "io"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/minio/minio/pkg/madmin"
	"github.com/minio/minio/pkg/policy"
	"go.uber.org/zap"
)

var errTemplate = "gateway error: %+v"

type gwLogWrap struct {
	gw minio.Gateway
}

// Gateway is a wrapper of minio.Gateway that logs errors before
// returning them.
func Gateway(gw minio.Gateway) minio.Gateway {
	return &gwLogWrap{gw: gw}
}

func (lg *gwLogWrap) Name() string     { return lg.gw.Name() }
func (lg *gwLogWrap) Production() bool { return lg.gw.Production() }
func (lg *gwLogWrap) NewGatewayLayer(creds auth.Credentials) (
	minio.ObjectLayer, error) {
	ol, err := lg.gw.NewGatewayLayer(creds)
	return &olLogWrap{ol: ol, logger: zap.S()}, err
}

type olLogWrap struct {
	ol     minio.ObjectLayer
	logger ErrorLogger
}

// ErrorLogger logs a templated error message
type ErrorLogger interface {
	Errorf(template string, args ...interface{})
}

func (ol *olLogWrap) wrap(err error) error {
	if err != nil {
		ol.logger.Errorf(errTemplate, err)
	}
	return err
}

func (ol *olLogWrap) Shutdown(ctx context.Context) error {
	return ol.wrap(ol.ol.Shutdown(ctx))
}

func (ol *olLogWrap) StorageInfo(ctx context.Context) minio.StorageInfo {
	return ol.ol.StorageInfo(ctx)
}

func (ol *olLogWrap) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	err := ol.ol.MakeBucketWithLocation(ctx, bucket, location)
	if _, ok := err.(minio.BucketAlreadyExists); ok {
		return err
	}
	return ol.wrap(err)
}

func (ol *olLogWrap) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo minio.BucketInfo, err error) {
	bucketInfo, err = ol.ol.GetBucketInfo(ctx, bucket)
	return bucketInfo, ol.wrap(err)
}

func (ol *olLogWrap) ListBuckets(ctx context.Context) (
	buckets []minio.BucketInfo, err error) {
	buckets, err = ol.ol.ListBuckets(ctx)
	return buckets, ol.wrap(err)
}

func (ol *olLogWrap) DeleteBucket(ctx context.Context, bucket string) error {
	err := ol.ol.DeleteBucket(ctx, bucket)
	if _, ok := err.(minio.BucketNotEmpty); ok {
		return err
	}
	if _, ok := err.(minio.BucketNotFound); ok {
		return err
	}
	return ol.wrap(err)
}

func (ol *olLogWrap) ListObjects(ctx context.Context,
	bucket, prefix, marker, delimiter string, maxKeys int) (
	result minio.ListObjectsInfo, err error) {
	result, err = ol.ol.ListObjects(ctx, bucket, prefix, marker, delimiter,
		maxKeys)
	return result, ol.wrap(err)
}

func (ol *olLogWrap) ListObjectsV2(ctx context.Context,
	bucket, prefix, continuationToken, delimiter string, maxKeys int,
	fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info,
	err error) {
	result, err = ol.ol.ListObjectsV2(ctx, bucket, prefix, continuationToken,
		delimiter, maxKeys, fetchOwner, startAfter)
	return result, ol.wrap(err)
}

func (ol *olLogWrap) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	return ol.wrap(ol.ol.GetObject(ctx, bucket, object, startOffset, length,
		writer, etag))
}

func (ol *olLogWrap) GetObjectInfo(ctx context.Context, bucket, object string) (
	objInfo minio.ObjectInfo, err error) {
	objInfo, err = ol.ol.GetObjectInfo(ctx, bucket, object)
	if _, ok := err.(minio.ObjectNotFound); ok {
		return objInfo, err
	}
	return objInfo, ol.wrap(err)
}

func (ol *olLogWrap) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo,
	err error) {
	objInfo, err = ol.ol.PutObject(ctx, bucket, object, data, metadata)
	return objInfo, ol.wrap(err)
}

func (ol *olLogWrap) CopyObject(ctx context.Context,
	srcBucket, srcObject, destBucket, destObject string,
	srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	objInfo, err = ol.ol.CopyObject(ctx, srcBucket, srcObject, destBucket,
		destObject, srcInfo)
	return objInfo, ol.wrap(err)
}

func (ol *olLogWrap) DeleteObject(ctx context.Context, bucket, object string) (
	err error) {
	return ol.wrap(ol.ol.DeleteObject(ctx, bucket, object))
}

func (ol *olLogWrap) ListMultipartUploads(ctx context.Context,
	bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (
	result minio.ListMultipartsInfo, err error) {
	result, err = ol.ol.ListMultipartUploads(ctx, bucket, prefix, keyMarker,
		uploadIDMarker, delimiter, maxUploads)
	return result, ol.wrap(err)
}

func (ol *olLogWrap) NewMultipartUpload(ctx context.Context,
	bucket, object string, metadata map[string]string) (uploadID string,
	err error) {
	uploadID, err = ol.ol.NewMultipartUpload(ctx, bucket, object, metadata)
	return uploadID, ol.wrap(err)
}

func (ol *olLogWrap) CopyObjectPart(ctx context.Context,
	srcBucket, srcObject, destBucket, destObject string, uploadID string,
	partID int, startOffset int64, length int64, srcInfo minio.ObjectInfo) (
	info minio.PartInfo, err error) {
	info, err = ol.ol.CopyObjectPart(ctx, srcBucket, srcObject, destBucket,
		destObject, uploadID, partID, startOffset, length, srcInfo)
	return info, ol.wrap(err)
}

func (ol *olLogWrap) PutObjectPart(ctx context.Context,
	bucket, object, uploadID string, partID int, data *hash.Reader) (
	info minio.PartInfo, err error) {
	info, err = ol.ol.PutObjectPart(ctx, bucket, object, uploadID, partID, data)
	return info, ol.wrap(err)
}

func (ol *olLogWrap) ListObjectParts(ctx context.Context,
	bucket, object, uploadID string, partNumberMarker int, maxParts int) (
	result minio.ListPartsInfo, err error) {
	result, err = ol.ol.ListObjectParts(ctx, bucket, object, uploadID,
		partNumberMarker, maxParts)
	return result, ol.wrap(err)
}

func (ol *olLogWrap) AbortMultipartUpload(ctx context.Context,
	bucket, object, uploadID string) error {
	return ol.wrap(ol.ol.AbortMultipartUpload(ctx, bucket, object, uploadID))
}

func (ol *olLogWrap) CompleteMultipartUpload(ctx context.Context,
	bucket, object, uploadID string, uploadedParts []minio.CompletePart) (
	objInfo minio.ObjectInfo, err error) {
	objInfo, err = ol.ol.CompleteMultipartUpload(ctx, bucket, object, uploadID,
		uploadedParts)
	return objInfo, ol.wrap(err)
}

func (ol *olLogWrap) ReloadFormat(ctx context.Context, dryRun bool) error {
	return ol.wrap(ol.ol.ReloadFormat(ctx, dryRun))
}

func (ol *olLogWrap) HealFormat(ctx context.Context, dryRun bool) (
	madmin.HealResultItem, error) {
	rv, err := ol.ol.HealFormat(ctx, dryRun)
	return rv, ol.wrap(err)
}

func (ol *olLogWrap) HealBucket(ctx context.Context, bucket string,
	dryRun bool) ([]madmin.HealResultItem, error) {
	rv, err := ol.ol.HealBucket(ctx, bucket, dryRun)
	return rv, ol.wrap(err)
}

func (ol *olLogWrap) HealObject(ctx context.Context, bucket, object string,
	dryRun bool) (madmin.HealResultItem, error) {
	rv, err := ol.ol.HealObject(ctx, bucket, object, dryRun)
	return rv, ol.wrap(err)
}

func (ol *olLogWrap) ListBucketsHeal(ctx context.Context) (
	buckets []minio.BucketInfo, err error) {
	buckets, err = ol.ol.ListBucketsHeal(ctx)
	return buckets, ol.wrap(err)
}

func (ol *olLogWrap) ListObjectsHeal(ctx context.Context,
	bucket, prefix, marker, delimiter string, maxKeys int) (
	minio.ListObjectsInfo, error) {
	rv, err := ol.ol.ListObjectsHeal(ctx, bucket, prefix, marker, delimiter,
		maxKeys)
	return rv, ol.wrap(err)
}

func (ol *olLogWrap) ListLocks(ctx context.Context, bucket, prefix string,
	duration time.Duration) ([]minio.VolumeLockInfo, error) {
	rv, err := ol.ol.ListLocks(ctx, bucket, prefix, duration)
	return rv, ol.wrap(err)
}

func (ol *olLogWrap) ClearLocks(ctx context.Context,
	lockInfos []minio.VolumeLockInfo) error {
	return ol.wrap(ol.ol.ClearLocks(ctx, lockInfos))
}

func (ol *olLogWrap) SetBucketPolicy(ctx context.Context, n string,
	p *policy.Policy) error {
	return ol.wrap(ol.ol.SetBucketPolicy(ctx, n, p))
}

func (ol *olLogWrap) GetBucketPolicy(ctx context.Context, n string) (
	*policy.Policy, error) {
	p, err := ol.ol.GetBucketPolicy(ctx, n)
	return p, ol.wrap(err)
}

func (ol *olLogWrap) DeleteBucketPolicy(ctx context.Context, n string) error {
	return ol.wrap(ol.ol.DeleteBucketPolicy(ctx, n))
}

func (ol *olLogWrap) IsNotificationSupported() bool {
	return ol.ol.IsNotificationSupported()
}

func (ol *olLogWrap) IsEncryptionSupported() bool {
	return ol.ol.IsEncryptionSupported()
}
