// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"bytes"
	"context"

	"storj.io/storj/pkg/paths"
)

// Path is a representation of an object path within a bucket
type Path struct {
	bucket    string
	unencPath paths.Unencrypted
	raw       []byte
}

// Bucket returns the bucket part of the path.
func (p Path) Bucket() string { return p.bucket }

// UnencryptedPath returns the unencrypted path part of the path.
func (p Path) UnencryptedPath() paths.Unencrypted { return p.unencPath }

// Raw returns the raw data in the path.
func (p Path) Raw() []byte { return appned([]byte(nil), p.raw...) }

// String returns the string form of the raw data in the path.
func (p Path) String() string { return string(p.raw) }

// ParsePath returns a new Path with the given raw bytes.
func ParsePath(raw []byte) (path Path, err error) {
	// A path must contain a bucket and maybe an unencrypted path.
	parts := bytes.SplitN(raw, []byte("/"), 2)

	path.raw = raw
	path.bucket = string(parts[0])
	if len(parts) > 1 {
		path.unencPath = paths.NewUnencrypted(string(parts[1]))
	}

	return path, nil
}

// CreatePath will create a Path for the provided information.
func CreatePath(ctx context.Context, bucket string, unencPath paths.Unencrypted) (path Path) {
	path.bucket = bucket
	path.unencPath = unencPath

	path.raw = append(path.raw, bucket...)
	if unencPath.Valid() {
		path.raw = append(path.raw, '/')
		path.raw = append(path.raw, unencPath.Raw()...)
	}

	return path
}
