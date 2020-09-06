// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/payments"
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

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		numberOfProjects := 19
		// generate test data, each user has one project, one coupon and some credits
		for i := 0; i < numberOfProjects; i++ {
			user, err := satellite.AddUser(ctx, console.CreateUser{
				FullName: "testuser" + strconv.Itoa(i),
				Email:    "user@test" + strconv.Itoa(i),
			}, 1)
			require.NoError(t, err)

			project, err := satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err)

			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, int64(i+10)*memory.GiB.Int64(), period)
			require.NoError(t, err)
		}

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

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		payments.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)

		numberOfProjects := 5
		storageHours := 24

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, numberOfProjects)
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
				BucketLocation: metabase.BucketLocation{
					ProjectID:  projects[i].ID,
					BucketName: "testbucket",
				},
				RemoteBytes: projectsStorage[i],
				ObjectCount: int64(i + 1),
			}
			tallies := map[metabase.BucketLocation]*accounting.BucketTally{
				{}: tally,
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

		couponsPage, err := satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 1, len(couponsPage.Usages))

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

		err = payments.Service.CreateInvoices(ctx, period)
		require.NoError(t, err)
	})
}

func TestService_InvoiceUserWithManyCoupons(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.CouponValue = 3
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		paymentsAPI := satellite.API.Payments

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		paymentsAPI.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)

		storageHours := 24

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 5)
		require.NoError(t, err)

		project, err := satellite.AddProject(ctx, user.ID, "testproject")
		require.NoError(t, err)

		sumOfCoupons := int64(0)
		for i := 0; i < 5; i++ {
			coupon, err := satellite.API.Payments.Accounts.Coupons().Create(ctx, payments.Coupon{
				ID:       testrand.UUID(),
				UserID:   user.ID,
				Amount:   int64(i + 4),
				Duration: 2,
				Status:   payments.CouponActive,
				Type:     payments.CouponTypePromotional,
			})
			require.NoError(t, err)
			sumOfCoupons += coupon.Amount
		}

		{
			// generate egress
			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, 10*memory.GiB.Int64(), period)
			require.NoError(t, err)

			// generate storage
			// we need at least two tallies across time to calculate storage
			tally := &accounting.BucketTally{
				BucketLocation: metabase.BucketLocation{
					ProjectID:  project.ID,
					BucketName: "testbucket",
				},
				RemoteBytes: memory.TiB.Int64(),
				ObjectCount: 45,
			}
			tallies := map[metabase.BucketLocation]*accounting.BucketTally{
				{}: tally,
			}
			err = satellite.DB.ProjectAccounting().SaveTallies(ctx, period, tallies)
			require.NoError(t, err)

			err = satellite.DB.ProjectAccounting().SaveTallies(ctx, period.Add(time.Duration(storageHours)*time.Hour), tallies)
			require.NoError(t, err)
		}

		err = paymentsAPI.Service.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		// we should have usages for coupons: created with user + created in test
		couponsPage, err := satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 1+5, len(couponsPage.Usages))

		coupons, err := satellite.DB.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(coupons), len(couponsPage.Usages))

		var sumCoupons int64
		var sumUsages int64
		for i, coupon := range coupons {
			sumCoupons += coupon.Amount
			require.NotEqual(t, payments.CouponExpired, coupon.Status)

			sumUsages += couponsPage.Usages[i].Amount
			require.Equal(t, stripecoinpayments.CouponUsageStatusUnapplied, couponsPage.Usages[i].Status)
		}

		require.Equal(t, sumCoupons, sumUsages)

		err = paymentsAPI.Service.InvoiceApplyCoupons(ctx, period)
		require.NoError(t, err)

		coupons, err = satellite.DB.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(coupons), len(couponsPage.Usages))

		for _, coupon := range coupons {
			require.Equal(t, payments.CouponUsed, coupon.Status)
		}

		couponsPage, err = satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 0, len(couponsPage.Usages))
	})
}

