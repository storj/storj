// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/uplink"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj Gateway error")
)

// NewStorjGateway creates a new Storj S3 gateway.
func NewStorjGateway(project *uplink.Project) *Gateway {
	return &Gateway{
		project:   project,
		multipart: NewMultipartUploads(),
	}
}

// Gateway is the implementation of a minio cmd.Gateway
type Gateway struct {
	project   *uplink.Project
	multipart *MultipartUploads
}

// Name implements cmd.Gateway
func (gateway *Gateway) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (gateway *Gateway) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
	return &gatewayLayer{gateway: gateway}, nil
}

// Production implements cmd.Gateway
func (gateway *Gateway) Production() bool {
	return false
}

type gatewayLayer struct {
	minio.GatewayUnsupported
	gateway *Gateway
}

func (layer *gatewayLayer) DeleteBucket(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = layer.gateway.project.DeleteBucket(ctx, bucketName)

	return convertError(err, bucketName, "")
}

func (layer *gatewayLayer) DeleteObject(ctx context.Context, bucketName, objectPath string) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return convertError(err, bucketName, objectPath)
	}

	_, err = layer.gateway.project.DeleteObject(ctx, bucketName, objectPath)

	return convertError(err, bucketName, objectPath)
}

func (layer *gatewayLayer) GetBucketInfo(ctx context.Context, bucketName string) (bucketInfo minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.StatBucket(ctx, bucketName)

	if err != nil {
		return minio.BucketInfo{}, convertError(err, bucketName, "")
	}

	return minio.BucketInfo{
		Name:    bucket.Name,
		Created: bucket.Created,
	}, nil
}

func (layer *gatewayLayer) GetObject(ctx context.Context, bucketName, objectPath string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return convertError(err, bucketName, objectPath)
	}

	download, err := layer.gateway.project.DownloadObject(ctx, bucketName, objectPath, &uplink.DownloadOptions{
		Offset: startOffset,
		Length: length,
	})
	if err != nil {
		return convertError(err, bucketName, objectPath)
	}
	defer func() { err = errs.Combine(err, download.Close()) }()

	object := download.Info()
	if startOffset < 0 || length < -1 || startOffset+length > object.Standard.ContentLength {
		return minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    startOffset + length,
			ResourceSize: object.Standard.ContentLength,
		}
	}

	_, err = io.Copy(writer, download)

	return err
}

func (layer *gatewayLayer) GetObjectInfo(ctx context.Context, bucketName, objectPath string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	object, err := layer.gateway.project.StatObject(ctx, bucketName, objectPath)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	return minioObjectInfo(bucketName, "", object), nil
}

func (layer *gatewayLayer) ListBuckets(ctx context.Context) (items []minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	buckets := layer.gateway.project.ListBuckets(ctx, nil)
	for buckets.Next() {
		info := buckets.Item()
		items = append(items, minio.BucketInfo{
			Name:    info.Name,
			Created: info.Created,
		})
	}
	if buckets.Err() != nil {
		return nil, buckets.Err()
	}
	return items, nil
}

func (layer *gatewayLayer) ListObjects(ctx context.Context, bucketName, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO maybe this should be checked by project.ListObjects
	if bucketName == "" {
		return minio.ListObjectsInfo{}, minio.BucketNameInvalid{}
	}

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsInfo{}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return result, convertError(err, bucketName, "")
	}

	list := layer.gateway.project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{
		Prefix:    prefix,
		Cursor:    marker,
		Recursive: delimiter == "",

		Info:     true,
		Standard: true,
		Custom:   true,
	})

	startAfter := marker
	var objects []minio.ObjectInfo
	var prefixes []string

	limit := maxKeys
	for (limit > 0 || maxKeys == 0) && list.Next() {
		limit--
		object := list.Item()
		if object.IsPrefix {
			prefixes = append(prefixes, object.Key)
			continue
		}

		objects = append(objects, minioObjectInfo(bucketName, "", object))

		startAfter = object.Key

	}
	if list.Err() != nil {
		return result, convertError(list.Err(), bucketName, "")
	}

	more := list.Next()
	if list.Err() != nil {
		return result, convertError(list.Err(), bucketName, "")
	}

	result = minio.ListObjectsInfo{
		IsTruncated: more,
		Objects:     objects,
		Prefixes:    prefixes,
	}
	if more {
		result.NextMarker = startAfter
	}

	return result, nil
}

