// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v75"
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
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

func TestService_SetInvoiceStatusUncollectible(t *testing.T) {
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

		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice
		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// run update invoice status to uncollectible
		// beginning of last month
		startPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		// end of current month
		endPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)

		t.Run("update invoice status to uncollectible", func(t *testing.T) {
			err = payments.StripeService.SetInvoiceStatus(ctx, startPeriod, endPeriod, "uncollectible", false)
			require.NoError(t, err)

			iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
				ListParams: stripe.ListParams{Context: ctx},
			})
			iter.Next()
			require.Equal(t, stripe.InvoiceStatusUncollectible, iter.Invoice().Status)
		})
	})
}

func TestService_SetInvoiceStatusVoid(t *testing.T) {
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

		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice
		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// run update invoice status to uncollectible
		// beginning of last month
		startPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		// end of current month
		endPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)

		t.Run("update invoice status to void", func(t *testing.T) {
			err = payments.StripeService.SetInvoiceStatus(ctx, startPeriod, endPeriod, "void", false)
			require.NoError(t, err)

			iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
				ListParams: stripe.ListParams{Context: ctx},
			})
			iter.Next()
			require.Equal(t, stripe.InvoiceStatusVoid, iter.Invoice().Status)
		})
	})
}

func TestService_SetInvoiceStatusPaid(t *testing.T) {
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

		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice
		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// run update invoice status to uncollectible
		// beginning of last month
		startPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		// end of current month
		endPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)

		t.Run("update invoice status to paid", func(t *testing.T) {
			err = payments.StripeService.SetInvoiceStatus(ctx, startPeriod, endPeriod, "paid", false)
			require.NoError(t, err)

			iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
				ListParams: stripe.ListParams{Context: ctx},
			})
			iter.Next()
			require.Equal(t, stripe.InvoiceStatusPaid, iter.Invoice().Status)
		})
	})
}

func TestService_SetInvoiceStatusInvalid(t *testing.T) {
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

		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice
		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// run update invoice status to uncollectible
		// beginning of last month
		startPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		// end of current month
		endPeriod := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)

		t.Run("update invoice status to invalid", func(t *testing.T) {
			err = payments.StripeService.SetInvoiceStatus(ctx, startPeriod, endPeriod, "not a real status", false)
			require.Error(t, err)
		})
	})
}

func TestService_BalanceInvoiceItems(t *testing.T) {
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

		numberOfUsers := 10
		users := make([]*console.User, numberOfUsers)
		projects := make([]*console.Project, numberOfUsers)
		// create a bunch of users
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

		// give one of the users a stripe balance
		cusID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, users[4].ID)
		require.NoError(t, err)
		_, err = payments.StripeClient.Customers().Update(cusID, &stripe.CustomerParams{
			Params: stripe.Params{
				Context: ctx,
			},
			Balance: stripe.Int64(1000),
		})
		require.NoError(t, err)

		// convert the stripe balance into an invoice item
		require.NoError(t, payments.StripeService.CreateBalanceInvoiceItems(ctx))

		// check that the invoice item was created
		itr := payments.StripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
			Customer: stripe.String(cusID),
		})
		require.True(t, itr.Next())
		require.NoError(t, itr.Err())
		require.Equal(t, int64(1000), itr.InvoiceItem().UnitAmount)

		// check that the stripe balance was reset
		cus, err := payments.StripeClient.Customers().Get(cusID, &stripe.CustomerParams{
			Params: stripe.Params{
				Context: ctx,
			},
		})
		require.NoError(t, err)
		require.Equal(t, int64(0), cus.Balance)

		// Deactivate the users and give them balances
		statusPending := console.PendingDeletion
		statusDeleted := console.Deleted
		statusLegalHold := console.LegalHold
		for i, user := range users {
			cusID, err = satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
			require.NoError(t, err)
			_, err = payments.StripeClient.Customers().Update(cusID, &stripe.CustomerParams{
				Params: stripe.Params{
					Context: ctx,
				},
				Balance: stripe.Int64(1000),
			})
			require.NoError(t, err)

			var status *console.UserStatus
			if i%2 == 0 {
				status = &statusDeleted
			} else if i%3 == 0 {
				status = &statusPending
			} else {
				status = &statusLegalHold
			}
			err := satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Status: status,
			})
			require.NoError(t, err)
		}

		// try to convert the stripe balance into an invoice item
		require.NoError(t, payments.StripeService.CreateBalanceInvoiceItems(ctx))

		// check no invoice item was created since all users are deactivated
		itr = payments.StripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
			Customer: stripe.String(cusID),
		})
		require.NoError(t, itr.Err())
		require.False(t, itr.Next())
	})
}

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
		numberOfInactiveUsers := 5
		status := console.PendingDeletion
		// user to be deactivated later
		var activeUser console.User
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

			if i < numberOfProjects-numberOfInactiveUsers {
				activeUser = *user
				continue
			}
			if i%2 == 0 {
				status = console.Deleted
			} else if i%3 == 0 {
				status = console.PendingDeletion
			} else {
				status = console.LegalHold
			}
			err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)
		}

		satellite.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		// check if we have project record for each project, except for inactive users
		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects-numberOfInactiveUsers, len(recordsPage.Records))

		// deactivate user
		err = satellite.DB.Console().Users().Update(ctx, activeUser.ID, console.UpdateUserRequest{
			Status: &status,
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied project records
		recordsPage, err = satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, 40, start, end)
		require.NoError(t, err)

		// the 1 remaining record is for the now inactive user
		require.Equal(t, 1, len(recordsPage.Records))
	})
}

