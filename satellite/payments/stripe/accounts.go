// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"
	"time"

	"github.com/stripe/stripe-go/v75"
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
			stripeErr := &stripe.Error{}
			if errors.As(err, &stripeErr) {
				err = errs.Wrap(errors.New(stripeErr.Msg))
			}
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
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return couponType, Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return couponType, Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
}

// EnsureUserHasCustomer creates a stripe customer for userID if non exists.
func (accounts *accounts) EnsureUserHasCustomer(ctx context.Context, userID uuid.UUID, email string, signupPromoCode string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	_, err = accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrNoCustomer) {
			return Error.Wrap(err)
		}

		_, err = accounts.Setup(ctx, userID, email, signupPromoCode)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// ChangeEmail changes a customer's email address.
func (accounts *accounts) ChangeEmail(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	cusID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
		Email:  stripe.String(email),
	}

	_, err = accounts.service.stripeClient.Customers().Update(cusID, params)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return Error.Wrap(err)
	}

	return nil
}

// SaveBillingAddress saves billing address for a user and returns the updated billing information.
func (accounts *accounts) SaveBillingAddress(ctx context.Context, userID uuid.UUID, address payments.BillingAddress) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	customerParams := &stripe.CustomerParams{
		Params: stripe.Params{
			Context: ctx,
		},
		Name: &address.Name,
		Address: &stripe.AddressParams{
			Line1:      stripe.String(address.Line1),
			Line2:      stripe.String(address.Line2),
			City:       stripe.String(address.City),
			PostalCode: stripe.String(address.PostalCode),
			State:      stripe.String(address.State),
			Country:    stripe.String(string(address.Country.Code)),
		},
	}
	customerParams.AddExpand("tax_ids")

	customer, err := accounts.service.stripeClient.Customers().Update(customerID, customerParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return nil, Error.Wrap(err)
	}

	return accounts.unpackBillingInformation(*customer)
}

// AddTaxID adds a new tax ID for a user and returns the updated billing information.
func (accounts *accounts) AddTaxID(ctx context.Context, userID uuid.UUID, taxID payments.TaxID) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	taxIDParams := stripe.TaxIDParams{
		Params: stripe.Params{
			Context: ctx,
		},
		Customer: &customerID,
		Type:     stripe.String(string(taxID.Tax.Code)),
		Value:    &taxID.Value,
	}
	_, err = accounts.service.stripeClient.TaxIDs().New(&taxIDParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			if stripeErr.Code == stripe.ErrorCodeTaxIDInvalid {
				err = Error.Wrap(payments.ErrInvalidTaxID.New("Tax validation error: %s", stripeErr.Msg))
			} else {
				err = errs.Wrap(errors.New(stripeErr.Msg))
			}
		}
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
	}
	params.AddExpand("tax_ids")
	customer, err := accounts.service.stripeClient.Customers().Get(customerID, params)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return accounts.unpackBillingInformation(*customer)
}

// RemoveTaxID removes a tax ID from a user and returns the updated billing information.
func (accounts *accounts) RemoveTaxID(ctx context.Context, userID uuid.UUID, id string) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = accounts.service.stripeClient.TaxIDs().Del(id, &stripe.TaxIDParams{
		Params: stripe.Params{
			Context: ctx,
		},
		Customer: &customerID,
	})
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
	}
	params.AddExpand("tax_ids")
	customer, err := accounts.service.stripeClient.Customers().Get(customerID, params)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return accounts.unpackBillingInformation(*customer)
}

// GetBillingInformation gets the billing information for a user.
func (accounts *accounts) GetBillingInformation(ctx context.Context, userID uuid.UUID) (info *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
	}
	params.AddExpand("tax_ids")
	customer, err := accounts.service.stripeClient.Customers().Get(customerID, params)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return nil, Error.Wrap(err)
	}

	return accounts.unpackBillingInformation(*customer)
}

