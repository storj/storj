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
	// Get returns coupon by ID.
	Get(ctx context.Context, couponID uuid.UUID) (payments.Coupon, error)
	// List returns all coupons with specified status.
	List(ctx context.Context, status payments.CouponStatus) ([]payments.Coupon, error)
	// ListByUserID returns all coupons of specified user.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]payments.Coupon, error)
	// ListByUserIDAndStatus returns all coupons of specified user and status.
	ListByUserIDAndStatus(ctx context.Context, userID uuid.UUID, status payments.CouponStatus) ([]payments.Coupon, error)
	// ListByProjectID returns all active coupons for specified project.
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]payments.Coupon, error)
	// ListPending returns paginated list of coupons with specified status.
	ListPaged(ctx context.Context, offset int64, limit int, before time.Time, status payments.CouponStatus) (payments.CouponsPage, error)

	// AddUsage creates new coupon usage record in database.
	AddUsage(ctx context.Context, usage CouponUsage) error
	// TotalUsage gets sum of all usage records for specified coupon.
	TotalUsage(ctx context.Context, couponID uuid.UUID) (int64, error)
	// GetLatest return period_end of latest coupon charge.
	GetLatest(ctx context.Context, couponID uuid.UUID) (time.Time, error)
	// ListUnapplied returns coupon usage page with unapplied coupon usages.
	ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (CouponUsagePage, error)
	// ApplyUsage applies coupon usage and updates its status.
	ApplyUsage(ctx context.Context, couponID uuid.UUID, period time.Time) error
}

// CouponUsage stores amount of money that should be charged from coupon for billing period.
type CouponUsage struct {
	CouponID uuid.UUID
	Amount   int64
	Status   CouponUsageStatus
	Period   time.Time
}

// CouponUsageStatus indicates the state of the coupon usage.
type CouponUsageStatus int

const (
	// CouponUsageStatusUnapplied is a default coupon usage state.
	CouponUsageStatusUnapplied CouponUsageStatus = 0
	// CouponUsageStatusApplied status indicates that coupon usage was used.
	CouponUsageStatusApplied CouponUsageStatus = 1
)

// CouponUsagePage holds coupons usages and
// indicates if there is more data available
// and provides next offset.
type CouponUsagePage struct {
	Usages     []CouponUsage
	Next       bool
	NextOffset int64
}
