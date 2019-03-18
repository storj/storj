// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/storj"
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
	Path storj.Cipher
}
