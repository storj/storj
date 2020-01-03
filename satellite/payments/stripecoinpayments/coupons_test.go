// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCouponRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		couponsRepo := db.StripeCoinPayments().Coupons()
		coupon := payments.Coupon{
			Duration:    time.Hour * 24,
			Amount:      10,
			Status:      payments.CouponActive,
			Description: "description",
			ProjectID:   testrand.UUID(),
			UserID:      testrand.UUID(),
		}

		now := time.Now().UTC()

		t.Run("insert", func(t *testing.T) {
			err := couponsRepo.Insert(ctx, coupon)
			assert.NoError(t, err)

			coupons, err := couponsRepo.List(ctx, payments.CouponActive)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(coupons))
			coupon = coupons[0]
		})

		t.Run("update", func(t *testing.T) {
			err := couponsRepo.Update(ctx, coupon.ID, payments.CouponUsed)
			assert.NoError(t, err)

			coupons, err := couponsRepo.List(ctx, payments.CouponUsed)
			assert.NoError(t, err)
			assert.Equal(t, payments.CouponUsed, coupons[0].Status)
			coupon = coupons[0]
		})

		t.Run("get latest on empty table return stripecoinpayments.ErrNoCouponUsages", func(t *testing.T) {
			_, err := couponsRepo.GetLatest(ctx, coupon.ID)
			assert.Error(t, err)
			assert.Equal(t, true, stripecoinpayments.ErrNoCouponUsages.Has(err))
		})

		t.Run("total on empty table returns 0", func(t *testing.T) {
			total, err := couponsRepo.TotalUsage(ctx, coupon.ID)
			assert.NoError(t, err)
			assert.Equal(t, int64(0), total)
		})

		t.Run("add usage", func(t *testing.T) {
			err := couponsRepo.AddUsage(ctx, stripecoinpayments.CouponUsage{
				CouponID: coupon.ID,
				Amount:   1,
				End:      now,
			})
			assert.NoError(t, err)
			date, err := couponsRepo.GetLatest(ctx, coupon.ID)
			assert.NoError(t, err)
			isoMillis := "2006-01-02T15:04:05.000-0700Z"
			// go and postgres has different precision. go - nanoseconds, postgres milli
			assert.Equal(t, date.Format(isoMillis), now.Format(isoMillis))
		})

		t.Run("total usage", func(t *testing.T) {
			amount, err := couponsRepo.TotalUsage(ctx, coupon.ID)
			assert.NoError(t, err)
			assert.Equal(t, amount, int64(1))
		})
	})
}
