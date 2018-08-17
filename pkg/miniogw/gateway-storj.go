// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"
	"io"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard End User Client errors
	Error = errs.Class("Storj Gateway error")
)

// NewStorjGateway creates a *Storj object from an existing ObjectStore
func NewStorjGateway(bs buckets.Store) *Storj {
	return &Storj{bs: bs}
}

//Storj is the implementation of a minio cmd.Gateway
type Storj struct {
	bs buckets.Store
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
	_, err = s.storj.bs.Get(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return minio.BucketNotFound{Bucket: bucket}
		}
		return err
	}
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return err
	}
	items, _, err := o.List(ctx, nil, nil, nil, true, 1, meta.None)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		return minio.BucketNotEmpty{Bucket: bucket}
	}
	return s.storj.bs.Delete(ctx, bucket)
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return err
	}
	return o.Delete(ctx, paths.New(object))
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo minio.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	meta, err := s.storj.bs.Get(ctx, bucket)
	if err != nil {
		return minio.BucketInfo{}, err
	}
	return minio.BucketInfo{Name: bucket, Created: meta.Created}, nil
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return err
	}
	rr, _, err := o.Get(ctx, paths.New(object))
	if err != nil {
		return err
	}
	defer utils.LogClose(rr)
	r, err := rr.Range(ctx, startOffset, length)
	if err != nil {
		return err
	}
	defer utils.LogClose(r)
	_, err = io.Copy(writer, r)
	return err
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket,
	object string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	m, err := o.Meta(ctx, paths.New(object))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return objInfo, minio.ObjectNotFound{
				Bucket: bucket,
				Object: object,
			}
		}

		return objInfo, err
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

func (s *storjObjects) ListBuckets(ctx context.Context) (
	bucketItems []minio.BucketInfo, err error) {
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

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	startAfter := paths.New(marker)
	var fl []minio.ObjectInfo
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ListObjectsInfo{}, err
	}
	items, more, err := o.List(ctx, paths.New(prefix), startAfter, nil, true, maxKeys, meta.All)
	if err != nil {
		return result, err
	}
	if len(items) > 0 {
		//Populate the objectlist (aka filelist)
		f := make([]minio.ObjectInfo, len(items))
		for i, fi := range items {
			f[i] = minio.ObjectInfo{
				Bucket:      bucket,
				Name:        fi.Path.String(),
				ModTime:     fi.Meta.Modified,
				Size:        fi.Meta.Size,
				ContentType: fi.Meta.ContentType,
				UserDefined: fi.Meta.UserDefined,
				ETag:        fi.Meta.Checksum,
			}
		}
		startAfter = items[len(items)-1].Path[len(paths.New(prefix)):]
		fl = f
	}

	result = minio.ListObjectsInfo{
		IsTruncated: more,
		Objects:     fl,
	}
	if more {
		result.NextMarker = startAfter.String()
	}

	return result, err
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) (err error) {
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
	if !storage.ErrKeyNotFound.Has(err) {
		return err
	}
	_, err = s.storj.bs.Put(ctx, bucket)
	return err
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo,
	err error) {
	defer mon.Task()(&ctx)(&err)
	tempContType := metadata["content-type"]
	delete(metadata, "content-type")
	//metadata serialized
	serMetaInfo := objects.SerializableMeta{
		ContentType: tempContType,
		UserDefined: metadata,
	}
	// setting zero value means the object never expires
	expTime := time.Time{}
	o, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	m, err := o.Put(ctx, paths.New(object), data, serMetaInfo, expTime)
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

func (s *storjObjects) Shutdown(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}
