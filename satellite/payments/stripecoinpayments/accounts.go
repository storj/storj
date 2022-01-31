// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"errors"
	"time"

	"github.com/stripe/stripe-go/v72"

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
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string, signupPromoCode string) (couponType payments.CouponType, err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	couponType = payments.FreeTierCoupon

	_, err = accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err == nil {
		return couponType, nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	if signupPromoCode == "" {

		params.Coupon = stripe.String(accounts.service.StripeFreeTierCouponID)

		customer, err := accounts.service.stripeClient.Customers().New(params)
		if err != nil {
			return couponType, Error.Wrap(err)
		}

		// TODO: delete customer from stripe, if db insertion fails
		return couponType, Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
	}

	promoCodeIter := accounts.service.stripeClient.PromoCodes().List(&stripe.PromotionCodeListParams{
		Code: stripe.String(signupPromoCode),
	})

	var promoCode *stripe.PromotionCode

	if promoCodeIter.Next() {
		promoCode = promoCodeIter.PromotionCode()
	} else {
		couponType = payments.NoCoupon
	}

	// If signup promo code is provided, apply this on account creation.
	// If a free tier coupon is provided with no signup promo code, apply this on account creation.
	if promoCode != nil && promoCode.Coupon != nil {
		params.Coupon = stripe.String(promoCode.Coupon.ID)
		couponType = payments.SignupCoupon
	} else if accounts.service.StripeFreeTierCouponID != "" {
		params.Coupon = stripe.String(accounts.service.StripeFreeTierCouponID)
	}

	customer, err := accounts.service.stripeClient.Customers().New(params)
	if err != nil {
		return couponType, Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return couponType, Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
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

	accountBalance := payments.Balance{
		Coins: -c.Balance,
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

		projectPrice := accounts.service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.SegmentCount)

		charges = append(charges, payments.ProjectCharge{
			ProjectUsage: *usage,

			ProjectID:    project.ID,
			Egress:       projectPrice.Egress.IntPart(),
			SegmentCount: projectPrice.Segments.IntPart(),
			StorageGbHrs: projectPrice.Storage.IntPart(),
		})
	}

	return charges, nil
}

// CheckProjectInvoicingStatus returns true if for the given project there are outstanding project records and/or usage
// which have not been applied/invoiced yet (meaning sent over to stripe).
func (accounts *accounts) CheckProjectInvoicingStatus(ctx context.Context, projectID uuid.UUID) (unpaidUsage bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// we do not want to delete projects that have usage for the current month.
	year, month, _ := accounts.service.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	currentUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth, accounts.service.nowFn())
	if err != nil {
		return false, err
	}
	if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.SegmentCount > 0 {
		return true, errors.New("usage for current month exists")
	}

	// if usage of last month exist, make sure to look for billing records
	lastMonthUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
	if err != nil {
		return false, err
	}

	if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.SegmentCount > 0 {
		// time passed into the check function need to be the UTC midnight dates of the first and last day of the month
		err = accounts.service.db.ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.Add(-time.Hour*24))
		if errors.Is(err, ErrProjectRecordExists) {
			record, err := accounts.service.db.ProjectRecords().Get(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.Add(-time.Hour*24))
			if err != nil {
				return true, err
			}
			// state = 0 means unapplied and not invoiced yet.
			if record.State == 0 {
				return true, errors.New("unapplied project invoice record exist")
			}
			// Record has been applied, so project can be deleted.
			return false, nil
		}
		if err != nil {
			return true, err
		}
		return true, errors.New("usage for last month exist, but is not billed yet")
	}
	return false, nil
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
