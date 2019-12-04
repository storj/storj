// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments"
)

// CouponsDB is an interface for managing coupons table.
//
// architecture: Database
type CouponsDB interface {
	// Insert inserts a coupon into the database.
	Insert(ctx context.Context, coupon payments.Coupon) error
	// Update updates coupon in database.
	Update(ctx context.Context, couponID uuid.UUID, status payments.CouponStatus) error
	// List returns all coupons with specified status.
	List(ctx context.Context, status payments.CouponStatus) (_ []payments.Coupon, err error)
	// List returns all coupons of specified user.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]payments.Coupon, error)
	// ListPending returns paginated list of coupons with specified status.
	ListPaged(ctx context.Context, offset int64, limit int, before time.Time, status payments.CouponStatus) (payments.CouponsPage, error)

	// AddUsage creates new coupon usage record in database.
	AddUsage(ctx context.Context, usage CouponUsage) error
	// TotalUsage gets sum of all usage records for specified coupon.
	TotalUsage(ctx context.Context, couponID uuid.UUID) (_ int64, err error)
	// GetLatest return period_end of latest coupon charge.
	GetLatest(ctx context.Context, couponID uuid.UUID) (time.Time, error)
}

// CouponUsage stores amount of money that should be charged from coupon for some period.
type CouponUsage struct {
	ID       uuid.UUID
	CouponID uuid.UUID
	Amount   int64
	End      time.Time
}
