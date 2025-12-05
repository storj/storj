// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ErrAccountNotSetup is an error type which indicates that payment account is not created.
var ErrAccountNotSetup = errs.Class("payment account is not set up")

// PartnersPlacementProductMap maps partners to placements to products map.
type PartnersPlacementProductMap map[string]PlacementProductIdMap

// GetProductByPartnerAndPlacement returns the product mapped to the given partner and placement.
func (p PartnersPlacementProductMap) GetProductByPartnerAndPlacement(partner string, placement int) (int32, bool) {
	placementProductMap, ok := p[partner]
	if !ok {
		return 0, false
	}
	return placementProductMap.GetProductByPlacement(placement)
}

// PlacementProductIdMap maps placements to products.
type PlacementProductIdMap map[int]int32

// GetProductByPlacement returns the product mapped to the given placement.
func (p PlacementProductIdMap) GetProductByPlacement(placement int) (int32, bool) {
	id, ok := p[placement]
	return id, ok
}

// Accounts exposes all needed functionality to manage payment accounts.
//
// architecture: Service
type Accounts interface {
	// Setup creates a payment account for the user.
	// If account is already set up it will return nil.
	Setup(ctx context.Context, userID uuid.UUID, email string, signupPromoCode string) (CouponType, error)

	// EnsureUserHasCustomer creates a stripe customer for userID if non exists.
	EnsureUserHasCustomer(ctx context.Context, userID uuid.UUID, email string, signupPromoCode string) error

	// ShouldSkipMinimumCharge returns true if, for the given user, we should not apply a minimum charge.
	ShouldSkipMinimumCharge(ctx context.Context, cusID string, userID uuid.UUID) (bool, error)

	// SaveBillingAddress saves billing address for a user and returns the updated billing information.
	SaveBillingAddress(ctx context.Context, cusID string, userID uuid.UUID, address BillingAddress) (*BillingInformation, error)

	// AddTaxID adds a new tax ID for a user and returns the updated billing information.
	AddTaxID(ctx context.Context, cusID string, userID uuid.UUID, params AddTaxParams) (*BillingInformation, error)

	// AddDefaultInvoiceReference adds a new default invoice reference to be displayed on each invoice and returns the updated billing information.
	AddDefaultInvoiceReference(ctx context.Context, userID uuid.UUID, reference string) (*BillingInformation, error)

	// RemoveTaxID removes a tax ID from a user and returns the updated billing information.
	RemoveTaxID(ctx context.Context, userID uuid.UUID, id string) (*BillingInformation, error)

	// GetBillingInformation gets the billing information for a user.
	GetBillingInformation(ctx context.Context, userID uuid.UUID) (*BillingInformation, error)

	// UpdatePackage updates a customer's package plan information.
	UpdatePackage(ctx context.Context, userID uuid.UUID, packagePlan *string, timestamp *time.Time) error

	// ChangeEmail changes a customer's email address.
	ChangeEmail(ctx context.Context, userID uuid.UUID, email string) error

	// ChangeCustomerEmail changes a customer's email address given the customer ID. This is meant for use
	// for methods that run in a transaction to avoid ChangeEmail's non-tx DB lookup. Callers are expected
	// to have retrieved customer ID from DB already.
	ChangeCustomerEmail(ctx context.Context, userID uuid.UUID, cusID, email string) (err error)

	// GetPackageInfo returns the package plan and time of purchase for a user.
	GetPackageInfo(ctx context.Context, userID uuid.UUID) (packagePlan *string, purchaseTime *time.Time, err error)

	// Balances exposes functionality to manage account balances.
	Balances() Balances

	// ProductCharges returns how much money current user will be charged for each project split by product.
	ProductCharges(ctx context.Context, userID uuid.UUID, since, before time.Time) (ProductChargesResponse, error)

	// CalculateProjectUsagePrice calculates the price for given project usage and price model.
	CalculateProjectUsagePrice(usage ProjectUsage, priceModel ProjectUsagePriceModel) UsageCost

	// GetProjectUsagePriceModel returns the project usage price model for a partner name.
	GetProjectUsagePriceModel(partner string) ProjectUsagePriceModel

	// GetPartnerPlacementPriceModel returns the productID and related usage price model for a partner and placement,
	// if there is none defined for the project ID.
	GetPartnerPlacementPriceModel(ctx context.Context, projectPublicID uuid.UUID, partner string, placement storj.PlacementConstraint) (productID int32, _ ProductUsagePriceModel)

	// GetPlacementProductMappings returns the placement to product ID mappings for a partner
	// and the default mappings.
	GetPlacementProductMappings(partner string) (partnerMap PlacementProductIdMap, defaultMap PlacementProductIdMap)

	// GetPartnerNames returns the partners relevant to billing.
	GetPartnerNames() []string

	// ProductIdAndPriceForUsageKey returns the product ID and usage price model for a given usage key
	// if there is none defined for the project ID.
	ProductIdAndPriceForUsageKey(ctx context.Context, projectPublicID uuid.UUID, key string) (int32, ProductUsagePriceModel)

	// GetPartnerPlacements returns the placements for a project or partner. It will return the placements allowed for the
	// project on the entitlements level or those allowed globally for the partner if entitlements are disabled.
	// In the case of disabled entitlements, it also includes the placements for the default product price
	// config that have not been overridden for the partner.
	// It also returns a boolean, entitlementHasPlacement, indicating if the project's entitlement has any new buckets
	// placements defined.
	GetPartnerPlacements(ctx context.Context, projectPublicID uuid.UUID, partner string) (_ []storj.PlacementConstraint, entitlementsHasPlacements bool, _ error)

	// CheckProjectInvoicingStatus returns error if for the given project there are outstanding project records and/or usage
	// which have not been applied/invoiced yet (meaning sent over to stripe).
	CheckProjectInvoicingStatus(ctx context.Context, projectID uuid.UUID) error

	// CheckProjectUsageStatus returns error if for the given project there is some usage for current or previous month.
	CheckProjectUsageStatus(ctx context.Context, projectID, projectPublicID uuid.UUID) (currentUsageExists, invoicingIncomplete bool, currentMonthPrice decimal.Decimal, err error)

	// Charges returns list of all credit card charges related to account.
	Charges(ctx context.Context, userID uuid.UUID) ([]Charge, error)

	// CreditCards exposes all needed functionality to manage account credit cards.
	CreditCards() CreditCards

	// PaymentIntents exposes all needed functionality to manage credit cards charging.
	PaymentIntents() PaymentIntents

	// StorjTokens exposes all storj token related functionality.
	StorjTokens() StorjTokens

	// Invoices exposes all needed functionality to manage account invoices.
	Invoices() Invoices

	// Coupons exposes all needed functionality to manage coupons.
	Coupons() Coupons

	// WebhookEvents exposes all needed functionality to handle a stripe webhook event.
	WebhookEvents() WebhookEvents
}
