// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"encoding/hex"
	"io"
	"strings"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/uplink/metainfo/kvmetainfo"
	"storj.io/storj/uplink/storage/streams"
	"storj.io/storj/uplink/stream"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj Gateway error")
)

// NewStorjGateway creates a *Storj object from an existing ObjectStore
func NewStorjGateway(project *uplink.Project, access *uplink.EncryptionAccess, pathCipher storj.CipherSuite, encryption storj.EncryptionParameters, redundancy storj.RedundancyScheme, segmentSize memory.Size) *Gateway {
	return &Gateway{
		project:     project,
		access:      access,
		pathCipher:  pathCipher,
		encryption:  encryption,
		redundancy:  redundancy,
		segmentSize: segmentSize,
		multipart:   NewMultipartUploads(),
	}
}

// Gateway is the implementation of a minio cmd.Gateway
type Gateway struct {
	project     *uplink.Project
	access      *uplink.EncryptionAccess
	pathCipher  storj.CipherSuite
	encryption  storj.EncryptionParameters
	redundancy  storj.RedundancyScheme
	segmentSize memory.Size
	multipart   *MultipartUploads
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

	empty, err := layer.bucketEmpty(ctx, bucketName)
	if err != nil {
		return convertError(err, bucketName, "")
	}

	if !empty {
		return minio.BucketNotEmpty{Bucket: bucketName}
	}

	err = layer.gateway.project.DeleteBucket(ctx, bucketName)

	return convertError(err, bucketName, "")
}

func (layer *gatewayLayer) bucketEmpty(ctx context.Context, bucketName string) (empty bool, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return false, convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	list, err := bucket.ListObjects(ctx, &storj.ListOptions{Direction: storj.After, Recursive: true, Limit: 1})
	if err != nil {
		return false, convertError(err, bucketName, "")
	}

	return len(list.Items) == 0, nil
}

func (layer *gatewayLayer) DeleteObject(ctx context.Context, bucketName, objectPath string) (err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	err = bucket.DeleteObject(ctx, objectPath)

	return convertError(err, bucketName, objectPath)
}

func (layer *gatewayLayer) GetBucketInfo(ctx context.Context, bucketName string) (bucketInfo minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, _, err := layer.gateway.project.GetBucketInfo(ctx, bucketName)

	if err != nil {
		return minio.BucketInfo{}, convertError(err, bucketName, "")
	}

	return minio.BucketInfo{Name: bucket.Name, Created: bucket.Created}, nil
}

func (layer *gatewayLayer) GetObject(ctx context.Context, bucketName, objectPath string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	object, err := bucket.OpenObject(ctx, objectPath)
	if err != nil {
		return convertError(err, bucketName, objectPath)
	}
	defer func() { err = errs.Combine(err, object.Close()) }()

	if startOffset < 0 || length < -1 || startOffset+length > object.Meta.Size {
		return minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    startOffset + length,
			ResourceSize: object.Meta.Size,
		}
	}

	reader, err := object.DownloadRange(ctx, startOffset, length)
	if err != nil {
		return convertError(err, bucketName, objectPath)
	}
	defer func() { err = errs.Combine(err, reader.Close()) }()

	_, err = io.Copy(writer, reader)

	return err
}

func (layer *gatewayLayer) GetObjectInfo(ctx context.Context, bucketName, objectPath string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	object, err := bucket.OpenObject(ctx, objectPath)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}
	defer func() { err = errs.Combine(err, object.Close()) }()

	return minio.ObjectInfo{
		Name:        object.Meta.Path,
		Bucket:      object.Meta.Bucket,
		ModTime:     object.Meta.Modified,
		Size:        object.Meta.Size,
		ETag:        hex.EncodeToString(object.Meta.Checksum),
		ContentType: object.Meta.ContentType,
		UserDefined: object.Meta.Metadata,
	}, err
}

