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
	Cursor   uuid.UUID

	Order Order
	After time.Time

	Limit int
	Next  *UsageIterator
}

// Order is sort order can only be asc or desc
type Order string

const (
	// Asc ascending sort order
	Asc Order = "asc"
	// Desc descending sort order
	Desc Order = "desc"
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