func (accounts *accounts) unpackBillingInformation(customer stripe.Customer) (info *payments.BillingInformation, err error) {

	// use customer.address to determine if the customer has custom billing information.
	hasNoAddress := customer.Address == nil || customer.Address == (&stripe.Address{})
	hasNoTaxInfo := customer.TaxIDs == nil || len(customer.TaxIDs.Data) == 0
	if hasNoAddress && hasNoTaxInfo {
		return &payments.BillingInformation{}, nil
	}

	var address *payments.BillingAddress
	taxIDs := make([]payments.TaxID, 0)
	if !hasNoAddress {
		stripeAddr := customer.Address
		countryCode := payments.CountryCode(stripeAddr.Country)
		var country payments.TaxCountry
		for _, taxCountry := range payments.TaxCountries {
			if taxCountry.Code == countryCode {
				country = taxCountry
				break
			}
		}
		if (country == payments.TaxCountry{}) {
			// if country is not found in the list of tax countries, use the country code as the name
			country.Name = stripeAddr.Country
			country.Code = countryCode
		}
		address = &payments.BillingAddress{
			Name:       customer.Name,
			Line1:      stripeAddr.Line1,
			Line2:      stripeAddr.Line2,
			City:       stripeAddr.City,
			PostalCode: stripeAddr.PostalCode,
			State:      stripeAddr.State,
			Country:    country,
		}
	}
	if !hasNoTaxInfo {
		for _, taxID := range customer.TaxIDs.Data {
			var tax payments.Tax
			for _, t := range payments.Taxes {
				if t.Code == taxID.Type {
					tax = t
					break
				}
			}
			taxIDs = append(taxIDs, payments.TaxID{
				ID:    taxID.ID,
				Tax:   tax,
				Value: taxID.Value,
			})
		}
	}

	return &payments.BillingInformation{
		Address: address,
		TaxIDs:  taxIDs,
	}, nil
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
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID, since, before time.Time) (charges payments.ProjectChargesResponse, err error) {
	defer mon.Task()(&ctx, userID, since, before)(&err)

	charges = make(payments.ProjectChargesResponse)

	projects, err := accounts.service.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, project := range projects {
		usages, err := accounts.service.usageDB.GetProjectTotalByPartner(ctx, project.ID, accounts.service.partnerNames, since, before)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		partnerCharges := make(map[string]payments.ProjectCharge)

		for partner, usage := range usages {
			priceModel := accounts.GetProjectUsagePriceModel(partner)
			usage.Egress = applyEgressDiscount(usage, priceModel)
			price := accounts.service.calculateProjectUsagePrice(usage, priceModel)

			partnerCharges[partner] = payments.ProjectCharge{
				ProjectUsage: usage,

				EgressMBCents:       price.Egress.IntPart(),
				SegmentMonthCents:   price.Segments.IntPart(),
				StorageMBMonthCents: price.Storage.IntPart(),
			}
		}

		// to return unpartnered empty charge if there's no usage
		if len(partnerCharges) == 0 {
			partnerCharges[""] = payments.ProjectCharge{
				ProjectUsage: accounting.ProjectUsage{Since: since, Before: before},
			}
		}

		charges[project.PublicID] = partnerCharges
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
func (accounts *accounts) CheckProjectUsageStatus(ctx context.Context, projectID uuid.UUID) (currentUsageExists, invoicingIncomplete bool, err error) {
	defer mon.Task()(&ctx)(&err)

	year, month, _ := accounts.service.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// check current month usage and do not allow deletion if usage exists
	currentUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth, accounts.service.nowFn())
	if err != nil {
		return false, false, err
	}
	if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.SegmentCount > 0 {
		return true, false, payments.ErrUnbilledUsageCurrentMonth
	}

	// check usage for last month, if exists, ensure we have an invoice item created.
	lastMonthUsage, err := accounts.service.usageDB.GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
	if err != nil {
		return false, false, err
	}
	if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.SegmentCount > 0 {
		err = accounts.service.db.ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
		if !errs.Is(err, ErrProjectRecordExists) {
			return false, true, payments.ErrUnbilledUsageLastMonth
		}
	}

	return false, false, nil
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
