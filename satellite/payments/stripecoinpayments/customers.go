// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// CustomersDB is interface for working with stripe customers table.
//
// architecture: Database
type CustomersDB interface {
	// Insert inserts a stripe customer into the database.
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error

	// GetCustomerID return stripe customers id.
	GetCustomerID(ctx context.Context, userID uuid.UUID) (string, error)
}
