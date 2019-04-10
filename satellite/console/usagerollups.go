// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// UsageRollups defines how console works with usage rollups
type UsageRollups interface {
	GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (*ProjectUsage, error)
	GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) ([]BucketUsageRollup, error)
	GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor BucketUsageCursor, since, before time.Time) (*BucketUsagePage, error)
}

// ProjectUsage consist of period total storage, egress
// and objects count per hour for certain Project
type ProjectUsage struct {
	Storage     float64
	Egress      float64
	ObjectCount float64

	Since  time.Time
	Before time.Time
}

// BucketUsageCursor holds info for bucket usage
// cursor pagination
type BucketUsageCursor struct {
	AfterBucket []byte
	Limit uint
}

// BucketUsagePage represents bucket usage page result
type BucketUsagePage struct {
	BucketUsages []BucketUsage
	HasMore bool
}

// BucketUsage consist of total bucket usage for period
type BucketUsage struct {
	ProjectID  uuid.UUID
	BucketName string

	Storage     float64
	Egress      float64
	ObjectCount float64

	Since  time.Time
	Before time.Time
}

// BucketUsageRollup is total bucket usage info
// for certain period
type BucketUsageRollup struct {
	ProjectID  uuid.UUID
	BucketName []byte

	RemoteStoredData float64
	InlineStoredData float64

	RemoteSegments float64
	InlineSegments float64
	ObjectCount    float64
	MetadataSize   float64

	RepairEgress float64
	GetEgress    float64
	AuditEgress  float64

	Since  time.Time
	Before time.Time
}
