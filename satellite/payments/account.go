// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// ErrAccountNotSetup is an error type which indicates that payment account is not created.
var ErrAccountNotSetup = errs.Class("payment account is not set up")

// Accounts exposes all needed functionality to manage payment accounts.
//
// architecture: Service
type Accounts interface {
	// Setup creates a payment account for the user.
	// If account is already set up it will return nil.
	Setup(ctx context.Context, userID uuid.UUID, email string, signupPromoCode string) (CouponType, error)

	// SaveBillingAddress saves billing address for a user and returns the updated billing information.
	SaveBillingAddress(ctx context.Context, userID uuid.UUID, address BillingAddress) (*BillingInformation, error)

	// AddTaxID adds a new tax ID for a user and returns the updated billing information.
	AddTaxID(ctx context.Context, userID uuid.UUID, taxID TaxID) (*BillingInformation, error)

	// RemoveTaxID removes a tax ID from a user and returns the updated billing information.
	RemoveTaxID(ctx context.Context, userID uuid.UUID, id string) (*BillingInformation, error)

	// GetBillingInformation gets the billing information for a user.
	GetBillingInformation(ctx context.Context, userID uuid.UUID) (*BillingInformation, error)

	// UpdatePackage updates a customer's package plan information.
	UpdatePackage(ctx context.Context, userID uuid.UUID, packagePlan *string, timestamp *time.Time) error

	// GetPackageInfo returns the package plan and time of purchase for a user.
	GetPackageInfo(ctx context.Context, userID uuid.UUID) (packagePlan *string, purchaseTime *time.Time, err error)

	// Balances exposes functionality to manage account balances.
	Balances() Balances

	// ProjectCharges returns how much money current user will be charged for each project.
	ProjectCharges(ctx context.Context, userID uuid.UUID, since, before time.Time) (ProjectChargesResponse, error)

	// GetProjectUsagePriceModel returns the project usage price model for a partner name.
	GetProjectUsagePriceModel(partner string) ProjectUsagePriceModel

	// CheckProjectInvoicingStatus returns error if for the given project there are outstanding project records and/or usage
	// which have not been applied/invoiced yet (meaning sent over to stripe).
	CheckProjectInvoicingStatus(ctx context.Context, projectID uuid.UUID) error

	// CheckProjectUsageStatus returns error if for the given project there is some usage for current or previous month.
	CheckProjectUsageStatus(ctx context.Context, projectID uuid.UUID) error

	// Charges returns list of all credit card charges related to account.
	Charges(ctx context.Context, userID uuid.UUID) ([]Charge, error)

	// CreditCards exposes all needed functionality to manage account credit cards.
	CreditCards() CreditCards

	// StorjTokens exposes all storj token related functionality.
	StorjTokens() StorjTokens

	// Invoices exposes all needed functionality to manage account invoices.
	Invoices() Invoices

	// Coupons exposes all needed functionality to manage coupons.
	Coupons() Coupons
}
