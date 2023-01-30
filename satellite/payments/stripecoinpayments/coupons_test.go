// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
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
			require.True(t, stripecoinpayments.ErrCouponConflict.Has(err))
		})
	})
}
