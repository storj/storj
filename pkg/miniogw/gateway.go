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

	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
)

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj Gateway error")
)

// NewStorjGateway creates a *Storj object from an existing ObjectStore
func NewStorjGateway(metainfo storj.Metainfo, streams streams.Store, pathCipher storj.Cipher, encryption storj.EncryptionScheme, redundancy storj.RedundancyScheme) *Gateway {
	return &Gateway{
		metainfo:   metainfo,
		streams:    streams,
		pathCipher: pathCipher,
		encryption: encryption,
		redundancy: redundancy,
		multipart:  NewMultipartUploads(),
	}
}

// Gateway is the implementation of a minio cmd.Gateway
type Gateway struct {
	metainfo   storj.Metainfo
	streams    streams.Store
	pathCipher storj.Cipher
	encryption storj.EncryptionScheme
	redundancy storj.RedundancyScheme
	multipart  *MultipartUploads
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

func (layer *gatewayLayer) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	list, err := layer.gateway.metainfo.ListObjects(ctx, bucket, storj.ListOptions{Direction: storj.After, Recursive: true, Limit: 1})
	if err != nil {
		return convertError(err, bucket, "")
	}

	if len(list.Items) > 0 {
		return minio.BucketNotEmpty{Bucket: bucket}
	}

	err = layer.gateway.metainfo.DeleteBucket(ctx, bucket)

	return convertError(err, bucket, "")
}

func (layer *gatewayLayer) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = layer.gateway.metainfo.DeleteObject(ctx, bucket, object)

	return convertError(err, bucket, object)
}

func (layer *gatewayLayer) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := layer.gateway.metainfo.GetBucket(ctx, bucket)

	if err != nil {
		return minio.BucketInfo{}, convertError(err, bucket, "")
	}

	return minio.BucketInfo{Name: info.Name, Created: info.Created}, nil
}

func (layer *gatewayLayer) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)

	readOnlyStream, err := layer.gateway.metainfo.GetObjectStream(ctx, bucket, object)
	if err != nil {
		return convertError(err, bucket, object)
	}

	if startOffset < 0 || length < -1 || startOffset+length > readOnlyStream.Info().Size {
		return minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    startOffset + length,
			ResourceSize: readOnlyStream.Info().Size,
		}
	}

	download := stream.NewDownload(ctx, readOnlyStream, layer.gateway.streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	_, err = download.Seek(startOffset, io.SeekStart)
	if err != nil {
		return err
	}

	if length == -1 {
		_, err = io.Copy(writer, download)
	} else {
		_, err = io.CopyN(writer, download, length)
	}

	return err
}

func (layer *gatewayLayer) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	obj, err := layer.gateway.metainfo.GetObject(ctx, bucket, object)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucket, object)
	}

	return minio.ObjectInfo{
		Name:        object,
		Bucket:      bucket,
		ModTime:     obj.Modified,
		Size:        obj.Size,
		ETag:        hex.EncodeToString(obj.Checksum),
		ContentType: obj.ContentType,
		UserDefined: obj.Metadata,
	}, err
}

func (layer *gatewayLayer) ListBuckets(ctx context.Context) (bucketItems []minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	startAfter := ""

	for {
		list, err := layer.gateway.metainfo.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After, Cursor: startAfter})
		if err != nil {
			return nil, err
		}

		for _, item := range list.Items {
			bucketItems = append(bucketItems, minio.BucketInfo{Name: item.Name, Created: item.Created})
		}

		if !list.More {
			break
		}

		startAfter = list.Items[len(list.Items)-1].Name
	}

	return bucketItems, err
}

func (layer *gatewayLayer) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsInfo{}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	startAfter := marker
	recursive := delimiter == ""

	var objects []minio.ObjectInfo
	var prefixes []string

	list, err := layer.gateway.metainfo.ListObjects(ctx, bucket, storj.ListOptions{
		Direction: storj.After,
		Cursor:    startAfter,
		Prefix:    prefix,
		Recursive: recursive,
		Limit:     maxKeys,
	})
	if err != nil {
		return result, convertError(err, bucket, "")
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
				Bucket:      bucket,
				IsDir:       false,
				Name:        path,
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
func (layer *gatewayLayer) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

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

	list, err := layer.gateway.metainfo.ListObjects(ctx, bucket, storj.ListOptions{
		Direction: storj.After,
		Cursor:    startAfterPath,
		Prefix:    prefix,
		Recursive: recursive,
		Limit:     maxKeys,
	})
	if err != nil {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, convertError(err, bucket, "")
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
				Bucket:      bucket,
				IsDir:       false,
				Name:        path,
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

func (layer *gatewayLayer) MakeBucketWithLocation(ctx context.Context, bucket string, location string) (err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: This current strategy of calling bs.Get
	// to check if a bucket exists, then calling bs.Put
	// if not, can create a race condition if two people
	// call MakeBucketWithLocation at the same time and
	// therefore try to Put a bucket at the same time.
	// The reason for the Get call to check if the
	// bucket already exists is to match S3 CLI behavior.
	_, err = layer.gateway.metainfo.GetBucket(ctx, bucket)
	if err == nil {
		return minio.BucketAlreadyExists{Bucket: bucket}
	}

	if !storj.ErrBucketNotFound.Has(err) {
		return convertError(err, bucket, "")
	}

	_, err = layer.gateway.metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: layer.gateway.pathCipher})

	return err
}

func (layer *gatewayLayer) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	readOnlyStream, err := layer.gateway.metainfo.GetObjectStream(ctx, srcBucket, srcObject)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, srcObject)
	}

	download := stream.NewDownload(ctx, readOnlyStream, layer.gateway.streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	info := readOnlyStream.Info()
	createInfo := storj.CreateObject{
		ContentType:      info.ContentType,
		Expires:          info.Expires,
		Metadata:         info.Metadata,
		RedundancyScheme: info.RedundancyScheme,
		EncryptionScheme: info.EncryptionScheme,
	}

	return layer.putObject(ctx, destBucket, destObject, download, &createInfo)
}

func (layer *gatewayLayer) putObject(ctx context.Context, bucket, object string, reader io.Reader, createInfo *storj.CreateObject) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	mutableObject, err := layer.gateway.metainfo.CreateObject(ctx, bucket, object, createInfo)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucket, object)
	}

	err = upload(ctx, layer.gateway.streams, mutableObject, reader)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	err = mutableObject.Commit(ctx)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	info := mutableObject.Info()

	return minio.ObjectInfo{
		Name:        object,
		Bucket:      bucket,
		ModTime:     info.Modified,
		Size:        info.Size,
		ETag:        hex.EncodeToString(info.Checksum),
		ContentType: info.ContentType,
		UserDefined: info.Metadata,
	}, nil
}

func upload(ctx context.Context, streams streams.Store, mutableObject storj.MutableObject, reader io.Reader) error {
	mutableStream, err := mutableObject.CreateStream(ctx)
	if err != nil {
		return err
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)

	_, err = io.Copy(upload, reader)

	return errs.Combine(err, upload.Close())
}

func (layer *gatewayLayer) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	contentType := metadata["content-type"]
	delete(metadata, "content-type")

	createInfo := storj.CreateObject{
		ContentType:      contentType,
		Metadata:         metadata,
		RedundancyScheme: layer.gateway.redundancy,
		EncryptionScheme: layer.gateway.encryption,
	}

	return layer.putObject(ctx, bucket, object, data, &createInfo)
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
