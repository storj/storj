// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

func TestAutoFreezeChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, newFreezeTrackerMock(t))
		chore := sat.Core.Payments.AccountFreeze
		chore.TestSetFreezeService(service)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 1)
		require.NoError(t, err)

		cus1, err := customerDB.GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		amount := int64(100)
		curr := string(stripe.CurrencyUSD)

		t.Run("No freeze event for paid invoice", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			item, err := stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
			})
			require.NoError(t, err)

			items := make([]*stripe.InvoiceUpcomingInvoiceItemParams, 0, 1)
			items = append(items, &stripe.InvoiceUpcomingInvoiceItemParams{
				InvoiceItem: &item.ID,
				Amount:      &amount,
				Currency:    &curr,
			})
			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:       stripe.Params{Context: ctx},
				Customer:     &cus1,
				InvoiceItems: items,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPaySuccess
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx)
			require.NoError(t, err)
			require.Equal(t, 0, len(failed))

			chore.Loop.TriggerWait()

			// user should not be warned or frozen.
			freeze, warning, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, warning)
			require.Nil(t, freeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().AddDate(0, 0, 50)
			})
			chore.Loop.TriggerWait()

			// user should still not be warned or frozen.
			freeze, warning, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freeze)
			require.Nil(t, warning)
		})

		t.Run("Freeze event for failed invoice", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			item, err := stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
			})
			require.NoError(t, err)

			items := make([]*stripe.InvoiceUpcomingInvoiceItemParams, 0, 1)
			items = append(items, &stripe.InvoiceUpcomingInvoiceItemParams{
				InvoiceItem: &item.ID,
				Amount:      &amount,
				Currency:    &curr,
			})
			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:       stripe.Params{Context: ctx},
				Customer:     &cus1,
				InvoiceItems: items,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))
			require.Equal(t, inv.ID, failed[0].ID)

			chore.Loop.TriggerWait()

			// user should be warned the first time
			freeze, warning, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, warning)
			require.Nil(t, freeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after grace period
				return time.Now().AddDate(0, 0, 50)
			})
			chore.Loop.TriggerWait()

			// user should be frozen this time around
			freeze, _, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freeze)
		})

		t.Run("Storjscan exceptions", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			storjscanUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "storjscanuser@mail.test",
			}, 1)
			require.NoError(t, err)

			// create a wallet and transaction for the new user in storjscan
			address, err := blockchain.BytesToAddress(testrand.Bytes(20))
			require.NoError(t, err)
			require.NoError(t, sat.DB.Wallets().Add(ctx, storjscanUser.ID, address))
			cachedPayments := []storjscan.CachedPayment{
				{
					From:        blockchaintest.NewAddress(),
					To:          address,
					TokenValue:  currency.AmountFromBaseUnits(1000, currency.StorjToken),
					USDValue:    currency.AmountFromBaseUnits(testrand.Int63n(1000), currency.USDollarsMicro),
					BlockHash:   blockchaintest.NewHash(),
					Transaction: blockchaintest.NewHash(),
					Status:      payments.PaymentStatusConfirmed,
					Timestamp:   time.Now(),
				},
			}
			require.NoError(t, sat.DB.StorjscanPayments().InsertBatch(ctx, cachedPayments))

			storjscanCus, err := customerDB.GetCustomerID(ctx, storjscanUser.ID)
			require.NoError(t, err)

			item, err := stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &storjscanCus,
			})
			require.NoError(t, err)

			items := make([]*stripe.InvoiceUpcomingInvoiceItemParams, 0, 1)
			items = append(items, &stripe.InvoiceUpcomingInvoiceItemParams{
				InvoiceItem: &item.ID,
				Amount:      &amount,
				Currency:    &curr,
			})
			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:       stripe.Params{Context: ctx},
				Customer:     &storjscanCus,
				InvoiceItems: items,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx)
			require.NoError(t, err)
			require.Equal(t, 2, len(failed))
			invFound := false
			for _, failedInv := range failed {
				if failedInv.ID == inv.ID {
					invFound = true
					break
				}
			}
			require.True(t, invFound)

			chore.Loop.TriggerWait()

			// user should not be warned or frozen due to storjscan payments
			freeze, warning, err := service.GetAll(ctx, storjscanUser.ID)
			require.NoError(t, err)
			require.Nil(t, warning)
			require.Nil(t, freeze)
		})
	})
}

type freezeTrackerMock struct {
	t            *testing.T
	freezeCounts map[string]int
	warnCounts   map[string]int
}

func newFreezeTrackerMock(t *testing.T) *freezeTrackerMock {
	return &freezeTrackerMock{
		t:            t,
		freezeCounts: map[string]int{},
		warnCounts:   map[string]int{},
	}
}

// The following functions are implemented from analytics.FreezeTracker.
// They mock/test to make sure freeze events are sent just once.

func (mock *freezeTrackerMock) TrackAccountFrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnfrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnwarned(_ uuid.UUID, email string) {
	mock.warnCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountFreezeWarning(_ uuid.UUID, email string) {
	mock.warnCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackLargeUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}

func (mock *freezeTrackerMock) TrackStorjscanUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}