func TestService_InvoiceElementsProcessingGrouped(t *testing.T) {
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
		numberOfInactiveUsers := 5
		status := console.PendingDeletion
		// user to be deactivated later
		var activeUser console.User
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

			if i < numberOfProjects-numberOfInactiveUsers {
				activeUser = *user
				continue
			}
			if i%2 == 0 {
				status = console.Deleted
			} else if i%3 == 0 {
				status = console.PendingDeletion
			} else {
				status = console.LegalHold
			}
			err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)
		}

		satellite.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		// check if we have project record for each project, except for inactive users
		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, numberOfProjects-numberOfInactiveUsers, len(recordsPage.Records))

		// deactivate user
		err = satellite.DB.Console().Users().Update(ctx, activeUser.ID, console.UpdateUserRequest{
			Status: &status,
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.InvoiceApplyProjectRecordsGrouped(ctx, period)
		require.NoError(t, err)

		// verify that we applied all unapplied project records
		recordsPage, err = satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, 40, start, end)
		require.NoError(t, err)

		// the 1 remaining record is for the now inactive user
		require.Equal(t, 1, len(recordsPage.Records))
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

		err = payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
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

		// deactivate user
		pendingDeletionStatus := console.PendingDeletion
		activeStatus := console.Active
		err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &pendingDeletionStatus,
		})
		require.NoError(t, err)

		err = payments.StripeService.CreateInvoices(ctx, period, false)
		require.NoError(t, err)

		// invoice wasn't created because user is deactivated
		itr := payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{})
		require.False(t, itr.Next())
		require.NoError(t, itr.Err())

		err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &activeStatus,
		})
		require.NoError(t, err)

		err = payments.StripeService.CreateInvoices(ctx, period, false)
		require.NoError(t, err)

		// invoice was created because user is active
		itr = payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{})
		require.True(t, itr.Next())
		require.NoError(t, itr.Err())
	})
}