func TestService_ApplyCouponsInTheOrder(t *testing.T) {
	// apply coupons in the order of their expiration date
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.CouponValue = 24
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		paymentsAPI := satellite.API.Payments

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		paymentsAPI.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 5)
		require.NoError(t, err)

		project, err := satellite.AddProject(ctx, user.ID, "testproject")
		require.NoError(t, err)

		additionalCoupons := 3
		// we will have coupons with duration 5, 4, 3 and 2 from coupon create with AddUser
		for i := 0; i < additionalCoupons; i++ {
			_, err = satellite.API.Payments.Accounts.Coupons().Create(ctx, payments.Coupon{
				ID:       testrand.UUID(),
				UserID:   user.ID,
				Amount:   24,
				Duration: additionalCoupons - i + 2,
				Status:   payments.CouponActive,
				Type:     payments.CouponTypePromotional,
			})
			require.NoError(t, err)
		}

		{
			// generate egress - 48 cents
			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, 10*memory.GiB.Int64(), period)
			require.NoError(t, err)
		}

		err = paymentsAPI.Service.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		// we should have usages for 2 coupons for which left to charge will be 0
		couponsPage, err := satellite.DB.StripeCoinPayments().Coupons().ListUnapplied(ctx, 0, 40, start)
		require.NoError(t, err)
		require.Equal(t, 2, len(couponsPage.Usages))

		err = paymentsAPI.Service.InvoiceApplyCoupons(ctx, period)
		require.NoError(t, err)

		usedCoupons, err := satellite.DB.StripeCoinPayments().Coupons().ListByUserIDAndStatus(ctx, user.ID, payments.CouponUsed)
		require.NoError(t, err)
		require.Equal(t, 2, len(usedCoupons))
		// coupons with duration 2 and 3 should be used
		for _, coupon := range usedCoupons {
			require.Less(t, coupon.Duration, 4)
		}

		activeCoupons, err := satellite.DB.StripeCoinPayments().Coupons().ListByUserIDAndStatus(ctx, user.ID, payments.CouponActive)
		require.NoError(t, err)
		require.Equal(t, 2, len(activeCoupons))
		// coupons with duration 4 and 5 should be NOT used
		for _, coupon := range activeCoupons {
			require.Greater(t, coupon.Duration, 3)
			require.EqualValues(t, 24, coupon.Amount)
		}
	})
}

func TestService_CouponStatus(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		for i, tt := range []struct {
			duration       int
			amount         int64
			egress         memory.Size
			expectedStatus payments.CouponStatus
		}{
			{
				duration:       2,   // expires one month after billed period
				amount:         100, // $1.00
				egress:         0,   // $0.00
				expectedStatus: payments.CouponActive,
			},
			{
				duration:       2,              // expires one month after billed period
				amount:         100,            // $1.00
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponActive,
			},
			{
				duration:       2,              // expires one month after billed period
				amount:         10,             // $0.10
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponUsed,
			},
			{
				duration:       1,   // the billed period is the last valid month
				amount:         100, // $1.00
				egress:         0,   // $0.00
				expectedStatus: payments.CouponExpired,
			},
			{
				duration:       1,              // the billed period is the last valid month
				amount:         100,            // $1.00
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponExpired,
			},
			{
				duration:       1,              // the billed period is the last valid month
				amount:         10,             // $0.10
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponUsed,
			},
			{
				duration:       0,   // expired before the billed period
				amount:         100, // $1.00
				egress:         0,   // $0.00
				expectedStatus: payments.CouponExpired,
			},
			{
				duration:       0,              // expired before the billed period
				amount:         100,            // $1.00
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponExpired,
			},
			{
				duration:       0,              // expired before the billed period
				amount:         10,             // $0.10
				egress:         10 * memory.GB, // $0.45
				expectedStatus: payments.CouponExpired,
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			user, err := satellite.AddUser(ctx, console.CreateUser{
				FullName: "testuser" + strconv.Itoa(i),
				Email:    "test@test" + strconv.Itoa(i),
			}, 1)
			require.NoError(t, err, errTag)

			project, err := satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err, errTag)

			// Delete any automatically added coupons
			coupons, err := satellite.API.Payments.Accounts.Coupons().ListByUserID(ctx, user.ID)
			require.NoError(t, err, errTag)
			for _, coupon := range coupons {
				err = satellite.DB.StripeCoinPayments().Coupons().Delete(ctx, coupon.ID)
				require.NoError(t, err, errTag)
			}

			// create a new coupon
			_, err = satellite.API.Payments.Accounts.Coupons().Create(ctx, payments.Coupon{
				ID:       testrand.UUID(),
				UserID:   user.ID,
				Amount:   tt.amount,
				Duration: tt.duration,
			})
			require.NoError(t, err, errTag)

			// pick a specific date so that it doesn't fail if it's the last day of the month
			// keep month + 1 because user needs to be created before calculation
			period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

			// generate egress
			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, tt.egress.Int64(), period)
			require.NoError(t, err, errTag)

			satellite.API.Payments.Service.SetNow(func() time.Time {
				return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			})

			err = satellite.API.Payments.Service.PrepareInvoiceProjectRecords(ctx, period)
			require.NoError(t, err, errTag)

			err = satellite.API.Payments.Service.InvoiceApplyCoupons(ctx, period)
			require.NoError(t, err, errTag)

			coupons, err = satellite.DB.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
			require.NoError(t, err, errTag)
			require.Len(t, coupons, 1, errTag)
			assert.Equal(t, tt.expectedStatus, coupons[0].Status, errTag)
		}
	})
}

