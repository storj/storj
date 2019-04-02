// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// BucketUsage is bucket usage rollup repository
type BucketUsage interface {
	Get(ctx context.Context, id uuid.UUID) (*BucketRollup, error)
	GetPaged(ctx context.Context, cursor *BucketRollupCursor) ([]BucketRollup, error)
	Create(ctx context.Context, rollup BucketRollup) (*BucketRollup, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Order is sorting order can be asc or desc
type Order string

const (
	// Asc ascending sort order
	Asc Order = "asc"
	// Desc descending sort order
	Desc Order = "desc"
)

// BucketRollupCursor encapsulates cursor based page
type BucketRollupCursor struct {
	BucketID uuid.UUID
	Before   time.Time
	After    time.Time

	Order Order

	PageSize int
	Next     *BucketRollupCursor
}

// BucketRollup holds usage rollup info
type BucketRollup struct {
	ID       uuid.UUID
	BucketID uuid.UUID

	RollupEndTime time.Time

	RemoteStoredData uint64
	InlineStoredData uint64
	RemoteSegments   uint
	InlineSegments   uint
	Objects          uint
	MetadataSize     uint64

	RepairEgress uint64
	GetEgress    uint64
	AuditEgress  uint64
}

// BucketBandwidthRollup contains data about bandwidth rollup
type BucketBandwidthRollup struct {
	BucketName string
	ProjectID  uuid.UUID

	IntervalStart   time.Time
	IntervalSeconds uint
	Action          uint

	Inline    uint64
	Allocated uint64
	Settled   uint64
}

// BucketStorageTally holds data about a bucket tally
type BucketStorageTally struct {
	BucketName    string
	ProjectID     uuid.UUID
	IntervalStart time.Time

	InlineSegmentCount int64
	RemoteSegmentCount int64

	ObjectCount int64

	InlineBytes  int64
	RemoteBytes  int64
	MetadataSize int64
}