func TestService_FinalizeInvoices(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		stripeClient := satellite.API.Payments.StripeClient

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(1000),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		itr := stripeClient.Invoices().List(&stripe.InvoiceListParams{
			Customer: &customer,
		})
		require.True(t, itr.Next())
		require.NoError(t, itr.Err())
		require.Equal(t, stripe.InvoiceStatusDraft, itr.Invoice().Status)

		// deactivate user
		pendingDeletionStatus := console.PendingDeletion
		err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &pendingDeletionStatus,
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.FinalizeInvoices(ctx)
		require.NoError(t, err)

		itr = stripeClient.Invoices().List(&stripe.InvoiceListParams{
			Customer: &customer,
		})
		require.True(t, itr.Next())
		require.NoError(t, itr.Err())
		// finalizing did not work because user is deactivated
		require.Equal(t, stripe.InvoiceStatusDraft, itr.Invoice().Status)

		activeStatus := console.Active
		err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &activeStatus,
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.FinalizeInvoices(ctx)
		require.NoError(t, err)

		itr = stripeClient.Invoices().List(&stripe.InvoiceListParams{
			Customer: &customer,
		})
		require.True(t, itr.Next())
		require.NoError(t, itr.Err())
		require.Equal(t, stripe.InvoiceStatusOpen, itr.Invoice().Status)
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
					_, err := satellite.DB.Console().ProjectMembers().Insert(ctx, user.ID, project.ID, console.RoleAdmin)
					require.NoError(t, err)
				}
			}
		}

		satellite.API.Payments.StripeService.SetNow(func() time.Time {
			return time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		})
		err := satellite.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
		require.NoError(t, err)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		recordsPage, err := satellite.DB.StripeCoinPayments().ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, 40, start, end)
		require.NoError(t, err)
		require.Equal(t, len(projects), len(recordsPage.Records))
	})
}

func TestService_InvoiceItemsFromProjectUsage(t *testing.T) {
	const (
		projectName           = "my-project"
		partnerName           = "partner"
		noOverridePartnerName = "no-override"

		hoursPerMonth       = 24 * 30
		bytesPerMegabyte    = int64(memory.MB / memory.B)
		byteHoursPerMBMonth = hoursPerMonth * bytesPerMegabyte
	)

	var (
		defaultPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB: "1",
			EgressTB:  "2",
			Segment:   "3",
		}
		partnerPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB:           "4",
			EgressTB:            "5",
			Segment:             "6",
			EgressDiscountRatio: 0.5,
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
		usage := map[string]accounting.ProjectUsage{
			"": {
				Storage:      10000000000,             // Byte-hours
				Egress:       123 * memory.GB.Int64(), // Bytes
				SegmentCount: 200000,                  // Segment-Hours
			},
			partnerName: {
				Storage:      20000000000,
				Egress:       456 * memory.GB.Int64(),
				SegmentCount: 400000,
			},
			noOverridePartnerName: {
				Storage:      30000000000,
				Egress:       789 * memory.GB.Int64(),
				SegmentCount: 600000,
			},
		}

		items := planet.Satellites[0].API.Payments.StripeService.InvoiceItemsFromProjectUsage(projectName, usage, false)
		require.Len(t, items, len(usage)*3)

		for i, tt := range []struct {
			name       string
			partner    string
			priceModel payments.ProjectUsagePriceModel
		}{
			{"default pricing - no partner", "", defaultModel},
			{"default pricing - no override for partner", noOverridePartnerName, defaultModel},
			{"partner pricing", partnerName, partnerModel},
		} {
			t.Run(tt.name, func(t *testing.T) {
				prefix := "Project " + projectName
				if tt.partner != "" {
					prefix += " (" + tt.partner + ")"
				}

				usage := usage[tt.partner]
				usage.Egress -= int64(math.Round(usage.Storage / hoursPerMonth * tt.priceModel.EgressDiscountRatio))
				if usage.Egress < 0 {
					usage.Egress = 0
				}

				expectedStorageQuantity := int64(math.Round(usage.Storage / float64(byteHoursPerMBMonth)))
				expectedEgressQuantity := int64(math.Round(float64(usage.Egress) / float64(bytesPerMegabyte)))
				expectedSegmentQuantity := int64(math.Round(usage.SegmentCount / hoursPerMonth))

				items := items[i*3 : (i*3)+3]
				for _, item := range items {
					require.NotNil(t, item)
				}

				require.Equal(t, prefix+" - Segment Storage (MB-Month)", *items[0].Description)
				require.Equal(t, expectedStorageQuantity, *items[0].Quantity)
				storage, _ := tt.priceModel.StorageMBMonthCents.Float64()
				require.Equal(t, storage, *items[0].UnitAmountDecimal)

				require.Equal(t, prefix+" - Egress Bandwidth (MB)", *items[1].Description)
				require.Equal(t, expectedEgressQuantity, *items[1].Quantity)
				egress, _ := tt.priceModel.EgressMBCents.Float64()
				require.Equal(t, egress, *items[1].UnitAmountDecimal)

				require.Equal(t, prefix+" - Segment Fee (Segment-Month)", *items[2].Description)
				require.Equal(t, expectedSegmentQuantity, *items[2].Quantity)
				segment, _ := tt.priceModel.SegmentMonthCents.Float64()
				require.Equal(t, segment, *items[2].UnitAmountDecimal)
			})
		}
	})
}

