// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectInvoiceStamps
type ProjectInvoiceStamps interface {
	Create(ctx context.Context, stamp ProjectInvoiceStamp) (*ProjectInvoiceStamp, error)
	GetByProjectIDStartDate(ctx context.Context, projectID uuid.UUID, startDate time.Time) (*ProjectInvoiceStamp, error)
	GetAll(ctx context.Context, projectID uuid.UUID) ([]ProjectInvoiceStamp, error)
}

// ProjectInvoiceStamp
type ProjectInvoiceStamp struct {
	ProjectID uuid.UUID
	InvoiceID string

	StartDate time.Time
	EndDate   time.Time

	CreatedAt time.Time
}
