// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ErrProjectRecordExists is error class defining that such project record already exists.
var ErrProjectRecordExists = Error.New("invoice project record already exists")

// ProjectRecordsDB is interface for working with invoice project records.
//
// architecture: Database
type ProjectRecordsDB interface {
	// Create creates new invoice project record in the DB.
	Create(ctx context.Context, records []CreateProjectRecord, start, end time.Time) error
	// Check checks if invoice project record for specified project and billing period exists.
	Check(ctx context.Context, projectID uuid.UUID, start, end time.Time) error
	// Consume consumes invoice project record.
	Consume(ctx context.Context, id uuid.UUID) error
	// ListUnapplied returns project records page with unapplied project records.
	ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (ProjectRecordsPage, error)
}

// CreateProjectRecord holds info needed for creation new invoice
// project record.
type CreateProjectRecord struct {
	ProjectID uuid.UUID
	Storage   float64
	Egress    int64
	Objects   int64
}

// ProjectRecord holds project usage particular for billing period.
type ProjectRecord struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Storage     float64
	Egress      int64
	Objects     int64
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// ProjectRecordsPage holds project records and
// indicates if there is more data available
// and provides next offset.
type ProjectRecordsPage struct {
	Records    []ProjectRecord
	Next       bool
	NextOffset int64
}
