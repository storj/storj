// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package attribution implements value attribution from docs/design/value-attribution.md
package attribution

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ErrBucketNotAttributed is returned if a requested bucket not attributed(entry not found).
var ErrBucketNotAttributed = errs.Class("bucket not attributed")

// Info describing value attribution from partner to bucket.
type Info struct {
	ProjectID  uuid.UUID
	BucketName []byte
	UserAgent  []byte
	Placement  *storj.PlacementConstraint
	CreatedAt  time.Time
}

// BucketUsage is the usage data for a single bucket.
type BucketUsage struct {
	UserAgent    []byte
	ProjectID    []byte
	BucketName   []byte
	ByteHours    float64
	SegmentHours float64
	ObjectHours  float64
	EgressData   int64
	Hours        int
}

// DB implements the database for value attribution table.
//
// architecture: Database
type DB interface {
	// Get retrieves attribution info using project id and bucket name.
	Get(ctx context.Context, projectID uuid.UUID, bucketName []byte) (*Info, error)
	// Insert creates and stores new Info.
	Insert(ctx context.Context, info *Info) (*Info, error)
	// UpdateUserAgent updates bucket attribution data.
	UpdateUserAgent(ctx context.Context, projectID uuid.UUID, bucketName string, userAgent []byte) error
	// UpdatePlacement updates bucket placement.
	UpdatePlacement(ctx context.Context, projectID uuid.UUID, bucketName string, placement *storj.PlacementConstraint) error
	// QueryAttribution queries partner bucket attribution data.
	QueryAttribution(ctx context.Context, userAgent []byte, start time.Time, end time.Time) ([]*BucketUsage, error)
	// QueryAllAttribution queries all partner bucket usage data.
	QueryAllAttribution(ctx context.Context, start time.Time, end time.Time) ([]*BucketUsage, error)
	// BackfillPlacementBatch updates up to batchSize rows of value_attributions.placement from bucket_metainfos.
	BackfillPlacementBatch(ctx context.Context, batchSize int) (int64, bool, error)
	// TestDelete is used for testing purposes to delete all attribution data for a given project and bucket.
	TestDelete(ctx context.Context, projectID uuid.UUID, bucketName []byte) error
}
