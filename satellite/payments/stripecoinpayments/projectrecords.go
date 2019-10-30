// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

var ErrProjectRecordExists = Error.New("invoice project record already exists")

type ProjectRecordsDB interface {
	Create(ctx context.Context, records []CreateProjectRecord, start, end time.Time) error
	Check(ctx context.Context, projectID uuid.UUID, start, end time.Time) error
	Consume(ctx context.Context, id uuid.UUID) error
	ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (ProjectRecordsPage, error)
}

type CreateProjectRecord struct {
	ProjectID uuid.UUID
	Storage   float64
	Egress    int64
	Objects   int64
}

type ProjectRecord struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Storage     float64
	Egress      int64
	Objects     int64
	PeriodStart time.Time
	PeriodEnd   time.Time
}

type ProjectRecordsPage struct {
	Records    []ProjectRecord
	Next       bool
	NextOffset int64
}