func (layer *gatewayLayer) ListBuckets(ctx context.Context) (bucketItems []minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	startAfter := ""

	listOpts := storj.BucketListOptions{
		Direction: storj.Forward,
		Cursor:    startAfter,
	}
	for {
		list, err := layer.gateway.project.ListBuckets(ctx, &listOpts)
		if err != nil {
			return nil, err
		}

		for _, item := range list.Items {
			bucketItems = append(bucketItems, minio.BucketInfo{Name: item.Name, Created: item.Created})
		}

		if !list.More {
			break
		}

		listOpts = listOpts.NextPage(list)
	}

	return bucketItems, err
}

func (layer *gatewayLayer) ListObjects(ctx context.Context, bucketName, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsInfo{}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return minio.ListObjectsInfo{}, convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	startAfter := marker
	recursive := delimiter == ""

	var objects []minio.ObjectInfo
	var prefixes []string

	list, err := bucket.ListObjects(ctx, &storj.ListOptions{
		Direction: storj.After,
		Cursor:    startAfter,
		Prefix:    prefix,
		Recursive: recursive,
		Limit:     maxKeys,
	})
	if err != nil {
		return result, convertError(err, bucketName, "")
	}

	if len(list.Items) > 0 {
		for _, item := range list.Items {
			path := item.Path
			if recursive && prefix != "" {
				path = storj.JoinPaths(strings.TrimSuffix(prefix, "/"), path)
			}
			if item.IsPrefix {
				prefixes = append(prefixes, path)
				continue
			}
			objects = append(objects, minio.ObjectInfo{
				Name:        path,
				Bucket:      item.Bucket.Name,
				ModTime:     item.Modified,
				Size:        item.Size,
				ETag:        hex.EncodeToString(item.Checksum),
				ContentType: item.ContentType,
				UserDefined: item.Metadata,
			})
		}
		startAfter = list.Items[len(list.Items)-1].Path
	}

	result = minio.ListObjectsInfo{
		IsTruncated: list.More,
		Objects:     objects,
		Prefixes:    prefixes,
	}
	if list.More {
		result.NextMarker = startAfter
	}

	return result, err
}

// ListObjectsV2 - Not implemented stub
func (layer *gatewayLayer) ListObjectsV2(ctx context.Context, bucketName, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return minio.ListObjectsV2Info{}, convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	recursive := delimiter == ""
	var nextContinuationToken string

	var startAfterPath storj.Path
	if continuationToken != "" {
		startAfterPath = continuationToken
	}
	if startAfterPath == "" && startAfter != "" {
		startAfterPath = startAfter
	}

	var objects []minio.ObjectInfo
	var prefixes []string

	list, err := bucket.ListObjects(ctx, &storj.ListOptions{
		Direction: storj.After,
		Cursor:    startAfterPath,
		Prefix:    prefix,
		Recursive: recursive,
		Limit:     maxKeys,
	})
	if err != nil {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, convertError(err, bucketName, "")
	}

	if len(list.Items) > 0 {
		for _, item := range list.Items {
			path := item.Path
			if recursive && prefix != "" {
				path = storj.JoinPaths(strings.TrimSuffix(prefix, "/"), path)
			}
			if item.IsPrefix {
				prefixes = append(prefixes, path)
				continue
			}
			objects = append(objects, minio.ObjectInfo{
				Name:        path,
				Bucket:      item.Bucket.Name,
				ModTime:     item.Modified,
				Size:        item.Size,
				ETag:        hex.EncodeToString(item.Checksum),
				ContentType: item.ContentType,
				UserDefined: item.Metadata,
			})
		}

		nextContinuationToken = list.Items[len(list.Items)-1].Path + "\x00"
	}

	result = minio.ListObjectsV2Info{
		IsTruncated:       list.More,
		ContinuationToken: continuationToken,
		Objects:           objects,
		Prefixes:          prefixes,
	}
	if list.More {
		result.NextContinuationToken = nextContinuationToken
	}

	return result, err
}

