// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"strconv"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/paths"
)

// Path is an opaque representation of a metainfo database key.
type Path struct {
	projectID    uuid.UUID
	segmentIndex int64
	bucket       string
	encPath      paths.Encrypted

	raw []byte // raw byte representation of the path

	hasBucket  bool // true if the bucket field is valid
}

// ProjectID returns the project id associated with the path.
func (p Path) ProjectID() uuid.UUID { return p.projectID }

// SegmentIndex returns the segment index associated with the path.
func (p Path) SegmentIndex() int64 { return p.segmentIndex }

// Bucket returns the bucket part of the path and a bool if it exists.
func (p Path) Bucket() (string, bool) { return p.bucket, p.hasBucket }

// EncryptedPath returns the encrypted path part of the path.
func (p Path) EncryptedPath() (paths.Encrypted) { return p.encPath }

// Raw returns the raw data in the path.
func (p Path) Raw() []byte { return p.raw }

// String returns the string form of the raw data in the path.
func (p Path) String() string { return string(p.raw) }

// ParsePath returns a new path with the given raw bytes.
func ParsePath(raw []byte) (path Path, err error) {
	// There are 3 components before the path, so we have at most 4 splits and require
	// at least 2 for the project and segment.
	parts := bytes.SplitN(raw, []byte("/"), 4)
	if len(parts) < 2 {
		return Path{}, errs.New("invalid metainfo path: %q", raw)
	}

	// Save the raw part.
	path.raw = raw

	// Parse the project id.
	projectID, err := uuid.Parse(string(parts[0]))
	if err != nil {
		return Path{}, errs.Wrap(err)
	}
	path.projectID = *projectID

	// Parse the segment index.
	if len(parts[1]) == 0 {
		return Path{}, errs.Wrap(err)
	} else if parts[1][0] == 'l' {
		path.segmentIndex = -1
	} else if parts[1][0] == 's' {
		path.segmentIndex, err = strconv.ParseInt(string(parts[1][1:]), 10, 64)
		if err != nil {
			return Path{}, errs.Wrap(err)
		}
	} else {
		return Path{}, errs.New("invalid segment in metainfo path: %q", raw)
	}

	// Parse the bucket and path.
	if len(parts) >= 3 {
		path.bucket, path.hasBucket = string(parts[2]), true
		if len(parts) == 4 {
			path.encPath = paths.NewEncrypted(string(parts[4]))
		}
	}

	return path, nil
}

// CreatePath will create a path for the provided information. An empty string or encrypted path
// is treated as them not existing.
func CreatePath(ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket string, encPath paths.Encrypted) (path Path, err error) {
	defer mon.Task()(&ctx)(&err)

	if segmentIndex < -1 {
		return Path{}, errs.New("invalid segment index")
	}

	path = Path{
		projectID:    projectID,
		segmentIndex: segmentIndex,
	}

	path.raw = append(path.raw, projectID.String()...)
	path.raw = append(path.raw, '/')

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
		path.bucket, path.hasBucket = bucket, true

		if encPath.Valid() {
			path.raw = append(path.raw, encPath.Raw()...)
			path.raw = append(path.raw, '/')
			path.encPath = encPath
		}
	}

	path.raw = path.raw[:len(path.raw)-1]
	return path, nil
}
