// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
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

// Balances exposes all needed functionality to manage account balances.
func (accounts *accounts) Balances() payments.Balances {
	return &balances{service: accounts.service}
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
		Params: stripe.Params{Context: ctx},
		Email:  stripe.String(email),
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
		ListParams: stripe.ListParams{Context: ctx},
		Code:       stripe.String(signupPromoCode),
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

// UpdatePackage updates a customer's package plan information.
func (accounts *accounts) UpdatePackage(ctx context.Context, userID uuid.UUID, packagePlan *string, timestamp *time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = accounts.service.db.Customers().UpdatePackage(ctx, userID, packagePlan, timestamp)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetPackageInfo returns the package plan and time of purchase for a user.
func (accounts *accounts) GetPackageInfo(ctx context.Context, userID uuid.UUID) (packagePlan *string, purchaseTime *time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	packagePlan, purchaseTime, err = accounts.service.db.Customers().GetPackageInfo(ctx, userID)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return
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
		totalUsage := accounting.ProjectUsage{Since: since, Before: before}

		usages, err := accounts.service.usageDB.GetProjectTotalByPartner(ctx, project.ID, accounts.service.partnerNames, since, before)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		var totalPrice projectUsagePrice

		for partner, usage := range usages {
			priceModel := accounts.GetProjectUsagePriceModel(partner)
			price := accounts.service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.SegmentCount, priceModel)

			totalPrice.Egress = totalPrice.Egress.Add(price.Egress)
			totalPrice.Segments = totalPrice.Segments.Add(price.Segments)
			totalPrice.Storage = totalPrice.Storage.Add(price.Storage)

			totalUsage.Egress += usage.Egress
			totalUsage.ObjectCount += usage.ObjectCount
			totalUsage.SegmentCount += usage.SegmentCount
			totalUsage.Storage += usage.Storage
		}

		charges = append(charges, payments.ProjectCharge{
			ProjectUsage: totalUsage,

			ProjectID:    project.PublicID,
			Egress:       totalPrice.Egress.IntPart(),
			SegmentCount: totalPrice.Segments.IntPart(),
			StorageGbHrs: totalPrice.Storage.IntPart(),
		})
	}

	return charges, nil
}

// GetProjectUsagePriceModel returns the project usage price model for a partner name.
func (accounts *accounts) GetProjectUsagePriceModel(partner string) payments.ProjectUsagePriceModel {
	if override, ok := accounts.service.usagePriceOverrides[partner]; ok {
		return override
	}
	return accounts.service.usagePrices
}

// CheckProjectInvoicingStatus returns error if for the given project there are outstanding project records and/or usage
// which have not been applied/invoiced yet (meaning sent over to stripe).
func (accounts *accounts) CheckProjectInvoicingStatus(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	year, month, _ := accounts.service.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Check if an invoice project record exists already
	err = accounts.service.db.ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
	if errs.Is(err, ErrProjectRecordExists) {
		record, err := accounts.service.db.ProjectRecords().Get(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
		if err != nil {
			return err
		}
		// state = 0 means unapplied and not invoiced yet.
		if record.State == 0 {
			return errs.New("unapplied project invoice record exist")
		}
		// Record has been applied, so project can be deleted.
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

// CheckProjectUsageStatus returns error if for the given project there is some usage for current or previous month.
func (accounts *accounts) CheckProjectUsageStatus(ctx context.Context, projectID uuid.UUID) error {
	var err error
	defer mon.Task()(&ctx)(&err)

	year, month, _ := accounts.service.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// check current month usage and do not allow deletion if usage exists
	currentUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth, accounts.service.nowFn())
	if err != nil {
		return err
	}
	if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.SegmentCount > 0 {
		return errs.New("usage for current month exists")
	}

	// check usage for last month, if exists, ensure we have an invoice item created.
	lastMonthUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
	if err != nil {
		return err
	}
	if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.SegmentCount > 0 {
		err = accounts.service.db.ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
		if !errs.Is(err, ErrProjectRecordExists) {
			return errs.New("usage for last month exist, but is not billed yet")
		}
	}

	return nil
}

// Charges returns list of all credit card charges related to account.
func (accounts *accounts) Charges(ctx context.Context, userID uuid.UUID) (_ []payments.Charge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.ChargeListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   stripe.String(customerID),
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
