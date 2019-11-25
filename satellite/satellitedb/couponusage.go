package satellitedb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that coupons implements payments.CouponsDB.
var _ stripecoinpayments.CouponUsageDB = (*couponUsage)(nil)

// coupon_usage is an implementation of payments.CouponUsageDB.
//
// architecture: Database
type couponUsage struct {
	db *dbx.DB
}

// Insert creates new coupon usage record in database.
func (couponUsage *couponUsage) Insert(ctx context.Context, usage stripecoinpayments.CouponUsage) (err error) {
	defer mon.Task()(&ctx, usage)(&err)

	id, err := uuid.New()
	if err != nil {
		return err
	}

	_, err = couponUsage.db.Create_CouponUsage(
		ctx,
		dbx.CouponUsage_Id(id[:]),
		dbx.CouponUsage_CouponId(usage.CouponID[:]),
		dbx.CouponUsage_Amount(usage.Amount),
		dbx.CouponUsage_PeriodStart(usage.Start),
		dbx.CouponUsage_PeriodEnd(usage.End),
	)

	return err
}

// TotalUsageForPeriod gets sum of all usage records for specified coupon.
func (couponUsage *couponUsage) TotalUsageForPeriod(ctx context.Context, couponID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	query := `SELECT SUM(amount)
			  FROM coupon_usages
			  WHERE coupon_id = ?`

	amountRow := couponUsage.db.QueryRowContext(ctx, query, couponID[:])

	var amount int64
	err = amountRow.Scan(&amount)

	return amount, err
}

// GetLatest return period_end of latest coupon charge.
func (couponUsage *couponUsage) GetLatest(ctx context.Context, couponID uuid.UUID) (_ time.Time, err error) {
	defer mon.Task()(&ctx, couponID)(&err)

	query := `SELECT period_end
			  FROM coupon_usages
			  WHERE coupon_id = ?
			  ORDER BY period_end DESC
			  LIMIT 1`

	amountRow := couponUsage.db.QueryRowContext(ctx, query, couponID[:])

	var created time.Time
	err = amountRow.Scan(&created)

	return created, err
}
