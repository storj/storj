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
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestService_InvoiceElementsProcessing(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.ListingLimit = 4
				config.Payments.CouponValue = 5
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

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
		err := satellite.API.Payments.Service.PrepareInvoiceProjectRecords(ctx, period)
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

func TestService_InvoiceUserWithManyProjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.ListingLimit = 4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		payments := satellite.API.Payments

		period := time.Now()
		payments.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)

		numberOfProjects := 5
		storageHours := 24

		user, err := satellite.AddUser(ctx, "testuser", "user@test", numberOfProjects)
		require.NoError(t, err)

		projects := make([]*console.Project, numberOfProjects)
		projectsEgress := make([]int64, len(projects))
		projectsStorage := make([]int64, len(projects))
		for i := 0; i < len(projects); i++ {
			projects[i], err = satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err)

			// generate egress
			projectsEgress[i] = int64(i+10) * memory.GiB.Int64()
			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, projects[i].ID, []byte("testbucket"),
				pb.PieceAction_GET, projectsEgress[i], period)
			require.NoError(t, err)

			// generate storage
			// we need at least two tallies across time to calculate storage
			projectsStorage[i] = int64(i+1) * memory.TiB.Int64()
			tally := &accounting.BucketTally{
				BucketName:  []byte("testbucket"),
				ProjectID:   projects[i].ID,
				RemoteBytes: projectsStorage[i],
				ObjectCount: int64(i + 1),
			}
			tallies := map[string]*accounting.BucketTally{
				"0": tally,
			}
			err = satellite.DB.ProjectAccounting().SaveTallies(ctx, period, tallies)
			require.NoError(t, err)

			err = satellite.DB.ProjectAccounting().SaveTallies(ctx, period.Add(time.Duration(storageHours)*time.Hour), tallies)
			require.NoError(t, err)

			// verify that projects don't have records yet
			projectRecord, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, projects[i].ID, start, end)
			require.NoError(t, err)
			require.Nil(t, projectRecord)
		}

		err = payments.Service.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		for i := 0; i < len(projects); i++ {
			projectRecord, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, projects[i].ID, start, end)
			require.NoError(t, err)
			require.NotNil(t, projectRecord)
			require.Equal(t, projects[i].ID, projectRecord.ProjectID)
			require.Equal(t, projectsEgress[i], projectRecord.Egress)

			expectedStorage := float64(projectsStorage[i] * int64(storageHours))
			require.Equal(t, expectedStorage, projectRecord.Storage)

			expectedObjectsCount := float64((i + 1) * storageHours)
			require.Equal(t, expectedObjectsCount, projectRecord.Objects)
		}

		// run all parts of invoice generation to see if there are no unexpected errors
		err = payments.Service.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		err = payments.Service.InvoiceApplyCoupons(ctx, period)
		require.NoError(t, err)

		err = payments.Service.InvoiceApplyCredits(ctx, period)
		require.NoError(t, err)

		err = payments.Service.CreateInvoices(ctx, period)
		require.NoError(t, err)
	})
}
