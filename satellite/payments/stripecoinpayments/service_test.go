// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestService_InvoiceElementsProcessing(t *testing.T) {
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
		period := time.Date(time.Now().Year(), time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		numberOfProjects := 19
		// generate test data, each user has one project and some credits
		for i := 0; i < numberOfProjects; i++ {
			user, err := satellite.AddUser(ctx, console.CreateUser{
				FullName: "testuser" + strconv.Itoa(i),
				Email:    "user@test" + strconv.Itoa(i),
			}, 1)
			require.NoError(t, err)

			project, err := satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err)

			err = satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
				pb.PieceAction_GET, int64(i+10)*memory.GiB.Int64(), 0, period)
			require.NoError(t, err)
		}

		satellite.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		// check if we have project record for each project
		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, 0, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects, len(recordsPage.Records))

		err = satellite.API.Payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied project records
		recordsPage, err = satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, 0, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, 0, len(recordsPage.Records))
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
		period := time.Date(time.Now().Year(), time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

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
				pb.PieceAction_GET, projectsEgress[i], 0, period)
			require.NoError(t, err)

			// generate storage
			// we need at least two tallies across time to calculate storage
			projectsStorage[i] = int64(i+1) * memory.TiB.Int64()
			tally := &accounting.BucketTally{
				BucketLocation: metabase.BucketLocation{
					ProjectID:  projects[i].ID,
					BucketName: "testbucket",
				},
				TotalBytes:    projectsStorage[i],
				TotalSegments: int64(i + 1),
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

		err = payments.StripeService.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		for i := 0; i < len(projects); i++ {
			projectRecord, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, projects[i].ID, start, end)
			require.NoError(t, err)
			require.NotNil(t, projectRecord)
			require.Equal(t, projects[i].ID, projectRecord.ProjectID)
			require.Equal(t, projectsEgress[i], projectRecord.Egress)

			expectedStorage := float64(projectsStorage[i] * int64(storageHours))
			require.Equal(t, expectedStorage, projectRecord.Storage)

			expectedSegmentsCount := float64((i + 1) * storageHours)
			require.Equal(t, expectedSegmentsCount, projectRecord.Segments)
		}

		// run all parts of invoice generation to see if there are no unexpected errors
		err = payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		err = payments.StripeService.CreateInvoices(ctx, period)
		require.NoError(t, err)

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
		period := time.Date(time.Now().Year(), time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

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

		satellite.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

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
		expectedSegmentPrice := 0.00022

		type TestCase struct {
			Storage  float64
			Egress   int64
			Segments float64

			StorageQuantity  int64
			EgressQuantity   int64
			SegmentsQuantity int64
		}

		testCases := []TestCase{
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
				Segments: 400000, // Segment-Hours
				// object quantity is calculated to Segment-Months
				// round(400000 / 720) Segment-Hours to Segment-Months, 720 - hours in month
				SegmentsQuantity: 556, // Segment-Months
			},
		}

		for _, tc := range testCases {
			record := stripecoinpayments.ProjectRecord{
				Storage:  tc.Storage,
				Egress:   tc.Egress,
				Segments: tc.Segments,
			}

			items := satellite.API.Payments.StripeService.InvoiceItemsFromProjectRecord("project name", record)

			require.Equal(t, tc.StorageQuantity, *items[0].Quantity)
			require.Equal(t, expectedStoragePrice, *items[0].UnitAmountDecimal)

			require.Equal(t, tc.EgressQuantity, *items[1].Quantity)
			require.Equal(t, expectedEgressPrice, *items[1].UnitAmountDecimal)

			require.Equal(t, tc.SegmentsQuantity, *items[2].Quantity)
			require.Equal(t, expectedSegmentPrice, *items[2].UnitAmountDecimal)
		}
	})
}

func TestService_InvoiceItemsFromZeroTokenBalance(t *testing.T) {
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

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)

		// setup storjscan wallet
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = satellite.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)
		_, err = satellite.DB.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      currency.AmountFromBaseUnits(1000, currency.USDollars),
			Description: "token payment credit",
			Source:      "storjscan",
			Status:      billing.TransactionStatusCompleted,
			Type:        billing.TransactionTypeCredit,
			Metadata:    nil,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// run apply token balance to see if there are no unexpected errors
		err = payments.StripeService.InvoiceApplyTokenBalance(ctx, time.Time{})
		require.NoError(t, err)
	})
}

func TestService_GenerateInvoice(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		payments := satellite.API.Payments

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(time.Now().Year(), time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)

		payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		proj, err := satellite.AddProject(ctx, user.ID, "testproject")
		require.NoError(t, err)

		require.NoError(t, payments.StripeService.GenerateInvoices(ctx, start))

		// ensure free tier coupon was applied
		cusID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		stripeUser, err := payments.StripeClient.Customers().Get(cusID, nil)
		require.NoError(t, err)
		require.NotNil(t, stripeUser.Discount)
		require.NotNil(t, stripeUser.Discount.Coupon)
		require.Equal(t, payments.StripeService.StripeFreeTierCouponID, stripeUser.Discount.Coupon.ID)

		// ensure project record was generated
		err = satellite.DB.StripeCoinPayments().ProjectRecords().Check(ctx, proj.ID, start, end)
		require.ErrorIs(t, stripecoinpayments.ErrProjectRecordExists, err)

		rec, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, proj.ID, start, end)
		require.NotNil(t, rec)
		require.NoError(t, err)

		// ensure an invoice was created
		invoiceIter := payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{Customer: &cusID})
		require.True(t, invoiceIter.Next())
		invoice := invoiceIter.Invoice()

		// ensure project record was applied as invoice items to that invoice
		itemIter := payments.StripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{Customer: &cusID})
		count := 0
		for ; itemIter.Next(); count++ {
			item := itemIter.InvoiceItem()
			require.Contains(t, item.Metadata, "projectID")
			require.Equal(t, item.Metadata["projectID"], proj.ID.String())
			require.NotNil(t, itemIter.InvoiceItem().Invoice)
			require.Equal(t, invoice.ID, itemIter.InvoiceItem().Invoice.ID)
		}
		require.NotZero(t, count)
	})
}
