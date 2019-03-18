// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/storj"
	ul "storj.io/storj/uplink"
)

// BucketOpts holds the cipher, path, key, and enc. scheme for each bucket since they
// can be different for each
type BucketOpts struct {
	PathCipher       storj.Cipher
	EncPathPrefix    storj.Path
	Key              storj.Key
	EncryptionScheme storj.EncryptionScheme
}

// Bucket is a struct that allows operations on a Bucket after a user providers Permissions
type Bucket struct {
	Access *Access
}

// CreateBucketOptions holds the bucket opts
type CreateBucketOptions struct {
	PathCipher storj.Cipher
	EncConfig  ul.EncryptionConfig // EncConfig is the default encryption configuration to create buckets with
	// this differs from storj.CreateBucket's choice of just using storj.Bucket
	// by not having 2/3 unsettable fields.
}

// OLD CODE STARTS HERE
// // GetBucket returns info about the requested bucket if authorized
// func (s *Session) GetBucket(ctx context.Context, bucket string) (storj.Bucket,
// 	error) {

// 	// info, err := s.Gateway.GetBucketInfo(ctx, bucket)
// 	// if err != nil {
// 	// 	return storj.Bucket{}, err
// 	// }

// 	// fmt.Printf("bucket info: %+v\n", *info)

// 	// TODO: Wire up info to bucket
// 	return storj.Bucket{}, nil
// }

// // CreateBucket creates a new bucket if authorized
// func (s *Session) CreateBucket(ctx context.Context, bucket string,
// 	opts *CreateBucketOptions) (storj.Bucket, error) {

// 	// s.Gateway.MakeBucketWithLocation(ctx, )

// 	return storj.Bucket{}, nil
// }

// // DeleteBucket deletes a bucket if authorized
// func (s *Session) DeleteBucket(ctx context.Context, bucket string) error {
// 	return errors.New("Not implemented")
// }

// // ListBuckets will list authorized buckets
// func (s *Session) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (
// 	storj.BucketList, error) {
// 	return storj.BucketList{}, nil
// }

// // DeleteBucket deletes a bucket and returns an error if there was a problem
// func (client *Client) DeleteBucket(ctx context.Context, bucket string) (err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	list, err := client.metainfo.ListObjects(ctx, bucket, storj.ListOptions{Direction: storj.After, Recursive: true, Limit: 1})
// 	if err != nil {
// 		return convertError(err, bucket, "")
// 	}

// 	if len(list.Items) > 0 {
// 		return minio.BucketNotEmpty{Bucket: bucket}
// 	}

// 	err = client.metainfo.DeleteBucket(ctx, bucket)

// 	return convertError(err, bucket, "")
// }

// // GetBucketInfo gets the bucket information and returns the info and an error type
// func (client *Client) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	info, err := client.metainfo.GetBucket(ctx, bucket)

// 	if err != nil {
// 		return minio.BucketInfo{}, convertError(err, bucket, "")
// 	}

// 	return minio.BucketInfo{Name: info.Name, Created: info.Created}, nil
// }

// // ListBuckets returns a list of buckets in the store
// func (client *Client) ListBuckets(ctx context.Context) (bucketItems []minio.BucketInfo, err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	startAfter := ""

// 	for {
// 		list, err := client.metainfo.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After, Cursor: startAfter})
// 		if err != nil {
// 			return nil, err
// 		}

// 		for _, item := range list.Items {
// 			bucketItems = append(bucketItems, minio.BucketInfo{Name: item.Name, Created: item.Created})
// 		}

// 		if !list.More {
// 			break
// 		}

// 		startAfter = list.Items[len(list.Items)-1].Name
// 	}

// 	return bucketItems, err
// }

// // MakeBucketWithLocation makes a bucket with a location and returns an error if there were any problems.
// func (client *Client) MakeBucketWithLocation(ctx context.Context, bucket string, location string) (err error) {
// 	defer mon.Task()(&ctx)(&err)
// 	// TODO: This current strategy of calling bs.Get
// 	// to check if a bucket exists, then calling bs.Put
// 	// if not, can create a race condition if two people
// 	// call MakeBucketWithLocation at the same time and
// 	// therefore try to Put a bucket at the same time.
// 	// The reason for the Get call to check if the
// 	// bucket already exists is to match S3 CLI behavior.
// 	_, err = client.metainfo.GetBucket(ctx, bucket)
// 	if err == nil {
// 		return minio.BucketAlreadyExists{Bucket: bucket}
// 	}

// 	if !storj.ErrBucketNotFound.Has(err) {
// 		return convertError(err, bucket, "")
// 	}

// 	_, err = client.metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: client.pathCipher})

// 	return err
// }
