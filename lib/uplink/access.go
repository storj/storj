// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"storj.io/storj/pkg/storj"
)

// Access holds a reference to Uplink and a set of permissions for actions on a bucket.
type Access struct {
	Permissions Permissions
	Uplink      *Uplink
}

// A Macaroon represents an access credential to certain resources
type Macaroon interface {
	Serialize() ([]byte, error)
	Restrict(caveats ...Caveat) Macaroon
}

// Permissions are parsed by Uplink and return an Access struct
type Permissions struct {
	Macaroon Macaroon
	APIKey   string
}

// Caveat could be a read-only restriction, a time-bound
// restriction, a bucket-specific restriction, a path-prefix restriction, a
// full path restriction, etc.
type Caveat interface {
}

// CreateBucketOptions holds the bucket opts
type CreateBucketOptions struct {
	Encryption Encryption
}

// CreateBucket creates a bucket from the passed opts
func (a *Access) CreateBucket(ctx context.Context, bucket string, opts CreateBucketOptions) (storj.Bucket, error) {
	metainfo, _, err := a.Uplink.config.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	return metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: opts.Encryption.PathCipher})
}

// DeleteBucket deletes a bucket if authorized
func (a *Access) DeleteBucket(ctx context.Context, bucket string) error {
	metainfo, _, err := a.Uplink.config.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return Error.Wrap(err)
	}

	return metainfo.DeleteBucket(ctx, bucket)
}

// ListBuckets will list authorized buckets
func (a *Access) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (storj.BucketList, error) {
	metainfo, _, err := a.Uplink.config.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.BucketList{}, Error.Wrap(err)
	}

	return metainfo.ListBuckets(ctx, opts)
}

// GetBucketInfo returns info about the requested bucket if authorized
func (a *Access) GetBucketInfo(ctx context.Context, bucket string) (storj.Bucket, error) {
	metainfo, _, err := a.Uplink.config.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.Bucket{}, Error.Wrap(err)
	}

	return metainfo.GetBucket(ctx, bucket)
}

// GetBucket returns a Bucket with the given Encryption information
func (a *Access) GetBucket(ctx context.Context, bucket string, encryption *Encryption) *Bucket {
	return &Bucket{
		Access: a,
		Enc:    encryption,
		Bucket: storj.Bucket{
			Name:       bucket,
			PathCipher: encryption.PathCipher,
		},
	}
}
