// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// CustomersDB is interface for working with stripe customers table.
type CustomersDB interface {
	// Insert inserts a stripe customer into the database.
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error
	// GetCustomerID return stripe customers id.
	GetCustomerID(ctx context.Context, userID uuid.UUID) (string, error)
	// List returns page with customers ids created before specified date.
	List(ctx context.Context, offset int64, limit int, before time.Time) (CustomerPage, error)
}

// Customer holds customer id and user id.
type Customer struct {
	ID     string
	UserID uuid.UUID
}

// CustomersPage holds customers and
// indicates if there is more data available
// and provides next offset.
type CustomerPage struct {
	Customers  []Customer
	Next       bool
	NextOffset int64
}