func TestService_PayInvoiceFromTokenBalance(t *testing.T) {
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

		tokenBalance := currency.AmountFromBaseUnits(1000, currency.USDollars)
		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice item
		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice
		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// setup storjscan wallet
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = satellite.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)
		_, err = satellite.DB.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      tokenBalance,
			Description: "token payment credit",
			Source:      billing.StorjScanEthereumSource,
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

		err = satellite.API.Payments.StripeService.PayInvoices(ctx, time.Time{})
		require.NoError(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx},
		})
		iter.Next()
		require.Equal(t, stripe.InvoiceStatusPaid, iter.Invoice().Status)

		// balance is in USDollars Micro, so it needs to be converted before comparison
		balance, err := satellite.DB.Billing().GetBalance(ctx, userID)
		balance = currency.AmountFromDecimal(balance.AsDecimal().Truncate(2), currency.USDollars)
		require.NoError(t, err)

		require.Equal(t, tokenBalance.BaseUnits()-invoiceBalance.BaseUnits(), balance.BaseUnits())
	})
}

func TestService_PayMultipleInvoiceFromTokenBalance(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// create user
		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice one
		inv1, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice two
		inv2, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &customer,
		})
		require.NoError(t, err)

		// create invoice items
		for _, info := range []struct {
			invID  string
			amount int64
		}{
			{inv1.ID, 75}, {inv2.ID, 100},
		} {
			for i := 0; i < 2; i++ {
				_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
					Params:   stripe.Params{Context: ctx},
					Amount:   stripe.Int64(info.amount),
					Currency: stripe.String(string(stripe.CurrencyUSD)),
					Customer: &customer,
					Invoice:  stripe.String(info.invID),
				})
				require.NoError(t, err)
			}
		}

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice one
		inv1, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv1.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv1.Status)

		// finalize invoice two
		inv2, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv2.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv2.Status)

		// setup storjscan wallet and user balance
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = satellite.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)
		// User balance is not enough to cover full amount of both invoices
		_, err = satellite.DB.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      currency.AmountFromBaseUnits(300, currency.USDollars),
			Description: "token payment credit",
			Source:      billing.StorjScanEthereumSource,
			Status:      billing.TransactionStatusCompleted,
			Type:        billing.TransactionTypeCredit,
			Metadata:    nil,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// attempt to apply token balance to invoices
		err = satellite.API.Payments.StripeService.InvoiceApplyTokenBalance(ctx, time.Time{})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.PayInvoices(ctx, time.Time{})
		require.NoError(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx},
		})
		for iter.Next() {
			if iter.Invoice().AmountRemaining == 0 {
				require.Equal(t, stripe.InvoiceStatusPaid, iter.Invoice().Status)
			} else {
				require.Equal(t, stripe.InvoiceStatusOpen, iter.Invoice().Status)
			}
		}
		require.NoError(t, iter.Err())
		balance, err := satellite.DB.Billing().GetBalance(ctx, userID)
		require.NoError(t, err)
		require.False(t, balance.IsNegative())
		require.Zero(t, balance.BaseUnits())
	})
}

