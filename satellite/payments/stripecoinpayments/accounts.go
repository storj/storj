// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"

	"storj.io/common/memory"
	"storj.io/storj/private/date"
	"storj.io/storj/satellite/payments"
)

// ensures that accounts implements payments.Accounts.
var _ payments.Accounts = (*accounts)(nil)

// accounts is an implementation of payments.Accounts.
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

	customer, err := accounts.service.stripeClient.Customers.New(params)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	c, err := accounts.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	// add all active coupons amount to balance.
	coupons, err := accounts.service.db.Coupons().ListByUserIDAndStatus(ctx, userID, payments.CouponActive)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	var couponsAmount int64 = 0
	for _, coupon := range coupons {
		alreadyUsed, err := accounts.service.db.Coupons().TotalUsage(ctx, coupon.ID)
		if err != nil {
			return 0, Error.Wrap(err)
		}

		couponsAmount += coupon.Amount - alreadyUsed
	}

	return -c.Balance + couponsAmount, nil
}

// AddCoupon attaches a coupon for payment account.
func (accounts *accounts) AddCoupon(ctx context.Context, userID, projectID uuid.UUID, amount int64, duration int, description string, couponType payments.CouponType) (err error) {
	defer mon.Task()(&ctx, userID, amount, duration, description, couponType)(&err)

	coupon := payments.Coupon{
		UserID:      userID,
		Status:      payments.CouponActive,
		ProjectID:   projectID,
		Amount:      amount,
		Description: description,
		Duration:    duration,
		Type:        couponType,
	}

	return Error.Wrap(accounts.service.db.Coupons().Insert(ctx, coupon))
}

// ProjectCharges returns how much money current user will be charged for each project.
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID) (charges []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	// to return empty slice instead of nil if there are no projects
	charges = make([]payments.ProjectCharge, 0)

	projects, err := accounts.service.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	start, end := date.MonthBoundary(time.Now().UTC())

	// TODO: we should improve performance of this block of code. It takes ~4-5 sec to get project charges.
	for _, project := range projects {
		usage, err := accounts.service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return charges, Error.Wrap(err)
		}

		projectPrice := accounts.service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount)

		charges = append(charges, payments.ProjectCharge{
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

	iter := accounts.service.stripeClient.Charges.List(params)

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

// Coupons return list of all coupons of specified payment account.
func (accounts *accounts) Coupons(ctx context.Context, userID uuid.UUID) (coupons []payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	coupons, err = accounts.service.db.Coupons().ListByUserID(ctx, userID)

	return coupons, Error.Wrap(err)
}

// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have
// a project, payment method and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (accounts *accounts) PopulatePromotionalCoupons(ctx context.Context, duration int, amount int64, projectLimit memory.Size) (err error) {
	defer mon.Task()(&ctx, duration, amount, projectLimit)(&err)

	const limit = 50
	before := time.Now()

	cusPage, err := accounts.service.db.Customers().List(ctx, 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	// taking only users that attached a payment method.
	var usersIDs []uuid.UUID
	for _, cus := range cusPage.Customers {
		params := &stripe.PaymentMethodListParams{
			Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
			Customer: stripe.String(cus.ID),
		}

		paymentMethodsIterator := accounts.service.stripeClient.PaymentMethods.List(params)
		for paymentMethodsIterator.Next() {
			// if user has at least 1 payment method - break a loop.
			usersIDs = append(usersIDs, cus.UserID)
			break
		}

		if err = paymentMethodsIterator.Err(); err != nil {
			return Error.Wrap(err)
		}
	}

	err = accounts.service.db.Coupons().PopulatePromotionalCoupons(ctx, usersIDs, duration, amount, projectLimit)
	if err != nil {
		return Error.Wrap(err)
	}

	for cusPage.Next {
		// we have to wait before each iteration because
		// Stripe has rate limits - 100 read and 100 write operations per second per secret key.
		time.Sleep(time.Second)

		var usersIDs []uuid.UUID
		for _, cus := range cusPage.Customers {
			params := &stripe.PaymentMethodListParams{
				Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
				Customer: stripe.String(cus.ID),
			}

			paymentMethodsIterator := accounts.service.stripeClient.PaymentMethods.List(params)
			for paymentMethodsIterator.Next() {
				usersIDs = append(usersIDs, cus.UserID)
				break
			}

			if err = paymentMethodsIterator.Err(); err != nil {
				return Error.Wrap(err)
			}
		}

		err = accounts.service.db.Coupons().PopulatePromotionalCoupons(ctx, usersIDs, duration, amount, projectLimit)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// StorjTokens exposes all storj token related functionality.
func (accounts *accounts) StorjTokens() payments.StorjTokens {
	return &storjTokens{service: accounts.service}
}
