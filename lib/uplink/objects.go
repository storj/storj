// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"encoding/hex"
	"io"
	"strings"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/hash"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
	"storj.io/storj/pkg/utils"
)

// ObjectMeta represents metadata about a specific Object
type ObjectMeta struct {
	Bucket   string
	Path     storj.Path
	IsPrefix bool

	Metadata map[string]string

	Created  time.Time
	Modified time.Time
	Expires  time.Time

	Size     int64
	Checksum string

	// this differs from storj.Object by not having Version (yet), and not
	// having a Stream embedded. I'm also not sold on splitting ContentType out
	// from Metadata but shrugemoji.
}

// GetObject returns a handle to the data for an object and its metadata, if
// authorized.
func (s *Session) GetObject(ctx context.Context, bucket string, path storj.Path) (
	ranger.Ranger, ObjectMeta, error) {

	return nil, ObjectMeta{}, nil
}

// ObjectPutOpts controls options about uploading a new Object, if authorized.
type ObjectPutOpts struct {
	Metadata map[string]string
	Expires  time.Time

	// the satellite should probably tell the uplink what to use for these
	// per bucket. also these should probably be denormalized and defined here.
	RS            *storj.RedundancyScheme
	NodeSelection *overlay.NodeSelectionConfig
}

// Upload uploads a new object, if authorized.
func (s *Session) Upload(ctx context.Context, bucket string, path storj.Path,
	data io.Reader, opts ObjectPutOpts) error {
	panic("TODO")
}

// DeleteObject removes an object, if authorized.
func (s *Session) DeleteObject(ctx context.Context, bucket string,
	path storj.Path) error {
	panic("TODO")
}

// ListObjectsField numbers the fields of list objects
type ListObjectsField int

const (
	// ListObjectsMetaNone opts
	ListObjectsMetaNone ListObjectsField = 0
	// ListObjectsMetaModified opts
	ListObjectsMetaModified ListObjectsField = 1 << iota
	// ListObjectsMetaExpiration opts
	ListObjectsMetaExpiration ListObjectsField = 1 << iota
	// ListObjectsMetaSize opts
	ListObjectsMetaSize ListObjectsField = 1 << iota
	// ListObjectsMetaChecksum opts
	ListObjectsMetaChecksum ListObjectsField = 1 << iota
	// ListObjectsMetaUserDefined opts
	ListObjectsMetaUserDefined ListObjectsField = 1 << iota
	// ListObjectsMetaAll opts
	ListObjectsMetaAll ListObjectsField = 1 << iota
)

// ListObjectsConfig holds params for listing objects with the Gateway
type ListObjectsConfig struct {
	// this differs from storj.ListOptions by removing the Delimiter field
	// (ours is hardcoded as "/"), and adding the Fields field to optionally
	// support efficient listing that doesn't require looking outside of the
	// path index in pointerdb.

	Prefix    storj.Path
	Cursor    storj.Path
	Recursive bool
	Direction storj.ListDirection
	Limit     int
	Fields    ListObjectsFields
}

// ListObjectsFields is an interface that I haven't figured out yet
type ListObjectsFields interface{}

// ListObjects lists objects a user is authorized to see.
func (s *Session) ListObjects(ctx context.Context, bucket string,
	cfg ListObjectsConfig) (items []ObjectMeta, more bool, err error) {

	// TODO: wire up ListObjectsV2

	// s.Gateway.ListObjectsV2(bucket, cfg.Prefix, "/", cfg.Limit)
	panic("TODO")
}

// DeleteObject deletes an object from the store and returns an error if there were any problems
func (client *Client) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = client.metainfo.DeleteObject(ctx, bucket, object)

	return convertError(err, bucket, object)
}

// GetObject retrieves a single object from the store and downloads it
func (client *Client) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer mon.Task()(&ctx)(&err)

	readOnlyStream, err := client.metainfo.GetObjectStream(ctx, bucket, object)
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

	download := stream.NewDownload(ctx, readOnlyStream, client.streams)
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

// GetObjectInfo returns the info for the object in question or an error
func (client *Client) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	obj, err := client.metainfo.GetObject(ctx, bucket, object)
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

// ListObjects returns a list of objects in a given Bucket
func (client *Client) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if delimiter != "" && delimiter != "/" {
		return minio.ListObjectsInfo{}, minio.UnsupportedDelimiter{Delimiter: delimiter}
	}

	startAfter := marker
	recursive := delimiter == ""

	var objects []minio.ObjectInfo
	var prefixes []string

	list, err := client.metainfo.ListObjects(ctx, bucket, storj.ListOptions{
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
func (client *Client) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
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

	list, err := client.metainfo.ListObjects(ctx, bucket, storj.ListOptions{
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

// CopyObject copies an object
func (client *Client) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	readOnlyStream, err := client.metainfo.GetObjectStream(ctx, srcBucket, srcObject)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, srcBucket, srcObject)
	}

	download := stream.NewDownload(ctx, readOnlyStream, client.streams)
	defer func() { err = errs.Combine(err, download.Close()) }()

	info := readOnlyStream.Info()
	createInfo := storj.CreateObject{
		ContentType:      info.ContentType,
		Expires:          info.Expires,
		Metadata:         info.Metadata,
		RedundancyScheme: info.RedundancyScheme,
		EncryptionScheme: info.EncryptionScheme,
	}

	return client.putObject(ctx, destBucket, destObject, download, &createInfo)
}

func (client *Client) putObject(ctx context.Context, bucket, object string, reader io.Reader, createInfo *storj.CreateObject) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	mutableObject, err := client.metainfo.CreateObject(ctx, bucket, object, createInfo)
	if err != nil {
		return minio.ObjectInfo{}, convertError(err, bucket, object)
	}

	err = upload(ctx, client.streams, mutableObject, reader)
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

	return utils.CombineErrors(err, upload.Close())
}

// PutObject starts an upload
func (client *Client) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	contentType := metadata["content-type"]
	delete(metadata, "content-type")

	createInfo := storj.CreateObject{
		ContentType:      contentType,
		Metadata:         metadata,
		RedundancyScheme: client.redundancy,
		EncryptionScheme: client.encryption,
	}

	return client.putObject(ctx, bucket, object, data, &createInfo)
}

// Shutdown registers with monkit and then returns nil.
func (client *Client) Shutdown(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// StorageInfo returns storage info
func (client *Client) StorageInfo(context.Context) minio.StorageInfo {
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