func TestService_PayMultipleInvoiceForCustomer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// create user
		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice one
		inv1, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:               stripe.Params{Context: ctx},
			Customer:             &customer,
			DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
		})
		require.NoError(t, err)

		// create invoice two
		inv2, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:               stripe.Params{Context: ctx},
			Customer:             &customer,
			DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
		})
		require.NoError(t, err)

		// create invoice items
		for _, info := range []struct {
			invID  string
			amount int64
		}{
			{inv1.ID, 75}, {inv2.ID, 100},
		} {
			for i := 0; i < 2; i++ {
				_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
					Params:   stripe.Params{Context: ctx},
					Amount:   stripe.Int64(info.amount),
					Currency: stripe.String(string(stripe.CurrencyUSD)),
					Customer: &customer,
					Invoice:  stripe.String(info.invID),
				})
				require.NoError(t, err)
			}
		}

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		// finalize invoice one
		inv1, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv1.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv1.Status)

		// finalize invoice two
		inv2, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv2.ID, finalizeParams)
		inv2.Metadata = map[string]string{"PaymentMethod": stripe1.MockInvoicesPaySuccess}
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv2.Status)

		// setup storjscan wallet and user balance
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = satellite.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)
		// User balance is not enough to cover full amount of both invoices
		_, err = satellite.DB.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      currency.AmountFromBaseUnits(300, currency.USDollars),
			Description: "token payment credit",
			Source:      billing.StorjScanEthereumSource,
			Status:      billing.TransactionStatusCompleted,
			Type:        billing.TransactionTypeCredit,
			Metadata:    nil,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// attempt to pay user invoices, CC should be used to cover remainder after token balance
		err = satellite.API.Payments.StripeService.InvoiceApplyCustomerTokenBalance(ctx, customer)
		require.NoError(t, err)
		err = satellite.API.Payments.StripeService.PayCustomerInvoices(ctx, customer)
		require.NoError(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx},
		})
		var stripeInvoices []*stripe.Invoice
		for iter.Next() {
			stripeInvoices = append(stripeInvoices, iter.Invoice())
		}
		require.Equal(t, 2, len(stripeInvoices))
		require.Equal(t, stripe.InvoiceStatusPaid, stripeInvoices[0].Status)
		require.Equal(t, stripe.InvoiceStatusPaid, stripeInvoices[1].Status)
		require.NoError(t, iter.Err())
		balance, err := satellite.DB.Billing().GetBalance(ctx, userID)
		require.NoError(t, err)
		require.False(t, balance.IsNegative())
		require.Zero(t, balance.BaseUnits())

		// create another invoice
		inv3, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:               stripe.Params{Context: ctx},
			Customer:             &customer,
			DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
		})
		require.NoError(t, err)

		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(100),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
			Customer: &customer,
			Invoice:  stripe.String(inv3.ID),
		})
		require.NoError(t, err)

		err = satellite.API.Payments.StripeService.FinalizeInvoices(ctx)
		require.NoError(t, err)

		// deactivate user
		status := console.PendingDeletion
		err = satellite.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &status,
		})
		require.NoError(t, err)

		// attempt to pay user invoices should not succeed since the user is now deactivated.
		err = satellite.API.Payments.StripeService.PayCustomerInvoices(ctx, customer)
		require.Error(t, err)
	})
}

