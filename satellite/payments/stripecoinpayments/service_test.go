// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"context"
	"fmt"
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
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
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

			projectsEgress[i] = int64(i+10) * memory.GiB.Int64()
			projectsStorage[i] = int64(i+1) * memory.TiB.Int64()
			totalSegments := int64(i + 1)
			generateProjectStorage(ctx, t, satellite.DB,
				projects[i].ID,
				period,
				period.Add(time.Duration(storageHours)*time.Hour),
				projectsEgress[i],
				projectsStorage[i],
				totalSegments)
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

			pricing := satellite.API.Payments.Accounts.GetProjectUsagePriceModel(nil)
			items := satellite.API.Payments.StripeService.InvoiceItemsFromProjectRecord("project name", record, pricing)

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
	for _, testCase := range []struct {
		desc              string
		skipEmptyInvoices bool
		addProjectUsage   bool
		expectInvoice     bool
	}{
		{
			desc:              "invoice with non-empty usage created if not configured to skip",
			skipEmptyInvoices: false,
			addProjectUsage:   true,
			expectInvoice:     true,
		},
		{
			desc:              "invoice with non-empty usage created if configured to skip",
			skipEmptyInvoices: true,
			addProjectUsage:   true,
			expectInvoice:     true,
		},
		{
			desc:              "invoice with empty usage created if not configured to skip",
			skipEmptyInvoices: false,
			addProjectUsage:   false,
			expectInvoice:     true,
		},
		{
			desc:              "invoice with empty usage not created if configured to skip",
			skipEmptyInvoices: true,
			addProjectUsage:   false,
			expectInvoice:     false,
		},
	} {
		t.Run(testCase.desc, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
				Reconfigure: testplanet.Reconfigure{
					Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
						config.Payments.StripeCoinPayments.SkipEmptyInvoices = testCase.skipEmptyInvoices
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

				user, err := satellite.AddUser(ctx, console.CreateUser{
					FullName: "Test User",
					Email:    "test@mail.test",
				}, 1)
				require.NoError(t, err)

				proj, err := satellite.AddProject(ctx, user.ID, "testproject")
				require.NoError(t, err)

				// optionally add some usage for the project
				if testCase.addProjectUsage {
					generateProjectStorage(ctx, t, satellite.DB,
						proj.ID,
						period,
						period.Add(24*time.Hour),
						100000,
						200000,
						99)
				}

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

				invoice, hasInvoice := getCustomerInvoice(payments.StripeClient, cusID)
				invoiceItems := getCustomerInvoiceItems(payments.StripeClient, cusID)

				// If invoicing empty usage invoices was skipped, then we don't
				// expect an invoice or invoice items.
				if !testCase.expectInvoice {
					require.False(t, hasInvoice, "expected no invoice but got one")
					require.Empty(t, invoiceItems, "not expecting any invoice items")
					return
				}

				// Otherwise, we expect one or more line items that have been
				// associated with the newly created invoice.
				require.True(t, hasInvoice, "expected invoice but did not get one")
				require.NotZero(t, len(invoiceItems), "expecting one or more invoice items")
				for _, item := range invoiceItems {
					require.Contains(t, item.Metadata, "projectID")
					require.Equal(t, item.Metadata["projectID"], proj.ID.String())
					require.NotNil(t, invoice, item.Invoice)
					require.Equal(t, invoice.ID, item.Invoice.ID)
				}
			})
		})
	}
}

func getCustomerInvoice(stripeClient stripecoinpayments.StripeClient, cusID string) (*stripe.Invoice, bool) {
	iter := stripeClient.Invoices().List(&stripe.InvoiceListParams{Customer: &cusID})
	if iter.Next() {
		return iter.Invoice(), true
	}
	return nil, false
}

func getCustomerInvoiceItems(stripeClient stripecoinpayments.StripeClient, cusID string) (items []*stripe.InvoiceItem) {
	iter := stripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{Customer: &cusID})
	for iter.Next() {
		items = append(items, iter.InvoiceItem())
	}
	return items
}

