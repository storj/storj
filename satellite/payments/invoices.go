// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Invoices exposes all needed functionality to manage account invoices.
type Invoices interface {
	// List returns a list of invoices for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]Invoice, error)
}

// Invoice holds all public information about invoice.
type Invoice struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	Link        string    `json:"link"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}
