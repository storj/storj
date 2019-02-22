// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// BucketUsages is bucket usage rollup store interface
type BucketUsages interface {
	Get(context.Context, uuid.UUID) (*BucketUsage, error)
	GetByBucketID(context.Context, *UsageIterator) ([]BucketUsage, error)
	Create(context.Context, BucketUsage) (*BucketUsage, error)
	Delete(context.Context, uuid.UUID) error
}

// UsageIterator help iterate over bucket usage
type UsageIterator struct {
	BucketID uuid.UUID
	Cursor   time.Time

	Direction Direction

	Limit int
	Next  *UsageIterator
}

// Direction is sort order can only be asc or desc
type Direction string

const (
	// Fwd ascending sort order
	Fwd Direction = "fwd"
	// Bkwd descending sort order
	Bkwd Direction = "bkwd"
)

// BucketUsage is a rollup information for particular timestamp
type BucketUsage struct {
	ID       uuid.UUID
	BucketID uuid.UUID

	RollupEndTime time.Time

	RemoteStoredData uint64
	InlineStoredData uint64
	Segments         uint
	MetadataSize     uint64

	RepairEgress uint64
	GetEgress    uint64
	AuditEgress  uint64
}