func TestFailPendingInvoicePayment(t *testing.T) {
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

		tokenBalance := currency.AmountFromBaseUnits(1000, currency.USDollars)
		invoiceBalance := currency.AmountFromBaseUnits(800, currency.USDollars)
		usdCurrency := string(stripe.CurrencyUSD)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)
		customer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// create invoice
		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:               stripe.Params{Context: ctx},
			Customer:             &customer,
			DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			Metadata:             map[string]string{"mock": stripe1.MockInvoicesPayFailure},
		})
		require.NoError(t, err)

		_, err = satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(invoiceBalance.BaseUnits()),
			Currency: stripe.String(usdCurrency),
			Customer: &customer,
			Invoice:  stripe.String(inv.ID),
		})
		require.NoError(t, err)

		// finalize invoice
		err = satellite.API.Payments.StripeService.FinalizeInvoices(ctx)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		// setup storjscan wallet
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = satellite.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)
		_, err = satellite.DB.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      tokenBalance,
			Description: "token payment credit",
			Source:      billing.StorjScanEthereumSource,
			Status:      billing.TransactionStatusCompleted,
			Type:        billing.TransactionTypeCredit,
			Metadata:    nil,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// run apply token balance to see if there are no unexpected errors
		err = payments.StripeService.InvoiceApplyTokenBalance(ctx, time.Time{})
		require.Error(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx},
		})
		iter.Next()
		require.Equal(t, stripe.InvoiceStatusOpen, iter.Invoice().Status)

		// balance is in USDollars Micro, so it needs to be converted before comparison
		balance, err := satellite.DB.Billing().GetBalance(ctx, userID)
		balance = currency.AmountFromDecimal(balance.AsDecimal().Truncate(2), currency.USDollars)
		require.NoError(t, err)

		// verify user balance wasn't changed
		require.Equal(t, tokenBalance.BaseUnits(), balance.BaseUnits())
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
						config.Payments.StripeCoinPayments.StripeFreeTierCouponID = stripe1.MockCouponID1
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

				require.NoError(t, payments.StripeService.GenerateInvoices(ctx, start, false, false, false))

				// ensure project record was generated
				err = satellite.DB.StripeCoinPayments().ProjectRecords().Check(ctx, proj.ID, start, end)
				require.ErrorIs(t, stripe1.ErrProjectRecordExists, err)

				rec, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, proj.ID, start, end)
				require.NotNil(t, rec)
				require.NoError(t, err)

				// validate generated invoices
				cusID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
				require.NoError(t, err)
				invoice, hasInvoice := getCustomerInvoice(ctx, payments.StripeClient, cusID)
				invoiceItems := getCustomerInvoiceItems(ctx, payments.StripeClient, cusID)

				// If invoicing empty usage invoices was skipped, then we don't
				// expect an invoice or invoice items.
				if !testCase.expectInvoice {
					require.False(t, hasInvoice, "expected no invoice but got one")
					require.Nil(t, invoice, "expected no invoice but got one")
					require.Empty(t, invoiceItems, "not expecting any invoice items")
					return
				}

				// Otherwise, we expect one or more line items that have been
				// associated with the newly created invoice.
				require.True(t, hasInvoice, "expected invoice but did not get one")
				require.NotNil(t, invoice, "expected invoice but did not get one")
				require.NotZero(t, len(invoiceItems), "expecting one or more invoice items")
				for _, item := range invoiceItems {
					require.Contains(t, item.Metadata, "projectID")
					require.Equal(t, item.Metadata["projectID"], proj.ID.String())
					require.NotNil(t, item.Invoice)
					require.Equal(t, invoice.ID, item.Invoice.ID)
				}
			})
		})
	}
}

func getCustomerInvoice(ctx context.Context, stripeClient stripe1.Client, cusID string) (*stripe.Invoice, bool) {
	iter := stripeClient.Invoices().List(&stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &cusID,
	})
	if iter.Next() {
		return iter.Invoice(), true
	}
	return nil, false
}

func getCustomerInvoiceItems(ctx context.Context, stripeClient stripe1.Client, cusID string) (items []*stripe.InvoiceItem) {
	iter := stripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &cusID,
	})
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
			{"default pricing - user agent is not valid partner name", []byte("invalid/v0.0"), defaultModel},
			{"partner pricing - user agent is partner name", []byte(partnerName), partnerModel},
			{"partner pricing - user agent prefixed with partner name", []byte(partnerName + " invalid/v0.0"), partnerModel},
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

				bucket, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      testrand.BucketName(),
					ProjectID: project.ID,
					UserAgent: tt.userAgent,
				})
				require.NoError(t, err)

				_, err = sat.DB.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(bucket.Name),
					UserAgent:  tt.userAgent,
				})
				require.NoError(t, err)

				err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, project.ID, []byte(bucket.Name),
					pb.PieceAction_GET, memory.TB.Int64(), 0, period)
				require.NoError(t, err)

				err = sat.API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
				require.NoError(t, err)

				err = sat.API.Payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
				require.NoError(t, err)

				cusID, err := sat.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
				require.NoError(t, err)

				items := getCustomerInvoiceItems(ctx, sat.API.Payments.StripeClient, cusID)
				require.Len(t, items, 3)
				sort.Slice(items, func(i, j int) bool {
					return items[i].Description < items[j].Description
				})
				egress, _ := tt.expectedPrice.EgressMBCents.Float64()
				require.Equal(t, egress, items[0].UnitAmountDecimal)
				segment, _ := tt.expectedPrice.SegmentMonthCents.Float64()
				require.Equal(t, segment, items[1].UnitAmountDecimal)
				storage, _ := tt.expectedPrice.StorageMBMonthCents.Float64()
				require.Equal(t, storage, items[2].UnitAmountDecimal)
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
		due := time.Now().Add(14 * 24 * time.Hour).Unix()

		for _, cusID := range []string{cus1, cus2} {
			_, err := satellite.API.Payments.StripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   stripe.Int64(100),
				Currency: stripe.String(string(stripe.CurrencyUSD)),
				Customer: stripe.String(cusID),
			})
			require.NoError(t, err)
		}

		inv, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &cus1,
		})
		require.NoError(t, err)

		finalizeParams := &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}}

		inv, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(inv.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		invWithDue, err := satellite.API.Payments.StripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &cus2,
			DueDate:  &due,
		})
		require.NoError(t, err)

		invWithDue, err = satellite.API.Payments.StripeClient.Invoices().FinalizeInvoice(invWithDue.ID, finalizeParams)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, invWithDue.Status)

		err = satellite.API.Payments.StripeService.PayInvoices(ctx, time.Time{})
		require.NoError(t, err)

		iter := satellite.API.Payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx},
		})
		for iter.Next() {
			i := iter.Invoice()
			if i.ID == inv.ID {
				require.Equal(t, stripe.InvoiceStatusPaid, i.Status)
			}
			// when due date is set invoice should not be paid
			if i.ID == invWithDue.ID {
				require.Equal(t, stripe.InvoiceStatusOpen, i.Status)
			}
		}
	})
}

