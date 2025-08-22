// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ErrNoCustomer is error class defining that there is no customer for user.
var ErrNoCustomer = Error.New("customer doesn't exist")

// CustomersDB is interface for working with stripe customers table.
//
// architecture: Database
type CustomersDB interface {
	// Insert inserts a stripe customer into the database.
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error
	// GetCustomerID return stripe customers id.
	GetCustomerID(ctx context.Context, userID uuid.UUID) (string, error)
	// GetUserID return userID given stripe customer id.
	GetUserID(ctx context.Context, customerID string) (uuid.UUID, error)
	// List returns page with customers ids created before specified date.
	List(ctx context.Context, userIDCursor uuid.UUID, limit int, before time.Time) (CustomersPage, error)
	// UpdatePackage updates the customer's package plan and purchase time.
	UpdatePackage(ctx context.Context, userID uuid.UUID, packagePlan *string, timestamp *time.Time) (*Customer, error)
	// GetPackageInfo returns the package plan and time of purchase for a user.
	GetPackageInfo(ctx context.Context, userID uuid.UUID) (packagePlan *string, purchaseTime *time.Time, err error)

	// ListMissingCustomers lists users that have a missing stripe entry.
	ListMissingCustomers(ctx context.Context) (_ []MissingCustomer, err error)
}

// Customer holds customer id, user id, and package information.
type Customer struct {
	ID                 string
	BillingID          *string
	UserID             uuid.UUID
	PackagePlan        *string
	PackagePurchasedAt *time.Time
}

// MissingCustomer holds customer id, user id, and package information.
type MissingCustomer struct {
	ID              uuid.UUID
	Email           string
	SignupPromoCode string
}

// CustomersPage holds customers and
// indicates if there is more data available
// and provides cursor for next page.
type CustomersPage struct {
	Customers []Customer
	Next      bool
	Cursor    uuid.UUID
}
