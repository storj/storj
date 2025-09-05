// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ErrProjectRecordExists is error class defining that such project record already exists.
var ErrProjectRecordExists = Error.New("invoice project record already exists")

// ProjectRecordsDB is interface for working with invoice project records.
//
// architecture: Database
type ProjectRecordsDB interface {
	// Create creates new invoice project record with credits spendings in the DB.
	Create(ctx context.Context, records []CreateProjectRecord, start, end time.Time) error
	// Check checks if invoice project record for specified project and billing period exists.
	Check(ctx context.Context, projectID uuid.UUID, start, end time.Time) error
	// Get returns record for specified project and billing period.
	Get(ctx context.Context, projectID uuid.UUID, start, end time.Time) (*ProjectRecord, error)
	// GetUnappliedByProjectIDs returns unapplied records within the billing period pertaining to a list of project IDs.
	GetUnappliedByProjectIDs(ctx context.Context, projectIDs []uuid.UUID, start, end time.Time) ([]ProjectRecord, error)
	// Consume consumes invoice project record.
	Consume(ctx context.Context, id uuid.UUID) error
	// ListUnapplied returns project records page with unapplied project records.
	// Cursor is not included into listing results.
	ListUnapplied(ctx context.Context, cursor uuid.UUID, limit int, start, end time.Time) (ProjectRecordsPage, error)
}

// CreateProjectRecord holds info needed for creation new invoice
// project record.
type CreateProjectRecord struct {
	ProjectID uuid.UUID
	Storage   float64
	Egress    int64
	Segments  float64
}

// ProjectRecord holds project usage particular for billing period.
type ProjectRecord struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Storage     float64
	Egress      int64
	Segments    float64
	PeriodStart time.Time
	PeriodEnd   time.Time
	State       int

	// transient field to retrieved from project records db.
	ProjectPublicID uuid.UUID
}

// ProjectRecordsPage holds project records and
// indicates if there is more data available
// and provides cursor for next listing.
type ProjectRecordsPage struct {
	Records []ProjectRecord
	Next    bool
	Cursor  uuid.UUID
}