func TestRemoveExpiredPackageCredit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		p := satellite.API.Payments
		u0 := planet.Uplinks[0].Projects[0].Owner.ID
		u1 := planet.Uplinks[1].Projects[0].Owner.ID
		u2 := planet.Uplinks[2].Projects[0].Owner.ID
		u3 := planet.Uplinks[3].Projects[0].Owner.ID

		credit := int64(1000)
		pkgDesc := "test package plan"
		now := time.Now()
		expiredPurchase := now.AddDate(-1, -1, 0)

		removeExpiredCredit := func(u uuid.UUID, expectAlert bool, purchaseTime *time.Time) {
			require.NoError(t, p.Accounts.UpdatePackage(ctx, u, &pkgDesc, purchaseTime))
			cPage, err := satellite.API.DB.StripeCoinPayments().Customers().List(ctx, uuid.UUID{}, 10, time.Now().Add(1*time.Hour))
			require.NoError(t, err)
			var c stripe1.Customer
			for _, cus := range cPage.Customers {
				if cus.UserID == u {
					c = cus
				}
			}

			alertSent, err := p.StripeService.RemoveExpiredPackageCredit(ctx, stripe1.Customer{
				ID:                 c.ID,
				UserID:             c.UserID,
				PackagePlan:        c.PackagePlan,
				PackagePurchasedAt: c.PackagePurchasedAt,
			})
			require.NoError(t, err)
			if expectAlert {
				require.True(t, alertSent)
			} else {
				require.False(t, alertSent)
			}
		}

		checkCreditAndPackage := func(u uuid.UUID, expectedBalance int64, expectNilPackage bool) {
			b, err := p.Accounts.Balances().Get(ctx, u)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(expectedBalance), b.Credits)

			dbPkgInfo, dbPurchaseTime, err := p.StripeService.Accounts().GetPackageInfo(ctx, u)
			require.NoError(t, err)
			if expectNilPackage {
				require.Nil(t, dbPkgInfo)
				require.Nil(t, dbPurchaseTime)
			} else {
				require.NotNil(t, dbPkgInfo)
				require.NotNil(t, dbPurchaseTime)
			}
		}

		t.Run("nil package plan returns safely", func(t *testing.T) {
			_, err := p.StripeService.RemoveExpiredPackageCredit(ctx, stripe1.Customer{
				ID:                 "test-customer-ID",
				UserID:             testrand.UUID(),
				PackagePlan:        nil,
				PackagePurchasedAt: &now,
			})
			require.NoError(t, err)
		})

		t.Run("nil package purchase time returns safely", func(t *testing.T) {
			_, err := p.StripeService.RemoveExpiredPackageCredit(ctx, stripe1.Customer{
				ID:                 "test-customer-ID",
				UserID:             testrand.UUID(),
				PackagePlan:        new(string),
				PackagePurchasedAt: nil,
			})
			require.NoError(t, err)
		})

		t.Run("package not expired retains credit", func(t *testing.T) {
			b, err := p.Accounts.Balances().ApplyCredit(ctx, u3, credit, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(credit), b.Credits)

			removeExpiredCredit(u3, false, &now)
			checkCreditAndPackage(u3, credit, false)
		})

		t.Run("used all credit", func(t *testing.T) {
			b, err := p.Accounts.Balances().ApplyCredit(ctx, u0, credit, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(credit), b.Credits)

			// remove credit as if they used it all
			b, err = p.Accounts.Balances().ApplyCredit(ctx, u0, -credit, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(0), b.Credits)

			removeExpiredCredit(u0, false, &expiredPurchase)
			checkCreditAndPackage(u0, 0, true)
		})

		t.Run("has remaining credit but no credit source other than package", func(t *testing.T) {
			b, err := p.Accounts.Balances().ApplyCredit(ctx, u1, credit, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(credit), b.Credits)

			// remove some credit, but not all, as if it were used
			toRemove := credit / 2
			remaining := credit - toRemove
			b, err = p.Accounts.Balances().ApplyCredit(ctx, u1, -toRemove, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(remaining), b.Credits)

			removeExpiredCredit(u1, false, &expiredPurchase)
			checkCreditAndPackage(u1, 0, true)
		})

		t.Run("has additional credit source", func(t *testing.T) {
			b, err := p.Accounts.Balances().ApplyCredit(ctx, u2, credit, pkgDesc)
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(credit), b.Credits)

			// give additional credit
			additional := int64(2000)
			b, err = p.Accounts.Balances().ApplyCredit(ctx, u2, additional, "additional credit")
			require.NoError(t, err)
			require.Equal(t, decimal.NewFromInt(credit+additional), b.Credits)

			removeExpiredCredit(u2, true, &expiredPurchase)
			checkCreditAndPackage(u2, credit+additional, false)
		})
	})
}

