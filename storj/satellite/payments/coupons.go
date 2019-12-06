// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Coupon is an entity that adds some funds to Accounts balance for some fixed period.
// Coupon is attached to the project.
// At the end of the period, the entire remaining coupon amount will be returned from the account balance.
type Coupon struct {
	ID          uuid.UUID     `json:"id"`
	UserID      uuid.UUID     `json:"userId"`
	ProjectID   uuid.UUID     `json:"projectId"`
	Amount      int64         `json:"amount"`   // Amount is stored in cents.
	Duration    time.Duration `json:"duration"` // Duration is stored in days.
	Description string        `json:"description"`
	Status      CouponStatus  `json:"status"`
	Created     time.Time     `json:"created"`
}

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
