// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"io"
	"strings"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj Gateway error")
)

// NewStorjGateway creates a *Storj object from an existing ObjectStore
func NewStorjGateway(bs buckets.Store, pathCipher storj.Cipher) *Storj {
	return &Storj{bs: bs, pathCipher: pathCipher, multipart: NewMultipartUploads()}
}

//Storj is the implementation of a minio cmd.Gateway
type Storj struct {
	bs         buckets.Store
	pathCipher storj.Cipher
	multipart  *MultipartUploads
}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	minio.ObjectLayer, error) {
	return &storjObjects{storj: s}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

type storjObjects struct {
	minio.GatewayUnsupported
	storj *Storj
}

func (s *storjObjects) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return convertBucketNotFoundError(err, bucket)
	}

	items, _, err := o.List(ctx, "", "", "", true, 1, meta.None)
	if err != nil {
		return err
	}

	if len(items) > 0 {
		return minio.BucketNotEmpty{Bucket: bucket}
	}

	err = s.storj.bs.Delete(ctx, bucket)

	return convertBucketNotFoundError(err, bucket)
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)

	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return convertBucketNotFoundError(err, bucket)
	}

	err = o.Delete(ctx, object)

	return convertObjectNotFoundError(err, bucket, object)
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	meta, err := s.storj.bs.Get(ctx, bucket)

	if err != nil {
		return minio.BucketInfo{}, convertBucketNotFoundError(err, bucket)
	}

	return minio.BucketInfo{Name: bucket, Created: meta.Created}, nil
}

func (s *storjObjects) getObject(ctx context.Context, bucket, object string) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return nil, convertBucketNotFoundError(err, bucket)
	}

	rr, _, err = o.Get(ctx, object)

	return rr, convertObjectNotFoundError(err, bucket, object)
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)

	rr, err := s.getObject(ctx, bucket, object)
	if err != nil {
		return err
	}

	if length == -1 {
		length = rr.Size() - startOffset
	}

	r, err := rr.Range(ctx, startOffset, length)
	if err != nil {
		return err
	}
	defer utils.LogClose(r)

	_, err = io.Copy(writer, r)

	return err
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ObjectInfo{}, convertBucketNotFoundError(err, bucket)
	}

	m, err := o.Meta(ctx, object)
	if err != nil {
		return objInfo, convertObjectNotFoundError(err, bucket, object)
	}

	return minio.ObjectInfo{
		Name:        object,
		Bucket:      bucket,
		ModTime:     m.Modified,
		Size:        m.Size,
		ETag:        m.Checksum,
		ContentType: m.ContentType,
		UserDefined: m.UserDefined,
	}, err
}

func (s *storjObjects) ListBuckets(ctx context.Context) (bucketItems []minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	startAfter := ""
	var items []buckets.ListItem

	for {
		moreItems, more, err := s.storj.bs.List(ctx, startAfter, "", 0)
		if err != nil {
			return nil, err
		}
		items = append(items, moreItems...)
		if !more {
			break
		}
		startAfter = moreItems[len(moreItems)-1].Bucket
	}

	bucketItems = make([]minio.BucketInfo, len(items))
	for i, item := range items {
		bucketItems[i].Name = item.Bucket
		bucketItems[i].Created = item.Meta.Created
	}

	return bucketItems, err
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsInfo{}, Error.New("delimiter %s not supported", delimiter)
	}

	startAfter := marker
	recursive := delimiter == ""

	var objects []minio.ObjectInfo
	var prefixes []string
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ListObjectsInfo{}, convertBucketNotFoundError(err, bucket)
	}
	items, more, err := o.List(ctx, prefix, startAfter, "", recursive, maxKeys, meta.All)
	if err != nil {
		return result, err
	}
	if len(items) > 0 {
		for _, item := range items {
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
				ModTime:     item.Meta.Modified,
				Size:        item.Meta.Size,
				ContentType: item.Meta.ContentType,
				UserDefined: item.Meta.UserDefined,
				ETag:        item.Meta.Checksum,
			})
		}
		startAfter = items[len(items)-1].Path
	}

	result = minio.ListObjectsInfo{
		IsTruncated: more,
		Objects:     objects,
		Prefixes:    prefixes,
	}
	if more {
		result.NextMarker = startAfter
	}

	return result, err
}