func TestService_PayInvoiceBillingID(t *testing.T) {
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

		storageHours := 24

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "testuser",
			Email:    "user@test",
		}, 1)
		require.NoError(t, err)

		// create billing customer ID
		billingUser, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "billinguser",
			Email:    "billing@test",
		}, 1)
		require.NoError(t, err)
		billingCustomer, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, billingUser.ID)
		require.NoError(t, err)

		// set billing customer ID
		_, err = satellite.DB.StripeCoinPayments().Customers().UpdateBillingCustomerID(ctx, user.ID, &billingCustomer)
		require.NoError(t, err)

		project, err := satellite.AddProject(ctx, user.ID, "testproject")
		require.NoError(t, err)

		projectsEgress := int64(10) * memory.GiB.Int64()
		projectsStorage := int64(1) * memory.TiB.Int64()
		totalSegments := int64(1)
		generateProjectStorage(ctx, t, satellite.DB,
			project.ID,
			period,
			period.Add(time.Duration(storageHours)*time.Hour),
			projectsEgress,
			projectsStorage,
			totalSegments)
		// verify that the project doesn't have records yet
		projectRecord, err := satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, project.ID, start, end)
		require.NoError(t, err)
		require.Nil(t, projectRecord)

		err = payments.StripeService.PrepareInvoiceProjectRecords(ctx, period, false)
		require.NoError(t, err)

		projectRecord, err = satellite.DB.StripeCoinPayments().ProjectRecords().Get(ctx, project.ID, start, end)
		require.NoError(t, err)
		require.NotNil(t, projectRecord)
		require.Equal(t, project.ID, projectRecord.ProjectID)
		require.Equal(t, projectsEgress, projectRecord.Egress)

		expectedStorage := float64(projectsStorage * int64(storageHours))
		require.Equal(t, expectedStorage, projectRecord.Storage)

		expectedSegmentsCount := float64((1) * storageHours)
		require.Equal(t, expectedSegmentsCount, projectRecord.Segments)

		// run all parts of invoice generation to see if there are no unexpected errors
		err = payments.StripeService.InvoiceApplyProjectRecords(ctx, period)
		require.NoError(t, err)

		err = payments.StripeService.CreateInvoices(ctx, period, false)
		require.NoError(t, err)

		itr := payments.StripeClient.Invoices().List(&stripe.InvoiceListParams{})
		require.True(t, itr.Next())
		// invoice should go to the billing customer not the customer with the usage
		require.Equal(t, billingCustomer, itr.Invoice().Customer.ID)
		require.NoError(t, itr.Err())
	})
}
