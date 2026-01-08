// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// DeletionRemainderCharge represents a charge for an object deleted before minimum retention.
type DeletionRemainderCharge struct {
	ProjectID      uuid.UUID
	BucketName     string
	CreatedAt      time.Time
	DeletedAt      time.Time
	ObjectSize     uint64
	RemainderHours float64
	ProductID      int32
	Billed         bool
}

// DeletionRemainderDB stores information about deletion remainder charges.
type DeletionRemainderDB interface {
	// Create inserts a new deletion remainder charge record.
	Create(ctx context.Context, charge DeletionRemainderCharge) error
	// GetUnbilledCharges retrieves all unbilled charges for a project in the given time period.
	GetUnbilledCharges(ctx context.Context, projectID uuid.UUID, from, to time.Time) ([]DeletionRemainderCharge, error)
	// MarkChargesAsBilled marks all unbilled charges in the time period as billed.
	MarkChargesAsBilled(ctx context.Context, projectID uuid.UUID, from, to time.Time) error
}
