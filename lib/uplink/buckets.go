// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"

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

// CreateBucketOptions holds the bucket opts
type CreateBucketOptions struct {
	PathCipher storj.Cipher
	EncConfig  ul.EncryptionConfig // EncConfig is the default encryption configuration to create buckets with
	// this differs from storj.CreateBucket's choice of just using storj.Bucket
	// by not having 2/3 unsettable fields.
}

// GetBucket returns info about the requested bucket if authorized
func (s *Session) GetBucket(ctx context.Context, bucket string) (storj.Bucket,
	error) {

	// info, err := s.Gateway.GetBucketInfo(ctx, bucket)
	// if err != nil {
	// 	return storj.Bucket{}, err
	// }

	// fmt.Printf("bucket info: %+v\n", *info)

	// TODO: Wire up info to bucket
	return storj.Bucket{}, nil
}

// CreateBucket creates a new bucket if authorized
func (s *Session) CreateBucket(ctx context.Context, bucket string,
	opts *CreateBucketOptions) (storj.Bucket, error) {

	// s.Gateway.MakeBucketWithLocation(ctx, )

	return storj.Bucket{}, nil
}

// DeleteBucket deletes a bucket if authorized
func (s *Session) DeleteBucket(ctx context.Context, bucket string) error {
	return errors.New("Not implemented")
}

// ListBuckets will list authorized buckets
func (s *Session) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (
	storj.BucketList, error) {
	return storj.BucketList{}, nil
}
