// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

var ErrInvoiceRollupExists = Error.New("project invoice rollup already exists")

type InvoicesDB interface {
	CheckRollup(ctx context.Context, projectID uuid.UUID, start, before time.Time) error
	InvoiceRollup(ctx context.Context, rollups []InvoiceRollup, start, before time.Time) error
	ListUnappliedIntents(ctx context.Context, offset int64, limit int, before time.Time) (InvoiceIntentPage, error)
	ConsumeIntent(ctx context.Context, projectID uuid.UUID, start, before time.Time) error
}

type InvoiceRollup struct {
	ProjectID uuid.UUID
	Storage   float64
	Egress    int64
	Objects   int64
}

type InvoiceIntent struct {
	ProjectID uuid.UUID
	Storage   float64
	Egress    int64
	Objects   int64
	Start     time.Time
	Before    time.Time
}

type InvoiceIntentPage struct {
	Intents    []InvoiceIntent
	Next       bool
	NextOffset int64
}