func generateProjectStorage(ctx context.Context, tb testing.TB, db satellite.DB, projectID uuid.UUID, start, end time.Time, egress, totalBytes, totalSegments int64) {
	// generate egress
	err := db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte("testbucket"),
		pb.PieceAction_GET, egress, 0, start)
	require.NoError(tb, err)

	// generate storage
	tallies := map[metabase.BucketLocation]*accounting.BucketTally{
		{}: {
			BucketLocation: metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
			},
			TotalBytes:    totalBytes,
			TotalSegments: totalSegments,
		},
	}
	err = db.ProjectAccounting().SaveTallies(ctx, start, tallies)
	require.NoError(tb, err)

	err = db.ProjectAccounting().SaveTallies(ctx, end, tallies)
	require.NoError(tb, err)
}

func TestProjectUsagePrice(t *testing.T) {
	var (
		defaultPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB: "1",
			EgressTB:  "2",
			Segment:   "3",
		}
		partnerName  = "partner"
		partnerPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB: "4",
			EgressTB:  "5",
			Segment:   "6",
		}
	)
	defaultModel, err := defaultPrice.ToModel()
	require.NoError(t, err)
	partnerModel, err := partnerPrice.ToModel()
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.UsagePrice = defaultPrice
				config.Payments.UsagePriceOverrides.SetMap(map[string]paymentsconfig.ProjectUsagePrice{
					partnerName: partnerPrice,
				})
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		// pick a specific date so that it doesn't fail if it's the last day of the month
		// keep month + 1 because user needs to be created before calculation
		period := time.Date(time.Now().Year(), time.Now().Month()+1, 20, 0, 0, 0, 0, time.UTC)
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})

		for i, tt := range []struct {
			name          string
			userAgent     []byte
			expectedPrice payments.ProjectUsagePriceModel
		}{
			{"default pricing", nil, defaultModel},
			{"default pricing - user agent is not valid partner ID", []byte("invalid/v0.0"), defaultModel},
			{"partner pricing - user agent is partner ID", []byte(partnerName), partnerModel},
			{"partner pricing - user agent includes partner ID", []byte("invalid/v0.0 " + partnerName + " invalid/v0.0"), partnerModel},
		} {
			t.Run(tt.name, func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName:  "Test User",
					Email:     fmt.Sprintf("user%d@mail.test", i),
					UserAgent: tt.userAgent,
				}, 1)
				require.NoError(t, err)

				project, err := sat.AddProject(ctx, user.ID, "testproject")
				require.NoError(t, err)

				err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte("testbucket"),
					pb.PieceAction_GET, memory.TB.Int64(), 0, period)
				require.NoError(t, err)

				err = sat.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period)
				require.NoError(t, err)

				err = sat.API.Payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
				require.NoError(t, err)

				cusID, err := sat.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
				require.NoError(t, err)

				items := getCustomerInvoiceItems(sat.API.Payments.StripeClient, cusID)
				require.Len(t, items, 3)
				storage, _ := tt.expectedPrice.StorageMBMonthCents.Float64()
				require.Equal(t, storage, items[0].UnitAmountDecimal)
				egress, _ := tt.expectedPrice.EgressMBCents.Float64()
				require.Equal(t, egress, items[1].UnitAmountDecimal)
				segment, _ := tt.expectedPrice.SegmentMonthCents.Float64()
				require.Equal(t, segment, items[2].UnitAmountDecimal)
			})
		}
	})
}

func TestPayInvoicesSkipDue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		cus1 := "cus_1"
		cus2 := "cus_2"
		amount := int64(100)
		curr := string(stripe.CurrencyUSD)
		due := time.Now().Add(14 * 24 * time.Hour).Unix()

		_, err := satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Amount:   &amount,
			Currency: &curr,
			Customer: &cus1,
		})
		require.NoError(t, err)
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Amount:   &amount,
			Currency: &curr,
			Customer: &cus2,
		})
		require.NoError(t, err)

		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Customer: &cus1,
		})
		require.NoError(t, err)
		invWithDue, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Customer: &cus2,
			DueDate:  &due,
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.PayInvoices(ctx, time.Time{})
		require.NoError(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{})
		for iter.Next() {
			i := iter.Invoice()
			if i.ID == inv.ID {
				require.Equal(t, stripe.InvoiceStatusPaid, i.Status)
			}
			// when due date is set invoice should not be paid
			if i.ID == invWithDue.ID {
				require.Equal(t, stripe.InvoiceStatus(""), i.Status)
			}
		}
	})
}
