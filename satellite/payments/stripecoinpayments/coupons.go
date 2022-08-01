// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v72"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// ensures that coupons implements payments.Coupons.
var _ payments.Coupons = (*coupons)(nil)

// coupons is an implementation of payments.Coupons.
//
// architecture: Service
type coupons struct {
	service *Service
}

// ApplyCouponCode attempts to apply a coupon code to the user via Stripe.
func (coupons *coupons) ApplyCouponCode(ctx context.Context, userID uuid.UUID, couponCode string) (_ *payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID, couponCode)(&err)

	promoCodeIter := coupons.service.stripeClient.PromoCodes().List(&stripe.PromotionCodeListParams{
		Code: stripe.String(couponCode),
	})
	if !promoCodeIter.Next() {
		return nil, Error.New("Invalid coupon code")
	}
	promoCode := promoCodeIter.PromotionCode()

	customerID, err := coupons.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		PromotionCode: stripe.String(promoCode.ID),
	}
	params.AddExpand("discount.promotion_code")

	customer, err := coupons.service.stripeClient.Customers().Update(customerID, params)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if customer.Discount == nil || customer.Discount.Coupon == nil {
		return nil, Error.New("invalid discount after coupon code application; user ID:%s, customer ID:%s", userID, customerID)
	}

	return stripeDiscountToPaymentsCoupon(customer.Discount)
}

// GetByUserID returns the coupon applied to the user.
func (coupons *coupons) GetByUserID(ctx context.Context, userID uuid.UUID) (_ *payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := coupons.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{}
	params.AddExpand("discount.promotion_code")

	customer, err := coupons.service.stripeClient.Customers().Get(customerID, params)
	if err != nil {
		return nil, err
	}

	if customer.Discount == nil || customer.Discount.Coupon == nil {
		return nil, nil
	}

	return stripeDiscountToPaymentsCoupon(customer.Discount)
}

// stripeDiscountToPaymentsCoupon converts a Stripe discount to a payments.Coupon.
func stripeDiscountToPaymentsCoupon(dc *stripe.Discount) (coupon *payments.Coupon, err error) {
	if dc == nil {
		return nil, Error.New("discount is nil")
	}

	if dc.Coupon == nil {
		return nil, Error.New("discount.Coupon is nil")
	}

	coupon = &payments.Coupon{
		ID:         dc.ID,
		Name:       dc.Coupon.Name,
		AmountOff:  dc.Coupon.AmountOff,
		PercentOff: dc.Coupon.PercentOff,
		AddedAt:    time.Unix(dc.Start, 0),
		ExpiresAt:  time.Unix(dc.End, 0),
		Duration:   payments.CouponDuration(dc.Coupon.Duration),
	}

	if dc.PromotionCode != nil {
		coupon.PromoCode = dc.PromotionCode.Code
	}

	return coupon, nil
}
