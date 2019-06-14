// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"bytes"
	"context"
	"strconv"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/paths"
)

// Path is a representation of a segmented object path within a bucket
type Path struct {
	segmentIndex int64
	bucket       string
	encPath      paths.Encrypted

	raw []byte

	hasBucket bool
}

// SegmentIndex returns the segment index associated with the path.
func (p Path) SegmentIndex() int64 { return p.segmentIndex }

// Bucket returns the bucket part of the path. Is the empty string if there's no bucket.
func (p Path) Bucket() (string, bool) { return p.bucket, p.hasBucket }

// EncryptedPath returns the encrypted path part of the path. Is the empty string if there's no path.
func (p Path) EncryptedPath() paths.Encrypted { return p.encPath }

// Raw returns the raw data in the path.
func (p Path) Raw() []byte { return p.raw }

// String returns the string form of the raw data in the path.
func (p Path) String() string { return string(p.raw) }

// ParsePath returns a new Path with the given raw bytes.
func ParsePath(raw []byte) (path Path, err error) {
	// There are 2 components before the path, so we have at most 3 splits and require
	// at least 1 for the segment.
	parts := bytes.SplitN(raw, []byte("/"), 3)
	if len(parts) < 1 {
		return Path{}, errs.New("invalid segments path: %q", raw)
	}

	// Save the raw part.
	path.raw = raw

	// Parse the segment index.
	if segment := parts[0]; len(segment) == 0 {
		return Path{}, errs.Wrap(err)
	} else if segment[0] == 'l' {
		path.segmentIndex = -1
	} else if segment[0] == 's' {
		path.segmentIndex, err = strconv.ParseInt(string(segment[1:]), 10, 64)
		if err != nil {
			return Path{}, errs.Wrap(err)
		}
	} else {
		return Path{}, errs.New("invalid segment in metainfo key: %q", raw)
	}

	// Parse the bucket and path.
	if len(parts) >= 2 {
		path.bucket, path.hasBucket = string(parts[1]), true
		if len(parts) == 3 {
			path.encPath = paths.NewEncrypted(string(parts[2]))
		}
	}

	return path, nil
}

// CreatePath will create a Path for the provided information. An empty string for the
// bucket or encrypted path is treated as them not existing.
func CreatePath(ctx context.Context, segmentIndex int64, bucket string, encPath paths.Encrypted) (path Path, err error) {
	defer mon.Task()(&ctx)(&err)

	if segmentIndex < -1 {
		return Path{}, errs.New("invalid segment index")
	}

	path = Path{
		segmentIndex: segmentIndex,
	}

	if segmentIndex > -1 {
		path.raw = append(path.raw, 's')
		path.raw = append(path.raw, strconv.FormatInt(segmentIndex, 10)...)
	} else {
		path.raw = append(path.raw, 'l')
	}
	path.raw = append(path.raw, '/')

	if len(bucket) > 0 {
		path.raw = append(path.raw, bucket...)
		path.raw = append(path.raw, '/')
		path.bucket = bucket

		if encPath.Valid() {
			path.raw = append(path.raw, encPath.Raw()...)
			path.raw = append(path.raw, '/')
			path.encPath = encPath
		}
	}

	path.raw = path.raw[:len(path.raw)-1]
	return path, nil
}
