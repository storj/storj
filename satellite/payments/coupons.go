// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	// ErrInvalidCoupon defines invalid coupon code error.
	ErrInvalidCoupon = errs.Class("invalid coupon code")
	// ErrCouponConflict occurs when attempting to replace a protected coupon.
	ErrCouponConflict = errs.Class("coupon conflict")
)

// Coupons exposes all needed functionality to manage coupons.
//
// architecture: Service
type Coupons interface {
	// GetByUserID returns the coupon applied to the specified user.
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Coupon, error)
	// ApplyFreeTierCoupon applies the free tier coupon to the specified user.
	ApplyFreeTierCoupon(ctx context.Context, userID uuid.UUID) (*Coupon, error)
	// ApplyCoupon applies coupon to user based on coupon ID.
	ApplyCoupon(ctx context.Context, userID uuid.UUID, couponID string) (*Coupon, error)
	// ApplyCouponCode attempts to apply a coupon code to the user.
	ApplyCouponCode(ctx context.Context, userID uuid.UUID, couponCode string) (*Coupon, error)
}

// Coupon describes a discount to the payment account of a user.
type Coupon struct {
	ID         string         `json:"id"`
	PromoCode  string         `json:"promoCode"`
	Name       string         `json:"name"`
	AmountOff  int64          `json:"amountOff"`
	PercentOff float64        `json:"percentOff"`
	AddedAt    time.Time      `json:"addedAt"`
	ExpiresAt  time.Time      `json:"expiresAt"`
	Duration   CouponDuration `json:"duration"`
}

// CouponDuration represents how many billing periods a coupon is applied.
type CouponDuration string

const (
	// CouponOnce indicates that a coupon can only be applied once.
	CouponOnce CouponDuration = "once"
	// CouponRepeating indicates that a coupon is applied every billing period for a definite amount of time.
	CouponRepeating = "repeating"
	// CouponForever indicates that a coupon is applied every billing period forever.
	CouponForever = "forever"
)

// CouponType is an enum representing the outcome a coupon validation check.
type CouponType string

const (
	// NoCoupon represents an invalid coupon registration attempt.
	NoCoupon CouponType = "noCoupon"
	// FreeTierCoupon represents the default free tier coupon.
	FreeTierCoupon = "freeTierCoupon"
	// SignupCoupon represents a valid promo code coupon.
	SignupCoupon = "signupCoupon"
)
