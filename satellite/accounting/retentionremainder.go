// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// RetentionRemainderContinuationToken is used for paginating through retention remainder charges.
type RetentionRemainderContinuationToken = dbx.Paged_RetentionRemainderChargesInDeletedAtRangeByProjectID_Continuation

// GetUnbilledChargesOptions contains options for retrieving unbilled retention remainder charges.
type GetUnbilledChargesOptions struct {
	ProjectID uuid.UUID
	From      time.Time
	To        time.Time
	Limit     int
	NextToken *RetentionRemainderContinuationToken
}

// RetentionRemainderCharge represents a charge for an object deleted before minimum retention.
type RetentionRemainderCharge struct {
	ProjectID          uuid.UUID
	BucketName         string
	DeletedAt          time.Time
	RemainderByteHours float64
	ProductID          int32
	Billed             bool
}

// RetentionRemainderDB stores information about retention remainder charges.
type RetentionRemainderDB interface {
	// Upsert inserts a new deletion remainder charge record or updates an existing one by adding the remainder byte hours.
	// The upsert hinges on a unique constraint on (project_id, bucket_name, deleted_at). deleted_at is at month precision.
	Upsert(ctx context.Context, charge RetentionRemainderCharge) error
	// GetUnbilledCharges retrieves unbilled charges for a project in the given time period.
	GetUnbilledCharges(ctx context.Context, options GetUnbilledChargesOptions) ([]RetentionRemainderCharge, *RetentionRemainderContinuationToken, error)
	// MarkChargesAsBilled marks all unbilled charges in the time period as billed.
	MarkChargesAsBilled(ctx context.Context, projectID uuid.UUID, from, to time.Time) error
}