func (layer *gatewayLayer) MakeBucketWithLocation(ctx context.Context, bucketName string, location string) (err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: This current strategy of calling bs.Get
	// to check if a bucket exists, then calling bs.Put
	// if not, can create a race condition if two people
	// call MakeBucketWithLocation at the same time and
	// therefore try to Put a bucket at the same time.
	// The reason for the Get call to check if the
	// bucket already exists is to match S3 CLI behavior.
	_, _, err = layer.gateway.project.GetBucketInfo(ctx, bucketName)
	if err == nil {
		return minio.BucketAlreadyExists{Bucket: bucketName}
	}

	if !storj.ErrBucketNotFound.Has(err) {
		return convertError(err, bucketName, "")
	}

	cfg := uplink.BucketConfig{
		PathCipher:           layer.gateway.pathCipher,
		EncryptionParameters: layer.gateway.encryption,
	}
	cfg.Volatile.RedundancyScheme = layer.gateway.redundancy
	cfg.Volatile.SegmentsSize = layer.gateway.segmentSize

	_, err = layer.gateway.project.CreateBucket(ctx, bucketName, &cfg)

	return err
}

func (layer *gatewayLayer) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, srcBucket, layer.gateway.access)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	object, err := bucket.OpenObject(ctx, srcObject)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, srcObject)
	}
	defer func() { err = errs.Combine(err, object.Close()) }()

	reader, err := object.DownloadRange(ctx, 0, -1)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, srcObject)
	}
	defer func() { err = errs.Combine(err, reader.Close()) }()

	opts := uplink.UploadOptions{
		ContentType: object.Meta.ContentType,
		Metadata:    object.Meta.Metadata,
		Expires:     object.Meta.Expires,
	}
	opts.Volatile.EncryptionParameters = object.Meta.Volatile.EncryptionParameters
	opts.Volatile.RedundancyScheme = object.Meta.Volatile.RedundancyScheme

	return layer.putObject(ctx, destBucket, destObject, reader, &opts)
}

func (layer *gatewayLayer) putObject(ctx context.Context, bucketName, objectPath string, reader io.Reader, opts *uplink.UploadOptions) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, err := layer.gateway.project.OpenBucket(ctx, bucketName, layer.gateway.access)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, "")
	}
	defer func() { err = errs.Combine(err, bucket.Close()) }()

	err = bucket.UploadObject(ctx, objectPath, reader, opts)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, "")
	}

	object, err := bucket.OpenObject(ctx, objectPath)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucketName, objectPath)
	}
	defer func() { err = errs.Combine(err, object.Close()) }()

	return minio.ObjectInfo{
		Name:        object.Meta.Path,
		Bucket:      object.Meta.Bucket,
		ModTime:     object.Meta.Modified,
		Size:        object.Meta.Size,
		ETag:        hex.EncodeToString(object.Meta.Checksum),
		ContentType: object.Meta.ContentType,
		UserDefined: object.Meta.Metadata,
	}, nil
}

func upload(ctx context.Context, streams streams.Store, mutableObject kvmetainfo.MutableObject, reader io.Reader) error {
	mutableStream, err := mutableObject.CreateStream(ctx)
	if err != nil {
		return err
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)

	_, err = io.Copy(upload, reader)

	return errs.Wrap(errs.Combine(err, upload.Close()))
}

func (layer *gatewayLayer) PutObject(ctx context.Context, bucketName, objectPath string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	contentType := metadata["content-type"]
	delete(metadata, "content-type")

	opts := uplink.UploadOptions{
		ContentType: contentType,
		Metadata:    metadata,
	}

	return layer.putObject(ctx, bucketName, objectPath, data, &opts)
}

func (layer *gatewayLayer) Shutdown(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

func (layer *gatewayLayer) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}

func convertError(err error, bucket, object string) error {
	if storj.ErrNoBucket.Has(err) {
		return minio.BucketNameInvalid{Bucket: bucket}
	}

	if storj.ErrBucketNotFound.Has(err) {
		return minio.BucketNotFound{Bucket: bucket}
	}

	if storj.ErrNoPath.Has(err) {
		return minio.ObjectNameInvalid{Bucket: bucket, Object: object}
	}

	if storj.ErrObjectNotFound.Has(err) {
		return minio.ObjectNotFound{Bucket: bucket, Object: object}
	}

	return err
}
