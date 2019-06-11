// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"strings"

	"github.com/zeebo/errs"
)

//
// To avoid confusion about when paths are encrypted, unencrypted, or contain a
// bucket prefix, we create some wrapper types so that the compiler will complain
// if someone attempts to use one in the wrong context.
//

// Unencrypted is an opaque type representing an unencrypted path.
type Unencrypted struct {
	raw string
}

// Encrypted is an opaque type representing an encrypted path.
type Encrypted struct {
	raw string
}

// UnencryptedBucket is an opaque type representing a bucket and unencrypted path.
type UnencryptedBucket struct {
	bucket string
	path   Unencrypted
}

// EncryptedBucket is an opaque type representing a bucket and encrypted path.
type EncryptedBucket struct {
	bucket string
	path   Encrypted
}

//
// unencrypted paths
//

// NewUnencrypted takes a raw unencrypted path and returns it wrapped.
func NewUnencrypted(raw string) Unencrypted {
	return Unencrypted{raw: raw}
}

// Raw returns the original raw path for the Unencrypted.
func (path Unencrypted) Raw() string { return path.raw }

// String returns a human readable form of the Unencrypted.
func (path Unencrypted) String() string { return "up:" + path.Raw() }

// Consume attempts to remove the prefix from the Unencrypted, and reports true
// if it was successful.
func (path Unencrypted) Consume(prefix Unencrypted) (Unencrypted, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewUnencrypted(path.raw[len(prefix.raw):]), true
	}
	return path, false
}

// WithBucket associates the bucket name with the Unencrypted.
func (path Unencrypted) WithBucket(bucket string) UnencryptedBucket {
	return UnencryptedBucket{
		bucket: bucket,
		path:   path,
	}
}

// Iterator returns an iterator over the components of the Unencrypted.
func (path Unencrypted) Iterator() Iterator {
	return Iterator{raw: path.raw}
}

//
// encrypted path
//

// NewEncrypted takes a raw encrypted path and returns it wrapped.
func NewEncrypted(raw string) Encrypted {
	return Encrypted{raw: raw}
}

// Raw returns the original path for the Encrypted.
func (path Encrypted) Raw() string { return path.raw }

// String returns a human readable form of the Encrypted.
func (path Encrypted) String() string { return "ep:" + path.Raw() }

// Consume attempts to remove the prefix from the Encrypted, and reports true
// if it was successful.
func (path Encrypted) Consume(prefix Encrypted) (Encrypted, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewEncrypted(path.raw[len(prefix.raw):]), true
	}
	return path, false
}

// WithBucket associates the bucket name with the Encrypted.
func (path Encrypted) WithBucket(bucket string) EncryptedBucket {
	return EncryptedBucket{
		bucket: bucket,
		path:   path,
	}
}

// Iterator returns an iterator over the components of the Encrypted.
func (path Encrypted) Iterator() Iterator {
	return Iterator{raw: path.raw}
}

//
// unencrypted paths with bucket
//

// ParseUnencryptedBucket parses a raw string into a bucket and unencrypted path.
func ParseUnencryptedBucket(raw string) (UnencryptedBucket, error) {
	parts := strings.SplitN(raw, "/", 1)
	if len(parts) != 2 {
		return UnencryptedBucket{}, errs.New("invalid unencrypted bucket path: %q", raw)
	}
	return NewUnencrypted(parts[1]).WithBucket(parts[0]), nil
}

// Raw returns the original bucket path pair for the UnencryptedBucket.
func (ubp UnencryptedBucket) Raw() string { return ubp.bucket + "/" + ubp.path.Raw() }

// String returns a human readable form of the UnencryptedBucket.
func (ubp UnencryptedBucket) String() string { return "ubp:" + ubp.Raw() }

// Bucket returns the bucket associated with the UnencryptedBucket.
func (ubp UnencryptedBucket) Bucket() string { return ubp.bucket }

// Path returns the Unencrypted associated with the UnencryptedBucket.
func (ubp UnencryptedBucket) Path() Unencrypted { return ubp.path }

//
// encrypted paths with bucket
//

// ParseEncryptedBucket parses a raw string into a bucket and encrypted path.
func ParseEncryptedBucket(raw string) (EncryptedBucket, error) {
	parts := strings.SplitN(raw, "/", 1)
	if len(parts) != 2 {
		return EncryptedBucket{}, errs.New("invalid encrypted bucket path: %q", raw)
	}
	return NewEncrypted(parts[1]).WithBucket(parts[0]), nil
}

// Raw returns the original bucket path pair for the EncryptedBucket.
func (ebp EncryptedBucket) Raw() string { return ebp.bucket + "/" + ebp.path.Raw() }

// String returns a human readable form of the UnencryptedBucket.
func (ebp EncryptedBucket) String() string { return "ebp:" + ebp.Raw() }

// Bucket returns the bucket associated with the EncryptedBucket.
func (ebp EncryptedBucket) Bucket() string { return ebp.bucket }

// Path returns the Encrypted associated with the EncryptedBucket.
func (ebp EncryptedBucket) Path() Encrypted { return ebp.path }

//
// path component iteration
//

// Iterator allows one to efficiently iterate over components of a path.
type Iterator struct {
	raw       string
	consumed  int
	lastEmpty bool
}

// Consumed reports how much of the path has been consumed.
func (pi Iterator) Consumed() string { return pi.raw[:pi.consumed] }

// Remaining reports how much of the path is remaining.
func (pi Iterator) Remaining() string { return pi.raw[pi.consumed:] }

// Done reports if the path has been fully consumed.
func (pi Iterator) Done() bool { return len(pi.raw) == pi.consumed && !pi.lastEmpty }

// Next returns the first component of the path, consuming it.
func (pi *Iterator) Next() string {
	rem := pi.Remaining()
	if index := strings.IndexByte(rem, '/'); index == -1 {
		pi.consumed += len(rem)
		pi.lastEmpty = false
		return rem
	} else {
		pi.consumed += index + 1
		pi.lastEmpty = index == len(rem)-1
		return rem[:index]
	}
}
