// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
)

// ErrAccountNotSetup is an error type which indicates that payment account is not created.
var ErrAccountNotSetup = errs.Class("payment account is not set up")

// Accounts exposes all needed functionality to manage payment accounts.
type Accounts interface {
	// Setup creates a payment account for the user.
	// If account is already set up it will return nil.
	Setup(ctx context.Context, userID uuid.UUID, email string) error

	// Balance returns an integer amount in cents that represents the current balance of payment account.
	Balance(ctx context.Context, userID uuid.UUID) (int64, error)

	// ProjectCharges returns how much money current user will be charged for each project.
	ProjectCharges(ctx context.Context, userID uuid.UUID) ([]ProjectCharge, error)

	// Charges returns list of all credit card charges related to account.
	Charges(ctx context.Context, userID uuid.UUID) ([]Charge, error)

	// Coupons return list of all coupons of specified payment account.
	Coupons(ctx context.Context, userID uuid.UUID) ([]Coupon, error)

	// CreditCards exposes all needed functionality to manage account credit cards.
	CreditCards() CreditCards

	// StorjTokens exposes all storj token related functionality.
	StorjTokens() StorjTokens

	// Invoices exposes all needed functionality to manage account invoices.
	Invoices() Invoices
}
