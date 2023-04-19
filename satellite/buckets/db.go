// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var (
	// ErrBucket is an error class for general bucket errors.
	ErrBucket = errs.Class("bucket")

	// ErrNoBucket is an error class for using empty bucket name.
	ErrNoBucket = errs.Class("no bucket specified")

	// ErrBucketNotFound is an error class for non-existing bucket.
	ErrBucketNotFound = errs.Class("bucket not found")
)

// Bucket contains information about a specific bucket.
type Bucket struct {
	ID                          uuid.UUID
	Name                        string
	ProjectID                   uuid.UUID
	PartnerID                   uuid.UUID
	UserAgent                   []byte
	Created                     time.Time
	PathCipher                  storj.CipherSuite
	DefaultSegmentsSize         int64
	DefaultRedundancyScheme     storj.RedundancyScheme
	DefaultEncryptionParameters storj.EncryptionParameters
	Placement                   storj.PlacementConstraint
}

// ListDirection specifies listing direction.
type ListDirection int32

const (
	// DirectionForward lists forwards from cursor, including cursor.
	DirectionForward = 1
	// DirectionAfter lists forwards from cursor, without cursor.
	DirectionAfter = 2
)

// MinimalBucket contains minimal bucket fields for metainfo protocol.
type MinimalBucket struct {
	Name      []byte
	CreatedAt time.Time
}

// ListOptions lists objects.
type ListOptions struct {
	Cursor    string
	Direction ListDirection
	Limit     int
}

// NextPage returns options for listing the next page.
func (opts ListOptions) NextPage(list List) ListOptions {
	if !list.More || len(list.Items) == 0 {
		return ListOptions{}
	}

	return ListOptions{
		Cursor:    list.Items[len(list.Items)-1].Name,
		Direction: DirectionAfter,
		Limit:     opts.Limit,
	}
}

// List is a list of buckets.
type List struct {
	More  bool
	Items []Bucket
}

// DB is the interface for the database to interact with buckets.
//
// architecture: Database
type DB interface {
	// CreateBucket creates a new bucket
	CreateBucket(ctx context.Context, bucket Bucket) (_ Bucket, err error)
	// GetBucket returns an existing bucket
	GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket Bucket, err error)
	// GetBucketPlacement returns with the placement constraint identifier.
	GetBucketPlacement(ctx context.Context, bucketName []byte, projectID uuid.UUID) (placement storj.PlacementConstraint, err error)
	// GetMinimalBucket returns existing bucket with minimal number of fields.
	GetMinimalBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket MinimalBucket, err error)
	// HasBucket returns if a bucket exists.
	HasBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (exists bool, err error)
	// GetBucketID returns an existing bucket id.
	GetBucketID(ctx context.Context, bucket metabase.BucketLocation) (id uuid.UUID, err error)
	// UpdateBucket updates an existing bucket
	UpdateBucket(ctx context.Context, bucket Bucket) (_ Bucket, err error)
	// DeleteBucket deletes a bucket
	DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error)
	// ListBuckets returns all buckets for a project
	ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts ListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList List, err error)
	// CountBuckets returns the number of buckets a project currently has
	CountBuckets(ctx context.Context, projectID uuid.UUID) (int, error)
	// IterateBucketLocations iterates through all buckets from some point with limit.
	IterateBucketLocations(ctx context.Context, projectID uuid.UUID, bucketName string, limit int, fn func([]metabase.BucketLocation) error) (more bool, err error)
}
