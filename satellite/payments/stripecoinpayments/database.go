// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// StripeCustomers is interface for working with stripe customers table
type Customers interface {
	// Insert is a method for inserting stripe customer into the database.
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error

	// GetAllCustomerIDs return all ids of stripe customers stored in DB
	GetAllCustomerIDs(ctx context.Context) (ids []string, err error)
}
