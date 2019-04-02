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
	Get(ctx context.Context, projectID uuid.UUID, bucketID []byte, before time.Time, count int) ([]UsageRollup, error)
}

// ProjectUsage consist of period total storage,
// egress and objects count for certain Project
type ProjectUsage struct {
	Storage      uint64
	Egress       uint64
	ObjectsCount uint

	Since  time.Time
	Before time.Time
}

// UsageRollup is usage rollup for a bucket
type UsageRollup struct {
	ID        []byte
	ProjectID uuid.UUID
	BucketID  []byte

	RollupEndTime time.Time

	RemoteStoredData uint64
	InlineStoredData uint64
	RemoteSegments   uint
	InlineSegments   uint
	Objects          uint
	Metadata_size    uint64
	RepairEgress     uint64
	GetEgress        uint64
	AuditEgress      uint64
}
