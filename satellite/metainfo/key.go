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

// Key is an opaque representation of a metainfo database key.
type Key struct {
	projectID    uuid.UUID
	segmentIndex int64
	bucket       string
	path         paths.Encrypted

	raw []byte // raw byte representation of the key

	hasBucket bool // true if the bucket field is valid
	hasPath   bool // true if the path field is valid
	empty     bool // true if the key has no valid fields
}

// ProjectID returns the project id associated with the key.
func (k Key) ProjectID() (uuid.UUID, bool) { return k.projectID, !k.empty }

// SegmentIndex returns the segment index associated with the key.
func (k Key) SegmentIndex() (int64, bool) { return k.segmentIndex, !k.empty }

// Bucket returns the bucket part of the key and a bool if it exists.
func (k Key) Bucket() (string, bool) { return k.bucket, k.hasBucket }

// Path returns the encrypted path part of the key and a bool if it exists.
func (k Key) Path() (paths.Encrypted, bool) { return k.path, k.hasPath }

// Raw returns the raw data in the key.
func (k Key) Raw() []byte { return k.raw }

// String returns the string form of the raw data in the key.
func (k Key) String() string { return string(k.raw) }

// Empty returns true if the key is empty.
func (k Key) Empty() bool { return k.empty }

// ParseKey returns a new Key with the given raw bytes.
func ParseKey(raw []byte) (key Key, err error) {
	// If the raw bytes are empty, we have no key.
	if len(raw) == 0 {
		return Key{empty: true}, nil
	}

	// There are 3 components before the path, so we have at most 4 splits and require
	// at least 2 for the project and segment.
	parts := bytes.SplitN(raw, []byte("/"), 4)
	if len(parts) < 2 {
		return Key{}, errs.New("invalid metainfo key: %q", raw)
	}

	// Save the raw part.
	key.raw = raw

	// Parse the project id.
	projectID, err := uuid.Parse(string(parts[0]))
	if err != nil {
		return Key{}, errs.Wrap(err)
	}
	key.projectID = *projectID

	// Parse the segment index.
	if len(parts[1]) == 0 {
		return Key{}, errs.Wrap(err)
	} else if parts[1][0] == 'l' {
		key.segmentIndex = -1
	} else if parts[1][0] == 's' {
		key.segmentIndex, err = strconv.ParseInt(string(parts[1][1:]), 10, 64)
		if err != nil {
			return Key{}, errs.Wrap(err)
		}
	} else {
		return Key{}, errs.New("invalid segment in metainfo key: %q", raw)
	}

	// Parse the bucket and path.
	if len(parts) >= 3 {
		key.bucket, key.hasBucket = string(parts[2]), true
		if len(parts) == 4 {
			key.path, key.hasPath = paths.NewEncrypted(string(parts[4])), true
		}
	}

	return key, nil
}

// CreateKey will create a key for the provided information. An empty string for the
// bucket or encrypted path is treated as them not existing.
func CreateKey(ctx context.Context, projectID uuid.UUID, segmentIndex int64, bucket string, path paths.Encrypted) (key Key, err error) {
	defer mon.Task()(&ctx)(&err)

	if segmentIndex < -1 {
		return Key{}, errs.New("invalid segment index")
	}

	key = Key{
		projectID:    projectID,
		segmentIndex: segmentIndex,
	}

	key.raw = append(key.raw, projectID.String()...)
	key.raw = append(key.raw, '/')

	if segmentIndex > -1 {
		key.raw = append(key.raw, 's')
		key.raw = append(key.raw, strconv.FormatInt(segmentIndex, 10)...)
	} else {
		key.raw = append(key.raw, 'l')
	}
	key.raw = append(key.raw, '/')

	if len(bucket) > 0 {
		key.raw = append(key.raw, bucket...)
		key.raw = append(key.raw, '/')
		key.bucket, key.hasBucket = bucket, true

		if path.Raw() != "" {
			key.raw = append(key.raw, path.Raw()...)
			key.raw = append(key.raw, '/')
			key.path, key.hasPath = path, true
		}
	}

	key.raw = key.raw[:len(key.raw)-1]
	return key, nil
}
