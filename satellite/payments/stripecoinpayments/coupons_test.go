// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
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
			Description: "qwe",
			ProjectID:   testrand.UUID(),
			UserID:      testrand.UUID(),
		}

		t.Run("Insertion", func(t *testing.T) {
			err := couponsRepo.Insert(ctx, coupon)
			assert.NoError(t, err)

			coupons, err := couponsRepo.List(ctx, payments.CouponActive)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(coupons))
			//coupon = coupons[0]
		})

		t.Run("update", func(t *testing.T) {
			err := couponsRepo.Update(ctx, coupon.ID, payments.CouponUsed)
			assert.NoError(t, err)

			_, err = couponsRepo.List(ctx, payments.CouponActive)
			assert.NoError(t, err)
			//assert.Equal(t, payments.CouponUsed, coupons[0].Status)
			//coupon = coupons[0]
		})
	})
}

func TestCouponUsageRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		couponUsage := db.StripeCoinPayments().CouponUsage()

		coupon := payments.Coupon{
			ID:          testrand.UUID(),
			Duration:    time.Hour * 24,
			Amount:      10,
			Status:      payments.CouponActive,
			Description: "qwe",
			ProjectID:   testrand.UUID(),
			UserID:      testrand.UUID(),
		}

		t.Run("Get latest on empty table", func(t *testing.T) {
			_, err := couponUsage.GetLatest(ctx, coupon.ID)
			assert.Error(t, err)
			fmt.Errorf(err.Error())
			assert.Equal(t, true, sql.ErrNoRows == err)
		})

	})
}
