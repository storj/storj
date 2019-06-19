// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"bytes"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
)

// BucketID represents a fully qualified identifier for a bucket name and some project id.
type BucketID struct {
	projectID uuid.UUID
	bucket    string
}

// NewBucketID constructs a BucketID for the project and bucket name.
func NewBucketID(projectID uuid.UUID, bucket string) BucketID {
	b := BucketID{
		projectID: projectID,
		bucket:    bucket,
	}
	return b
}

// ParseBucketID parses a BucketID from raw bytes.
func ParseBucketID(raw []byte) (BucketID, error) {
	index := bytes.IndexByte(raw, '/')
	if index == -1 {
		return BucketID{}, errs.New("invalid raw bucket id: %q", raw)
	}
	projectID, err := uuid.Parse(string(raw[:index]))
	if err != nil {
		return BucketID{}, errs.Wrap(err)
	}
	return BucketID{
		projectID: *projectID,
		bucket:    string(raw[index+1:]),
	}, nil
}

// ProjectID returns the project id associated with the BucketID.
func (b BucketID) ProjectID() uuid.UUID { return b.projectID }

// Bucket returns the bucket name associated with the BucketID.
func (b BucketID) Bucket() string { return b.bucket }

// Raw returns a byte representation of the BucketID.
func (b BucketID) Raw() []byte {
	var out []byte
	out = append(out, b.ProjectID().String()...)
	out = append(out, '/')
	out = append(out, b.bucket...)
	return out
}