func (layer *gatewayLayer) ListObjectsV2(ctx context.Context, bucketName, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, convertError(err, bucketName, "")
	}

	recursive := delimiter == ""

	var startAfterPath storj.Path
	if continuationToken != "" {
		startAfterPath = continuationToken
	}
	if startAfterPath == "" && startAfter != "" {
		startAfterPath = startAfter
	}

	var objects []minio.ObjectInfo
	var prefixes []string

	list := layer.gateway.project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{
		Prefix:    prefix,
		Cursor:    startAfterPath,
		Recursive: recursive,

		Info:     true,
		Standard: true,
		Custom:   true,
	})

	limit := maxKeys
	for (limit > 0 || maxKeys == 0) && list.Next() {
		limit--
		object := list.Item()
		if object.IsPrefix {
			prefixes = append(prefixes, object.Key)
			continue
		}

		objects = append(objects, minioObjectInfo(bucketName, "", object))

		startAfter = object.Key
	}
	if list.Err() != nil {
		return result, convertError(list.Err(), bucketName, "")
	}

	more := list.Next()
	if list.Err() != nil {
		return result, convertError(list.Err(), bucketName, "")
	}

	result = minio.ListObjectsV2Info{
		IsTruncated:       more,
		ContinuationToken: startAfter,
		Objects:           objects,
		Prefixes:          prefixes,
	}
	if more {
		result.NextContinuationToken = startAfter
	}

	return result, nil
}

func (layer *gatewayLayer) MakeBucketWithLocation(ctx context.Context, bucketName string, location string) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: maybe this should return an error since we don't support locations

	_, err = layer.gateway.project.CreateBucket(ctx, bucketName)

	return convertError(err, bucketName, "")
}

func (layer *gatewayLayer) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if srcObject == "" {
		return minio.ObjectInfo{}, minio.ObjectNameInvalid{Bucket: srcBucket}
	}
	if destObject == "" {
		return minio.ObjectInfo{}, minio.ObjectNameInvalid{Bucket: destBucket}
	}

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, srcBucket)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, "")
	}

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, destBucket)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, destBucket, "")
	}

	download, err := layer.gateway.project.DownloadObject(ctx, srcBucket, srcObject, nil)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, srcObject)
	}
	defer func() {
		// TODO: this hides minio error
		err = errs.Combine(err, download.Close())
	}()

	upload, err := layer.gateway.project.UploadObject(ctx, destBucket, destObject, nil)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, destBucket, destObject)
	}

	info := download.Info()
	err = upload.SetMetadata(ctx, &info.Standard, info.Custom)
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return minio.ObjectInfo{}, convertError(err, destBucket, destObject)
	}

	reader, err := hash.NewReader(download, info.Standard.ContentLength, "", "")
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return minio.ObjectInfo{}, convertError(err, destBucket, destObject)
	}

	_, err = io.Copy(upload, reader)
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return minio.ObjectInfo{}, convertError(err, destBucket, destObject)
	}

	err = upload.Commit()
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, destBucket, destObject)
	}

	return minioObjectInfo(destBucket, hex.EncodeToString(reader.MD5Current()), upload.Info()), nil
}

func (layer *gatewayLayer) PutObject(ctx context.Context, bucketName, objectPath string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO this should be removed and implemented on satellite side
	_, err = layer.gateway.project.StatBucket(ctx, bucketName)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	if data == nil {
		data, err = hash.NewReader(bytes.NewReader([]byte{}), 0, "", "")
		if err != nil {
			return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
		}
	}

	upload, err := layer.gateway.project.UploadObject(ctx, bucketName, objectPath, nil)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	n, err := io.Copy(upload, data)
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	contentType := metadata["content-type"]
	delete(metadata, "content-type")

	err = upload.SetMetadata(ctx, &uplink.StandardMetadata{
		ContentLength: n,
		ContentType:   contentType,
	}, metadata)
	if err != nil {
		abortErr := upload.Abort()
		err = errs.Combine(err, abortErr)
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	err = upload.Commit()
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}

	return minioObjectInfo(bucketName, data.MD5HexString(), upload.Info()), nil
}

func (layer *gatewayLayer) Shutdown(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

func (layer *gatewayLayer) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}

func convertError(err error, bucket, object string) error {
	if uplink.ErrBucketNameInvalid.Has(err) {
		return minio.BucketNameInvalid{Bucket: bucket}
	}

	if uplink.ErrBucketAlreadyExists.Has(err) {
		return minio.BucketAlreadyExists{Bucket: bucket}
	}

	if uplink.ErrBucketNotFound.Has(err) {
		return minio.BucketNotFound{Bucket: bucket}
	}

	if uplink.ErrBucketNotEmpty.Has(err) {
		return minio.BucketNotEmpty{Bucket: bucket}
	}

	if uplink.ErrObjectKeyInvalid.Has(err) {
		return minio.ObjectNameInvalid{Bucket: bucket, Object: object}
	}

	if uplink.ErrObjectNotFound.Has(err) {
		return minio.ObjectNotFound{Bucket: bucket, Object: object}
	}

	return err
}

func minioObjectInfo(bucket, etag string, object *uplink.Object) minio.ObjectInfo {
	return minio.ObjectInfo{
		Bucket:      bucket,
		Name:        object.Key,
		Size:        object.Standard.ContentLength,
		ETag:        etag,
		ModTime:     object.Info.Created,
		ContentType: object.Standard.ContentType,
		UserDefined: object.Custom,
	}
}