// ListObjectsV2 - Not implemented stub
func (s *storjObjects) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, Error.New("delimiter %s not supported", delimiter)
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
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ListObjectsV2Info{ContinuationToken: continuationToken}, convertBucketNotFoundError(err, bucket)
	}
	items, more, err := o.List(ctx, prefix, startAfterPath, "", recursive, maxKeys, meta.All)
	if err != nil {
		return result, err
	}

	if len(items) > 0 {
		for _, item := range items {
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
				ModTime:     item.Meta.Modified,
				Size:        item.Meta.Size,
				ContentType: item.Meta.ContentType,
				UserDefined: item.Meta.UserDefined,
				ETag:        item.Meta.Checksum,
			})
		}

		nextContinuationToken = items[len(items)-1].Path + "\x00"
	}

	result = minio.ListObjectsV2Info{
		IsTruncated:       more,
		ContinuationToken: continuationToken,
		Objects:           objects,
		Prefixes:          prefixes,
	}
	if more {
		result.NextContinuationToken = nextContinuationToken
	}

	return result, err
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context, bucket string, location string) (err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: This current strategy of calling bs.Get
	// to check if a bucket exists, then calling bs.Put
	// if not, can create a race condition if two people
	// call MakeBucketWithLocation at the same time and
	// therefore try to Put a bucket at the same time.
	// The reason for the Get call to check if the
	// bucket already exists is to match S3 CLI behavior.
	_, err = s.storj.bs.Get(ctx, bucket)
	if err == nil {
		return minio.BucketAlreadyExists{Bucket: bucket}
	}
	if !storj.ErrBucketNotFound.Has(err) {
		return err
	}
	_, err = s.storj.bs.Put(ctx, bucket, s.storj.pathCipher)
	return err
}

func (s *storjObjects) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rr, err := s.getObject(ctx, srcBucket, srcObject)
	if err != nil {
		return objInfo, err
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return objInfo, err
	}

	defer utils.LogClose(r)

	serMetaInfo := pb.SerializableMeta{
		ContentType: srcInfo.ContentType,
		UserDefined: srcInfo.UserDefined,
	}

	return s.putObject(ctx, destBucket, destObject, r, serMetaInfo)
}

func (s *storjObjects) putObject(ctx context.Context, bucket, object string, r io.Reader, meta pb.SerializableMeta) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// setting zero value means the object never expires
	expTime := time.Time{}
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ObjectInfo{}, convertBucketNotFoundError(err, bucket)
	}
	m, err := o.Put(ctx, object, r, meta, expTime)
	return minio.ObjectInfo{
		Name:        object,
		Bucket:      bucket,
		ModTime:     m.Modified,
		Size:        m.Size,
		ETag:        m.Checksum,
		ContentType: m.ContentType,
		UserDefined: m.UserDefined,
	}, err
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {

	defer mon.Task()(&ctx)(&err)
	tempContType := metadata["content-type"]
	delete(metadata, "content-type")
	//metadata serialized
	serMetaInfo := pb.SerializableMeta{
		ContentType: tempContType,
		UserDefined: metadata,
	}

	return s.putObject(ctx, bucket, object, data, serMetaInfo)
}

func (s *storjObjects) Shutdown(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}

func convertBucketNotFoundError(err error, bucket string) error {
	if storj.ErrBucketNotFound.Has(err) {
		return minio.BucketNotFound{Bucket: bucket}
	}
	return err
}

func convertObjectNotFoundError(err error, bucket, object string) error {
	if storj.ErrObjectNotFound.Has(err) {
		return minio.ObjectNotFound{Bucket: bucket, Object: object}
	}
	return err
}
