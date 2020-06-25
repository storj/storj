// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// CouponsDB is an interface for managing coupons table.
//
// architecture: Database
type CouponsDB interface {
	// Insert inserts a coupon into the database.
	Insert(ctx context.Context, coupon payments.Coupon) (payments.Coupon, error)
	// Update updates coupon in database.
	Update(ctx context.Context, couponID uuid.UUID, status payments.CouponStatus) (payments.Coupon, error)
	// Get returns coupon by ID.
	Get(ctx context.Context, couponID uuid.UUID) (payments.Coupon, error)
	// Delete removes a coupon from the database
	Delete(ctx context.Context, couponID uuid.UUID) error
	// List returns all coupons with specified status.
	List(ctx context.Context, status payments.CouponStatus) ([]payments.Coupon, error)
	// ListByUserID returns all coupons of specified user.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]payments.Coupon, error)
	// ListByUserIDAndStatus returns all coupons of specified user and status. Results are ordered (asc) by expiration date.
	ListByUserIDAndStatus(ctx context.Context, userID uuid.UUID, status payments.CouponStatus) ([]payments.Coupon, error)
	// ListPending returns paginated list of coupons with specified status.
	ListPaged(ctx context.Context, offset int64, limit int, before time.Time, status payments.CouponStatus) (payments.CouponsPage, error)

	// AddUsage creates new coupon usage record in database.
	AddUsage(ctx context.Context, usage CouponUsage) error
	// TotalUsage gets sum of all usage records for specified coupon.
	TotalUsage(ctx context.Context, couponID uuid.UUID) (int64, error)
	// GetLatest return period_end of latest coupon charge.
	GetLatest(ctx context.Context, couponID uuid.UUID) (time.Time, error)
	// ListUnapplied returns coupon usage page with unapplied coupon usages.
	ListUnapplied(ctx context.Context, offset int64, limit int, period time.Time) (CouponUsagePage, error)
	// ApplyUsage applies coupon usage and updates its status.
	ApplyUsage(ctx context.Context, couponID uuid.UUID, period time.Time) error

	// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have a project
	// and do not have a promotional coupon yet. And updates project limits to selected size.
	PopulatePromotionalCoupons(ctx context.Context, users []uuid.UUID, duration int, amount int64, projectLimit memory.Size) error
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

// ensures that coupons implements payments.Coupons.
var _ payments.Coupons = (*coupons)(nil)

// coupons is an implementation of payments.Coupons.
//
// architecture: Service
type coupons struct {
	service *Service
}

// Create attaches a coupon for payment account.
func (coupons *coupons) Create(ctx context.Context, coupon payments.Coupon) (coup payments.Coupon, err error) {
	defer mon.Task()(&ctx, coupon)(&err)

	coup, err = coupons.service.db.Coupons().Insert(ctx, coupon)

	return coup, Error.Wrap(err)
}

// ListByUserID return list of all coupons of specified payment account.
func (coupons *coupons) ListByUserID(ctx context.Context, userID uuid.UUID) (_ []payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	couponList, err := coupons.service.db.Coupons().ListByUserID(ctx, userID)

	return couponList, Error.Wrap(err)
}

// TotalUsage returns sum of all usage records for specified coupon.
func (coupons *coupons) TotalUsage(ctx context.Context, couponID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	totalUsage, err := coupons.service.db.Coupons().TotalUsage(ctx, couponID)

	return totalUsage, Error.Wrap(err)
}

// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have
// a project, payment method and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (coupons *coupons) PopulatePromotionalCoupons(ctx context.Context, duration int, amount int64, projectLimit memory.Size) (err error) {
	defer mon.Task()(&ctx, duration, amount, projectLimit)(&err)

	const limit = 50
	before := time.Now()

	cusPage, err := coupons.service.db.Customers().List(ctx, 0, limit, before)
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

		paymentMethodsIterator := coupons.service.stripeClient.PaymentMethods().List(params)
		for paymentMethodsIterator.Next() {
			// if user has at least 1 payment method - break a loop.
			usersIDs = append(usersIDs, cus.UserID)
			break
		}

		if err = paymentMethodsIterator.Err(); err != nil {
			return Error.Wrap(err)
		}
	}

	err = coupons.service.db.Coupons().PopulatePromotionalCoupons(ctx, usersIDs, duration, amount, projectLimit)
	if err != nil {
		return Error.Wrap(err)
	}

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		cusPage, err = coupons.service.db.Customers().List(ctx, cusPage.NextOffset, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		// we have to wait before each iteration because
		// Stripe has rate limits - 100 read and 100 write operations per second per secret key.
		time.Sleep(time.Second)

		var usersIDs []uuid.UUID
		for _, cus := range cusPage.Customers {
			params := &stripe.PaymentMethodListParams{
				Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
				Customer: stripe.String(cus.ID),
			}

			paymentMethodsIterator := coupons.service.stripeClient.PaymentMethods().List(params)
			for paymentMethodsIterator.Next() {
				usersIDs = append(usersIDs, cus.UserID)
				break
			}

			if err = paymentMethodsIterator.Err(); err != nil {
				return Error.Wrap(err)
			}
		}

		err = coupons.service.db.Coupons().PopulatePromotionalCoupons(ctx, usersIDs, duration, amount, projectLimit)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// AddPromotionalCoupon is used to add a promotional coupon for specified users who already have
// a project and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (coupons *coupons) AddPromotionalCoupon(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return Error.Wrap(coupons.service.db.Coupons().PopulatePromotionalCoupons(ctx, []uuid.UUID{userID}, int(coupons.service.CouponDuration), coupons.service.CouponValue, coupons.service.CouponProjectLimit))
}
