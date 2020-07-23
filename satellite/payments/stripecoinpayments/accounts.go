// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// ensures that accounts implements payments.Accounts.
var _ payments.Accounts = (*accounts)(nil)

// accounts is an implementation of payments.Accounts.
//
// architecture: Service
type accounts struct {
	service *Service
}

// CreditCards exposes all needed functionality to manage account credit cards.
func (accounts *accounts) CreditCards() payments.CreditCards {
	return &creditCards{service: accounts.service}
}

// Invoices exposes all needed functionality to manage account invoices.
func (accounts *accounts) Invoices() payments.Invoices {
	return &invoices{service: accounts.service}
}

// Setup creates a payment account for the user.
// If account is already set up it will return nil.
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	_, err = accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err == nil {
		return nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	customer, err := accounts.service.stripeClient.Customers().New(params)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ payments.Balance, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	c, err := accounts.service.stripeClient.Customers().Get(customerID, nil)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	// add all active coupons amount to balance.
	coupons, err := accounts.service.db.Coupons().ListByUserIDAndStatus(ctx, userID, payments.CouponActive)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	var couponsAmount int64 = 0
	for _, coupon := range coupons {
		alreadyUsed, err := accounts.service.db.Coupons().TotalUsage(ctx, coupon.ID)
		if err != nil {
			return payments.Balance{}, Error.Wrap(err)
		}

		couponsAmount += coupon.Amount - alreadyUsed
	}

	accountBalance := payments.Balance{
		FreeCredits: couponsAmount,
		Coins:       -c.Balance,
	}

	return accountBalance, nil
}

// ProjectCharges returns how much money current user will be charged for each project.
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID, since, before time.Time) (charges []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx, userID, since, before)(&err)

	// to return empty slice instead of nil if there are no projects
	charges = make([]payments.ProjectCharge, 0)

	projects, err := accounts.service.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, project := range projects {
		usage, err := accounts.service.usageDB.GetProjectTotal(ctx, project.ID, since, before)
		if err != nil {
			return charges, Error.Wrap(err)
		}

		projectPrice := accounts.service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount)

		charges = append(charges, payments.ProjectCharge{
			ProjectUsage: *usage,

			ProjectID:    project.ID,
			Egress:       projectPrice.Egress.IntPart(),
			ObjectCount:  projectPrice.Objects.IntPart(),
			StorageGbHrs: projectPrice.Storage.IntPart(),
		})
	}

	return charges, nil
}

// Charges returns list of all credit card charges related to account.
func (accounts *accounts) Charges(ctx context.Context, userID uuid.UUID) (_ []payments.Charge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.ChargeListParams{
		Customer: stripe.String(customerID),
	}
	params.Filters.AddFilter("limit", "", "100")

	iter := accounts.service.stripeClient.Charges().List(params)

	var charges []payments.Charge
	for iter.Next() {
		charge := iter.Charge()

		// ignore all non credit card charges
		if charge.PaymentMethodDetails.Type != stripe.ChargePaymentMethodDetailsTypeCard {
			continue
		}
		if charge.PaymentMethodDetails.Card == nil {
			continue
		}

		charges = append(charges, payments.Charge{
			ID:     charge.ID,
			Amount: charge.Amount,
			CardInfo: payments.CardInfo{
				ID:       charge.PaymentMethod,
				Brand:    string(charge.PaymentMethodDetails.Card.Brand),
				LastFour: charge.PaymentMethodDetails.Card.Last4,
			},
			CreatedAt: time.Unix(charge.Created, 0).UTC(),
		})
	}

	if err = iter.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return charges, nil
}

// StorjTokens exposes all storj token related functionality.
func (accounts *accounts) StorjTokens() payments.StorjTokens {
	return &storjTokens{service: accounts.service}
}

// Coupons exposes all needed functionality to manage coupons.
func (accounts *accounts) Coupons() payments.Coupons {
	return &coupons{service: accounts.service}
}

// PaywallEnabled returns a true if a credit card or account
// balance is required to create projects.
func (accounts *accounts) PaywallEnabled(userID uuid.UUID) bool {
	return BytesAreWithinProportion(userID, accounts.service.PaywallProportion)
}

//BytesAreWithinProportion returns true if first byte is less than the normalized proportion [0..1].
func BytesAreWithinProportion(uuidBytes [16]byte, proportion float64) bool {
	return int(uuidBytes[0]) < int(proportion*256)
}
