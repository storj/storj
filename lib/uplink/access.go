// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"

	"storj.io/storj/pkg/storj"
)

// Access is all of the access information an application needs to store and
// retrieve data. Someone with a share may have no restrictions within a project
// (can create buckets, list buckets, list files, upload files, delete files,
// etc), may be restricted to a single bucket, may be restricted to a prefix
// within a bucket, or may even be restricted to a single file within a bucket.
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
}

// Caveat could be a read-only restriction, a time-bound
// restriction, a bucket-specific restriction, a path-prefix restriction, a
// full path restriction, etc.
type Caveat interface {
}

// ParseAccess parses a serialized Access
func ParseAccess(data []byte) (Access, error) {
	return Access{}, errors.New("not implemented")
}

// Serialize serializes an Access message
func (a *Access) Serialize() ([]byte, error) {
	return []byte{}, errors.New("not implemented")
}

// CreateBucket creates a bucket from the passed opts
func (a *Access) CreateBucket(ctx context.Context, bucket string, opts CreateBucketOptions) (storj.Bucket, error) {
	cfg := a.Uplink.config
	metainfo, _, err := cfg.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.Bucket{}, nil
	}

	encScheme := cfg.GetEncryptionScheme()

	created, err := metainfo.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: encScheme.Cipher})
	if err != nil {
		return storj.Bucket{}, err
	}
	return created, nil
}

// DeleteBucket deletes a bucket if authorized
func (a *Access) DeleteBucket(ctx context.Context, bucket string) error {
	cfg := a.Uplink.config
	metainfo, _, err := cfg.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return err
	}

	return metainfo.DeleteBucket(ctx, bucket)
}

// ListBuckets will list authorized buckets
func (a *Access) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (storj.BucketList, error) {
	cfg := a.Uplink.config
	metainfo, _, err := cfg.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.BucketList{}, err
	}

	return metainfo.ListBuckets(ctx, opts)
}

// GetBucketInfo returns info about the requested bucket if authorized
func (a *Access) GetBucketInfo(ctx context.Context, bucket string) (storj.Bucket, error) {
	config := a.Uplink.config
	metainfo, _, err := config.GetMetainfo(ctx, a.Uplink.id)
	if err != nil {
		return storj.Bucket{}, err
	}

	b, err := metainfo.GetBucket(ctx, bucket)
	if err != nil {
		return storj.Bucket{}, err
	}

	return b, nil
}

// GetBucket returns a Bucket with the given Encryption information
func (a *Access) GetBucket(ctx context.Context, bucket string, encryption storj.EncryptionScheme) *Bucket {
	opts := &BucketOpts{
		PathCipher:       encryption.Cipher,
		EncryptionScheme: a.Uplink.config.GetEncryptionScheme(),
	}
	return &Bucket{
		Access: a,
		Opts:   opts,
	}
}
