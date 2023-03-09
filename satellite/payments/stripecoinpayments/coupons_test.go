// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestCouponConflict(t *testing.T) {
	const (
		partnerName  = "partner"
		partnerCode  = "promo1"
		standardCode = "promo2"
	)
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.PackagePlans.Packages = map[string]payments.PackagePlan{
					partnerName: {CouponID: "c1"},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		coupons := sat.Core.Payments.Accounts.Coupons()

		t.Run("standard user can replace partner coupon", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "user@mail.test",
			}, 2)
			require.NoError(t, err)

			_, err = coupons.ApplyCouponCode(ctx, user.ID, partnerCode)
			require.NoError(t, err)
			_, err = coupons.ApplyCouponCode(ctx, user.ID, standardCode)
			require.NoError(t, err)
		})

		partneredUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User",
			Email:     "user2@mail.test",
			UserAgent: []byte(partnerName),
		}, 2)
		require.NoError(t, err)

		t.Run("partnered user can replace standard coupon", func(t *testing.T) {
			_, err = coupons.ApplyCouponCode(ctx, partneredUser.ID, standardCode)
			require.NoError(t, err)
			_, err = coupons.ApplyCouponCode(ctx, partneredUser.ID, partnerCode)
			require.NoError(t, err)
		})

		t.Run("partnered user cannot replace partner coupon", func(t *testing.T) {
			_, err = coupons.ApplyCouponCode(ctx, partneredUser.ID, standardCode)
			require.True(t, payments.ErrCouponConflict.Has(err))
		})
	})
}

func TestCoupons(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.StripeFreeTierCouponID = stripecoinpayments.MockCouponID1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		c := satellite.API.Payments.Accounts.Coupons()
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		t.Run("ApplyCoupon fails with no matching coupon", func(t *testing.T) {
			coupon, err := c.ApplyCoupon(ctx, userID, "unknown_coupon_id")
			require.Error(t, err)
			require.Nil(t, coupon)
		})
		t.Run("ApplyCoupon fails with no matching customer", func(t *testing.T) {
			coupon, err := c.ApplyCoupon(ctx, testrand.UUID(), stripecoinpayments.MockCouponID2)
			require.Error(t, err)
			require.Nil(t, coupon)
		})
		t.Run("ApplyCoupon, GetByUserID succeeds", func(t *testing.T) {
			id := stripecoinpayments.MockCouponID1
			coupon, err := c.ApplyCoupon(ctx, userID, id)
			require.NoError(t, err)
			require.NotNil(t, coupon)
			require.Equal(t, id, coupon.ID)

			coupon, err = c.GetByUserID(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, id, coupon.ID)
		})
		t.Run("ApplyFreeTierCoupon succeeds", func(t *testing.T) {
			id := satellite.API.Payments.StripeService.StripeFreeTierCouponID
			coupon, err := c.ApplyFreeTierCoupon(ctx, userID)
			require.NoError(t, err)
			require.NotNil(t, coupon)
			require.Equal(t, id, coupon.ID)

			coupon, err = c.GetByUserID(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, id, coupon.ID)
		})
		t.Run("ApplyFreeTierCoupon fails with unknown user", func(t *testing.T) {
			coupon, err := c.ApplyFreeTierCoupon(ctx, testrand.UUID())
			require.Error(t, err)
			require.Nil(t, coupon)
		})
	})
}
