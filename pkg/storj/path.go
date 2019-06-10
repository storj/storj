// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"strings"

	"github.com/zeebo/errs"
)

//
// To avoid confusion about when paths are encrypted, unencrypted, or contain a
// bucket prefix, we create some wrapper types so that the compiler will complain
// if someone attempts to use one in the wrong context.
//

// UnencryptedPath is an opaque type representing an unencrypted path.
type UnencryptedPath struct {
	raw string
}

// EncryptedPath is an opaque type representing an encrypted path.
type EncryptedPath struct {
	raw string
}

// UnencryptedBucketPath is an opaque type representing a bucket and unencrypted path.
type UnencryptedBucketPath struct {
	bucket string
	path   UnencryptedPath
}

// EncryptedBucketPath is an opaque type representing a bucket and encrypted path.
type EncryptedBucketPath struct {
	bucket string
	path   EncryptedPath
}

//
// unencrypted paths
//

// NewUnencryptedPath takes a raw unencrypted path and returns it wrapped.
func NewUnencryptedPath(raw string) UnencryptedPath {
	return UnencryptedPath{raw: raw}
}

// Raw returns the original raw path for the UnencryptedPath.
func (path UnencryptedPath) Raw() string { return path.raw }

// String returns a human readable form of the UnencryptedPath.
func (path UnencryptedPath) String() string { return "up:" + path.Raw() }

// Consume attempts to remove the prefix from the UnencryptedPath, and reports true
// if it was successful.
func (path UnencryptedPath) Consume(prefix UnencryptedPath) (UnencryptedPath, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewUnencryptedPath(path.raw[len(prefix.raw):]), true
	}
	return path, false
}

// WithBucket associates the bucket name with the UnencryptedPath.
func (path UnencryptedPath) WithBucket(bucket string) UnencryptedBucketPath {
	return UnencryptedBucketPath{
		bucket: bucket,
		path:   path,
	}
}

// Iterator returns an iterator over the components of the UnencryptedPath.
func (path UnencryptedPath) Iterator() PathIterator {
	return PathIterator{raw: path.raw}
}

//
// encrypted path
//

// NewEncryptedPath takes a raw encrypted path and returns it wrapped.
func NewEncryptedPath(raw string) EncryptedPath {
	return EncryptedPath{raw: raw}
}

// Raw returns the original path for the EncryptedPath.
func (path EncryptedPath) Raw() string { return path.raw }

// String returns a human readable form of the EncryptedPath.
func (path EncryptedPath) String() string { return "ep:" + path.Raw() }

// Consume attempts to remove the prefix from the EncryptedPath, and reports true
// if it was successful.
func (path EncryptedPath) Consume(prefix EncryptedPath) (EncryptedPath, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewEncryptedPath(path.raw[len(prefix.raw):]), true
	}
	return path, false
}

// WithBucket associates the bucket name with the EncryptedPath.
func (path EncryptedPath) WithBucket(bucket string) EncryptedBucketPath {
	return EncryptedBucketPath{
		bucket: bucket,
		path:   path,
	}
}

// Iterator returns an iterator over the components of the EncryptedPath.
func (path EncryptedPath) Iterator() PathIterator {
	return PathIterator{raw: path.raw}
}

//
// unencrypted paths with bucket
//

// ParseUnencryptedBucketPath parses a raw string into a bucket and unencrypted path.
func ParseUnencryptedBucketPath(raw string) (UnencryptedBucketPath, error) {
	parts := strings.SplitN(raw, "/", 1)
	if len(parts) != 2 {
		return UnencryptedBucketPath{}, errs.New("invalid unencrypted bucket path: %q", raw)
	}
	return NewUnencryptedPath(parts[1]).WithBucket(parts[0]), nil
}

// Raw returns the original bucket path pair for the UnencryptedBucketPath.
func (ubp UnencryptedBucketPath) Raw() string { return ubp.bucket + "/" + ubp.path.Raw() }

// String returns a human readable form of the UnencryptedBucketPath.
func (ubp UnencryptedBucketPath) String() string { return "ubp:" + ubp.Raw() }

// Bucket returns the bucket associated with the UnencryptedBucketPath.
func (ubp UnencryptedBucketPath) Bucket() string { return ubp.bucket }

// Path returns the UnencryptedPath associated with the UnencryptedBucketPath.
func (ubp UnencryptedBucketPath) Path() UnencryptedPath { return ubp.path }

//
// encrypted paths with bucket
//

// ParseEncryptedBucketPath parses a raw string into a bucket and encrypted path.
func ParseEncryptedBucketPath(raw string) (EncryptedBucketPath, error) {
	parts := strings.SplitN(raw, "/", 1)
	if len(parts) != 2 {
		return EncryptedBucketPath{}, errs.New("invalid encrypted bucket path: %q", raw)
	}
	return NewEncryptedPath(parts[1]).WithBucket(parts[0]), nil
}

// Raw returns the original bucket path pair for the EncryptedBucketPath.
func (ebp EncryptedBucketPath) Raw() string { return ebp.bucket + "/" + ebp.path.Raw() }

// String returns a human readable form of the UnencryptedBucketPath.
func (ebp EncryptedBucketPath) String() string { return "ebp:" + ebp.Raw() }

// Bucket returns the bucket associated with the EncryptedBucketPath.
func (ebp EncryptedBucketPath) Bucket() string { return ebp.bucket }

// Path returns the EncryptedPath associated with the EncryptedBucketPath.
func (ebp EncryptedBucketPath) Path() EncryptedPath { return ebp.path }

//
// path component iteration
//

// PathIterator allows one to efficiently iterate over components of a path.
type PathIterator struct {
	raw       string
	consumed  int
	lastEmpty bool
}

// Consumed reports how much of the path has been consumed.
func (pi PathIterator) Consumed() string { return pi.raw[:pi.consumed] }

// Remaining reports how much of the path is remaining.
func (pi PathIterator) Remaining() string { return pi.raw[pi.consumed:] }

// Done reports if the path has been fully consumed.
func (pi PathIterator) Done() bool { return len(pi.raw) == pi.consumed && !pi.lastEmpty }

// Next returns the first component of the path, consuming it.
func (pi *PathIterator) Next() string {
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
