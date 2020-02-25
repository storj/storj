// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/common/memory"
)

// Coupons exposes all needed functionality to manage coupons.
//
// architecture: Service
type Coupons interface {
	// ListByUserID return list of all coupons of specified payment account.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]Coupon, error)

	// Create attaches a coupon for payment account.
	Create(ctx context.Context, coupon Coupon) (err error)

	// AddPromotionalCoupon is used to add a promotional coupon for specified users who already have
	// a project and do not have a promotional coupon yet.
	// And updates project limits to selected size.
	AddPromotionalCoupon(ctx context.Context, userID uuid.UUID, duration int, amount int64, projectLimit memory.Size) error

	// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have
	// a project, payment method and do not have a promotional coupon yet.
	// And updates project limits to selected size.
	PopulatePromotionalCoupons(ctx context.Context, duration int, amount int64, projectLimit memory.Size) error
}

// Coupon is an entity that adds some funds to Accounts balance for some fixed period.
// Coupon is attached to the project.
// At the end of the period, the entire remaining coupon amount will be returned from the account balance.
type Coupon struct {
	ID          uuid.UUID    `json:"id"`
	UserID      uuid.UUID    `json:"userId"`
	ProjectID   uuid.UUID    `json:"projectId"`
	Amount      int64        `json:"amount"`   // Amount is stored in cents.
	Duration    int          `json:"duration"` // Duration is stored in number ob billing periods.
	Description string       `json:"description"`
	Type        CouponType   `json:"type"`
	Status      CouponStatus `json:"status"`
	Created     time.Time    `json:"created"`
}

// IsExpired checks if coupon is not after its rollup period.
func (coupon *Coupon) IsExpired() bool {
	expirationDate := time.Date(coupon.Created.Year(), coupon.Created.Month(), 1, 0, 0, 0, 0, coupon.Created.Location())
	expirationDate = expirationDate.AddDate(0, coupon.Duration, 0)

	now := time.Now().UTC()
	now = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, coupon.Created.Location())

	return expirationDate.Before(now)
}

// CouponType indicates the type of the coupon.
type CouponType int

const (
	// CouponTypePromotional defines that this coupon is a promotional coupon.
	// Promotional coupon is added only once per account.
	CouponTypePromotional CouponType = 0
)

// CouponStatus indicates the state of the coupon.
type CouponStatus int

const (
	// CouponActive is a default coupon state.
	CouponActive CouponStatus = 0
	// CouponUsed status indicates that coupon was used.
	CouponUsed CouponStatus = 1
	// CouponExpired status indicates that coupon is expired and unavailable.
	CouponExpired CouponStatus = 2
)

// CouponsPage holds set of coupon and indicates if
// there are more coupons to fetch.
type CouponsPage struct {
	Coupons    []Coupon
	Next       bool
	NextOffset int64
}