func TestService_ProjectsWithMembers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.ListingLimit = 4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(2020, time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		numberOfUsers := 5
		users := make([]*console.User, numberOfUsers)
		projects := make([]*console.Project, numberOfUsers)
		for i := 0; i < numberOfUsers; i++ {
			var err error

			users[i], err = satellite.AddUser(ctx, console.CreateUser{
				FullName: "testuser" + strconv.Itoa(i),
				Email:    "user@test" + strconv.Itoa(i),
			}, 1)
			require.NoError(t, err)

			projects[i], err = satellite.AddProject(ctx, users[i].ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err)
		}

		// all users are members in all projects
		for _, project := range projects {
			for _, user := range users {
				if project.OwnerID != user.ID {
					_, err := satellite.DB.Console().ProjectMembers().Insert(ctx, user.ID, project.ID)
					require.NoError(t, err)
				}
			}
		}

		satellite.API.Payments.Service.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.Service.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)

		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, 0, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, len(projects), len(recordsPage.Records))
	})
}

func TestService_InvoiceItemsFromProjectRecord(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// these numbers are fraction of cents, not of dollars.
		expectedStoragePrice := 0.001
		expectedEgressPrice := 0.0045
		expectedObjectPrice := 0.00022

		type TestCase struct {
			Storage float64
			Egress  int64
			Objects float64

			StorageQuantity int64
			EgressQuantity  int64
			ObjectsQuantity int64
		}

		var testCases = []TestCase{
			{}, // all zeros
			{
				Storage: 10000000000, // Byte-Hours
				// storage quantity is calculated to Megabyte-Months
				// (10000000000 / 1000000) Byte-Hours to Megabytes-Hours
				// round(10000 / 720) Megabytes-Hours to Megabyte-Months, 720 - hours in month
				StorageQuantity: 14, // Megabyte-Months
			},
			{
				Egress: 134 * memory.GB.Int64(), // Bytes
				// egress quantity is calculated to Megabytes
				// (134000000000 / 1000000) Bytes to Megabytes
				EgressQuantity: 134000, // Megabytes
			},
			{
				Objects: 400000, // Object-Hours
				// object quantity is calculated to Object-Months
				// round(400000 / 720) Object-Hours to Object-Months, 720 - hours in month
				ObjectsQuantity: 556, // Object-Months
			},
		}

		for _, tc := range testCases {
			record := stripecoinpayments.ProjectRecord{
				Storage: tc.Storage,
				Egress:  tc.Egress,
				Objects: tc.Objects,
			}

			items := satellite.API.Payments.Service.InvoiceItemsFromProjectRecord("project name", record)

			require.Equal(t, tc.StorageQuantity, *items[0].Quantity)
			require.Equal(t, expectedStoragePrice, *items[0].UnitAmountDecimal)

			require.Equal(t, tc.EgressQuantity, *items[1].Quantity)
			require.Equal(t, expectedEgressPrice, *items[1].UnitAmountDecimal)

			require.Equal(t, tc.ObjectsQuantity, *items[2].Quantity)
			require.Equal(t, expectedObjectPrice, *items[2].UnitAmountDecimal)
		}
	})
}
