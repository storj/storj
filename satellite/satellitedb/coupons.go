// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that coupons implements payments.CouponsDB.
var _ stripecoinpayments.CouponsDB = (*coupons)(nil)

// coupons is an implementation of payments.CouponsDB.
//
// architecture: Database
type coupons struct {
	db *dbx.DB
}

// Insert inserts a coupon into the database.
func (coupons *coupons) Insert(ctx context.Context, coupon payments.Coupon) (err error) {
	defer mon.Task()(&ctx, coupon)(&err)

	id, err := uuid.New()
	if err != nil {
		return err
	}

	_, err = coupons.db.Create_Coupon(
		ctx,
		dbx.Coupon_Id(id[:]),
		dbx.Coupon_ProjectId(coupon.ProjectID[:]),
		dbx.Coupon_UserId(coupon.UserID[:]),
		dbx.Coupon_Amount(coupon.Amount),
		dbx.Coupon_Description(coupon.Description),
		dbx.Coupon_Status(int(coupon.Status)),
		dbx.Coupon_Duration(int64(coupon.Duration)),
	)

	return err
}

// Update updates coupon in database.
func (coupons *coupons) Update(ctx context.Context, couponID uuid.UUID, status payments.CouponStatus) (err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	_, err = coupons.db.Update_Coupon_By_Id(
		ctx,
		dbx.Coupon_Id(couponID[:]),
		dbx.Coupon_Update_Fields{
			Status: dbx.Coupon_Status(int(status)),
		},
	)

	return err
}

// List returns all coupons of specified user.
func (coupons *coupons) ListByUserID(ctx context.Context, userID uuid.UUID) (_ []payments.Coupon, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	dbxCoupons, err := coupons.db.All_Coupon_By_UserId_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.Coupon_UserId(userID[:]),
	)
	if err != nil {
		return nil, err
	}

	return couponsFromDbxSlice(dbxCoupons)
}

// List returns all coupons with specified status.
func (coupons *coupons) List(ctx context.Context, status payments.CouponStatus) (_ []payments.Coupon, err error) {
	defer mon.Task()(&ctx, status)(&err)

	dbxCoupons, err := coupons.db.All_Coupon_By_Status_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.Coupon_Status(int(status)),
	)
	if err != nil {
		return nil, err
	}

	return couponsFromDbxSlice(dbxCoupons)
}

// ListPending returns paginated list of pending transactions.
func (coupons *coupons) ListPaged(ctx context.Context, offset int64, limit int, before time.Time, status payments.CouponStatus) (_ payments.CouponsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page payments.CouponsPage

	dbxCoupons, err := coupons.db.Limited_Coupon_By_CreatedAt_LessOrEqual_And_Status_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.Coupon_CreatedAt(before.UTC()),
		dbx.Coupon_Status(coinpayments.StatusPending.Int()),
		limit+1,
		offset,
	)
	if err != nil {
		return payments.CouponsPage{}, err
	}

	if len(dbxCoupons) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit) + 1

		dbxCoupons = dbxCoupons[:len(dbxCoupons)-1]
	}

	page.Coupons, err = couponsFromDbxSlice(dbxCoupons)
	if err != nil {
		return payments.CouponsPage{}, nil
	}

	return page, nil
}

// fromDBXCoupon converts *dbx.Coupon to *payments.Coupon.
func fromDBXCoupon(dbxCoupon *dbx.Coupon) (coupon payments.Coupon, err error) {
	coupon.UserID, err = dbutil.BytesToUUID(dbxCoupon.UserId)
	if err != nil {
		return payments.Coupon{}, err
	}

	coupon.ProjectID, err = dbutil.BytesToUUID(dbxCoupon.ProjectId)
	if err != nil {
		return payments.Coupon{}, err
	}

	coupon.ID, err = dbutil.BytesToUUID(dbxCoupon.Id)
	if err != nil {
		return payments.Coupon{}, err
	}

	coupon.Duration = time.Duration(dbxCoupon.Duration)
	coupon.Description = dbxCoupon.Description
	coupon.Amount = dbxCoupon.Amount
	coupon.Created = dbxCoupon.CreatedAt
	coupon.Status = payments.CouponStatus(dbxCoupon.Status)

	return coupon, nil
}

// AddUsage creates new coupon usage record in database.
func (coupons *coupons) AddUsage(ctx context.Context, usage stripecoinpayments.CouponUsage) (err error) {
	defer mon.Task()(&ctx, usage)(&err)

	id, err := uuid.New()
	if err != nil {
		return err
	}

	_, err = coupons.db.Create_CouponUsage(
		ctx,
		dbx.CouponUsage_Id(id[:]),
		dbx.CouponUsage_CouponId(usage.CouponID[:]),
		dbx.CouponUsage_Amount(usage.Amount),
		dbx.CouponUsage_IntervalEnd(usage.End),
	)

	return err
}

// TotalUsage gets sum of all usage records for specified coupon.
func (coupons *coupons) TotalUsage(ctx context.Context, couponID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	query := coupons.db.Rebind(
		`SELECT COALESCE(SUM(amount), 0)
			  FROM coupon_usages
			  WHERE coupon_id = ?;`,
	)

	amountRow := coupons.db.QueryRowContext(ctx, query, couponID[:])

	var amount int64
	err = amountRow.Scan(&amount)

	return amount, err
}

// GetLatest return period_end of latest coupon charge.
func (coupons *coupons) GetLatest(ctx context.Context, couponID uuid.UUID) (_ time.Time, err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	query := coupons.db.Rebind(
		`SELECT interval_end 
			  FROM coupon_usages 
			  WHERE coupon_id = ? 
			  ORDER BY interval_end DESC
			  LIMIT 1;`,
	)

	amountRow := coupons.db.QueryRowContext(ctx, query, couponID[:])

	var created time.Time
	err = amountRow.Scan(&created)
	if err == sql.ErrNoRows {
		return created, stripecoinpayments.ErrNoCouponUsages.Wrap(err)
	}

	return created.UTC(), err
}

// couponsFromDbxSlice is used for creating []payments.Coupon entities from autogenerated []dbx.Coupon struct.
func couponsFromDbxSlice(couponsDbx []*dbx.Coupon) (_ []payments.Coupon, err error) {
	var coupons = make([]payments.Coupon, 0)
	var errors []error

	// Generating []dbo from []dbx and collecting all errors
	for _, couponDbx := range couponsDbx {
		coupon, err := fromDBXCoupon(couponDbx)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		coupons = append(coupons, coupon)
	}

	return coupons, errs.Combine(errors...)
}
