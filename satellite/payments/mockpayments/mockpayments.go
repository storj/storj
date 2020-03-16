// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mockpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/storj/satellite/payments"
)

var (
	// Error defines mock payment service error.
	Error = errs.Class("mock payment service error")

	mon = monkit.Package()
)

var _ payments.Accounts = (*accounts)(nil)

// accounts is a mock implementation of payments.Accounts.
//
// architecture: Service
type accounts struct{}

var _ payments.CreditCards = (*creditCards)(nil)

// creditCards is a mock implementation of payments.CreditCards.
//
// architecture: Service
type creditCards struct{}

var _ payments.Invoices = (*invoices)(nil)

// invoices is a mock implementation of payments.Invoices.
//
// architecture: Service
type invoices struct{}

var _ payments.StorjTokens = (*storjTokens)(nil)

// storjTokens is a mock implementation of payments.StorjTokens.
//
// architecture: Service
type storjTokens struct{}

// ensures that coupons implements payments.Coupons.
var _ payments.Coupons = (*coupons)(nil)

// coupons is an implementation of payments.Coupons.
//
// architecture: Service
type coupons struct{}

// ensures that credits implements payments.Credits.
var _ payments.Credits = (*credits)(nil)

// credits is an implementation of payments.Credits.
//
// architecture: Service
type credits struct{}

// Accounts exposes all needed functionality to manage payment accounts.
func Accounts() payments.Accounts {
	return &accounts{}
}

// CreditCards exposes all needed functionality to manage account credit cards.
func (accounts *accounts) CreditCards() payments.CreditCards {
	return &creditCards{}
}

// Invoices exposes all needed functionality to manage account invoices.
func (accounts *accounts) Invoices() payments.Invoices {
	return &invoices{}
}

// StorjTokens exposes all storj token related functionality.
func (accounts *accounts) StorjTokens() payments.StorjTokens {
	return &storjTokens{}
}

// Coupons exposes all needed functionality to manage coupons.
func (accounts *accounts) Coupons() payments.Coupons {
	return &coupons{}
}

// Credits exposes all needed functionality to manage coupons.
func (accounts *accounts) Credits() payments.Credits {
	return &credits{}
}

// Setup creates a payment account for the user.
// If account is already set up it will return nil.
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	return nil
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return 0, nil
}

// ProjectCharges returns how much money current user will be charged for each project.
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID, since, before time.Time) (charges []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx, userID, since, before)(&err)

	return []payments.ProjectCharge{}, nil
}

// Charges returns empty charges list.
func (accounts accounts) Charges(ctx context.Context, userID uuid.UUID) (_ []payments.Charge, err error) {
	defer mon.Task()(&ctx, userID)(&err)
	return []payments.Charge{}, nil
}

// List returns a list of credit cards for a given payment account.
func (creditCards *creditCards) List(ctx context.Context, userID uuid.UUID) (_ []payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return []payments.CreditCard{payments.CreditCard{
		ID:        "pm_randomcardid",
		ExpMonth:  12,
		ExpYear:   2030,
		Brand:     "Mastercard",
		Last4:     "1234",
		IsDefault: true,
	}}, nil
}

// Add is used to save new credit card, attach it to payment account and make it default.
func (creditCards *creditCards) Add(ctx context.Context, userID uuid.UUID, cardToken string) (err error) {
	defer mon.Task()(&ctx, userID, cardToken)(&err)

	return nil
}

// MakeDefault makes a credit card default payment method.
func (creditCards *creditCards) MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, userID, cardID)(&err)

	return nil
}

// Remove is used to remove credit card from payment account.
func (creditCards *creditCards) Remove(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	return nil
}

// List returns a list of invoices for a given payment account.
func (invoices *invoices) List(ctx context.Context, userID uuid.UUID) (_ []payments.Invoice, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return []payments.Invoice{}, nil
}

// Deposit creates new deposit transaction.
func (tokens *storjTokens) Deposit(ctx context.Context, userID uuid.UUID, amount int64) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx, userID, amount)(&err)

	return nil, Error.Wrap(errs.New("can not make deposit"))
}

// ListTransactionInfos returns empty transaction infos slice.
func (tokens *storjTokens) ListTransactionInfos(ctx context.Context, userID uuid.UUID) (_ []payments.TransactionInfo, err error) {
	defer mon.Task()(&ctx, userID)(&err)
	return ([]payments.TransactionInfo)(nil), nil
}

// Create attaches a coupon for payment account.
func (coupons *coupons) Create(ctx context.Context, coupon payments.Coupon) (err error) {
	defer mon.Task()(&ctx, coupon)(&err)

	return nil
}

// ListByUserID return list of all coupons of specified payment account.
func (coupons *coupons) ListByUserID(ctx context.Context, userID uuid.UUID) (_ []payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return ([]payments.Coupon)(nil), Error.Wrap(err)
}

// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have
// a project, payment method and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (coupons *coupons) PopulatePromotionalCoupons(ctx context.Context, duration int, amount int64, projectLimit memory.Size) (err error) {
	defer mon.Task()(&ctx, duration, amount, projectLimit)(&err)

	return nil
}

// AddPromotionalCoupon is used to add a promotional coupon for specified users who already have
// a project and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (coupons *coupons) AddPromotionalCoupon(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return nil
}

// Create attaches a credit for payment account.
func (credits *credits) Create(ctx context.Context, credit payments.Credit) (err error) {
	defer mon.Task()(&ctx, credit)(&err)

	return nil
}

// ListByUserID return list of all credits of specified payment account.
func (credits *credits) ListByUserID(ctx context.Context, userID uuid.UUID) (_ []payments.Credit, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return ([]payments.Credit)(nil), Error.Wrap(err)
}
