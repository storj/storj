// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCustomersUpdateGetPackage(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		userID := testrand.UUID()
		customerID := "testCustomerID"
		var packagePlan string
		var purchaseTime time.Time
		packagePlan = "package-plan-1"
		purchaseTime = time.Now()

		require.NoError(t, db.StripeCoinPayments().Customers().Insert(ctx, userID, customerID))

		dbPackagePlan, dbPurchaseTime, err := db.StripeCoinPayments().Customers().GetPackageInfo(ctx, userID)
		require.NoError(t, err)
		require.Nil(t, dbPackagePlan)
		require.Nil(t, dbPurchaseTime)

		c, err := db.StripeCoinPayments().Customers().UpdatePackage(ctx, userID, &packagePlan, &purchaseTime)
		require.NoError(t, err)
		require.Equal(t, userID, c.UserID)
		require.Equal(t, customerID, c.ID)
		require.NotNil(t, c.PackagePlan)
		require.Equal(t, packagePlan, *c.PackagePlan)
		require.NotNil(t, c.PackagePurchasedAt)
		require.WithinDuration(t, purchaseTime.Truncate(time.Millisecond), c.PackagePurchasedAt.Truncate(time.Millisecond), time.Nanosecond)

		dbPackagePlan, dbPurchaseTime, err = db.StripeCoinPayments().Customers().GetPackageInfo(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, dbPackagePlan)
		require.NotNil(t, dbPurchaseTime)
		require.Equal(t, packagePlan, *dbPackagePlan)
		require.WithinDuration(t, purchaseTime.Truncate(time.Millisecond), dbPurchaseTime.Truncate(time.Millisecond), time.Nanosecond)

		c, err = db.StripeCoinPayments().Customers().UpdatePackage(ctx, userID, nil, nil)
		require.NoError(t, err)
		require.Equal(t, userID, c.UserID)
		require.Equal(t, customerID, c.ID)
		require.Nil(t, c.PackagePlan)
		require.Nil(t, c.PackagePurchasedAt)

		dbPackagePlan, dbPurchaseTime, err = db.StripeCoinPayments().Customers().GetPackageInfo(ctx, userID)
		require.NoError(t, err)
		require.Nil(t, dbPackagePlan)
		require.Nil(t, dbPurchaseTime)
	})
}
