// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestService_InvoiceElementsProcessing(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.ListingLimit = 4
				config.Payments.CouponValue = 5
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// delete preconfigured project to not mess with test flow
		err := satellite.DB.Console().Projects().Delete(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		numberOfProjects := 19
		// generate test data, each user has one project, one coupon and some credits
		for i := 0; i < numberOfProjects; i++ {
			user, err := satellite.AddUser(ctx, "testuser"+strconv.Itoa(i), "user@test"+strconv.Itoa(i), 1)
			require.NoError(t, err)

			project, err := satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err)

			credit := payments.Credit{
				UserID:        user.ID,
				Amount:        9,
				TransactionID: coinpayments.TransactionID("transID" + strconv.Itoa(i)),
			}
			err = satellite.DB.StripeCoinPayments().Credits().InsertCredit(ctx, credit)
			require.NoError(t, err)

			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, int64(i+10)*memory.GiB.Int64(), time.Now())
			require.NoError(t, err)
		}

		period := time.Now()
		satellite.API.Payments.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err = satellite.API.Payments.Service.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)

		// check if we have project record for each project
		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, 0, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects, len(recordsPage.Records))

		// check if we have coupon for each project
		couponsPage, err := satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects, len(couponsPage.Usages))

		// check if we have credits spendings for each project
		spendingsPage, err := satellite.DB.StripeCoinPayments().Credits().ListCreditsSpendingsPaged(ctx, int(stripecoinpayments.CreditsSpendingStatusUnapplied), 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects, len(spendingsPage.Spendings))

		err = satellite.API.Payments.Service.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied project records
		recordsPage, err = satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, 0, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, 0, len(recordsPage.Records))

		err = satellite.API.Payments.Service.InvoiceApplyCoupons(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied coupons
		couponsPage, err = satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 0, len(couponsPage.Usages))

		err = satellite.API.Payments.Service.InvoiceApplyCredits(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied credits spendings
		spendingsPage, err = satellite.DB.StripeCoinPayments().Credits().ListCreditsSpendingsPaged(ctx, int(stripecoinpayments.CreditsSpendingStatusUnapplied), 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 0, len(spendingsPage.Spendings))
	})
}
