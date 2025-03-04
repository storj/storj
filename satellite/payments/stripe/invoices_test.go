// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/billing"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

func TestInvoices(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID
		price := int64(1000)
		desc := "test payment intent description"

		t.Run("Create with unknown user fails", func(t *testing.T) {
			pi, err := satellite.API.Payments.Accounts.Invoices().Create(ctx, testrand.UUID(), price, desc)
			require.Error(t, err)
			require.Nil(t, pi)
		})
		t.Run("Create returns error when underlying StripeInvoices.Create returns error", func(t *testing.T) {
			pi, err := satellite.API.Payments.Accounts.Invoices().Create(ctx, testrand.UUID(), price, stripe1.MockInvoicesNewFailure)
			require.Error(t, err)
			require.Nil(t, pi)
		})
		t.Run("Pay returns error with unknown invoice ID", func(t *testing.T) {
			confirmedPI, err := satellite.API.Payments.Accounts.Invoices().Pay(ctx, "unknown_id", "test_payment_method")
			require.Error(t, err)
			require.Nil(t, confirmedPI)
		})
		t.Run("Create and Pay success", func(t *testing.T) {
			pi, err := satellite.API.Payments.Accounts.Invoices().Create(ctx, userID, price, desc)
			require.NoError(t, err)
			require.NotNil(t, pi)

			confirmedPI, err := satellite.API.Payments.Accounts.Invoices().Pay(ctx, pi.ID, stripe1.MockInvoicesPaySuccess)
			require.NoError(t, err)
			require.Equal(t, pi.ID, confirmedPI.ID)
			require.Equal(t, string(stripe.InvoiceStatusPaid), confirmedPI.Status)
		})
		t.Run("Pay returns error when underlying StripeInvoices.Pay returns error", func(t *testing.T) {
			pi, err := satellite.API.Payments.Accounts.Invoices().Create(ctx, userID, price, desc)
			require.NoError(t, err)
			require.NotNil(t, pi)

			confirmedPI, err := satellite.API.Payments.Accounts.Invoices().Pay(ctx, pi.ID, stripe1.MockInvoicesPayFailure)
			require.Error(t, err)
			require.Nil(t, confirmedPI)

			_, err = satellite.API.Payments.Accounts.Invoices().Pay(ctx, pi.ID, stripe1.MockInvoicesPaySuccess)
			require.NoError(t, err)
		})
		t.Run("Create and Get success", func(t *testing.T) {
			pi, err := satellite.API.Payments.Accounts.Invoices().Create(ctx, userID, price, desc)
			require.NoError(t, err)
			require.NotNil(t, pi)

			pi2, err := satellite.API.Payments.Accounts.Invoices().Get(ctx, pi.ID)
			require.NoError(t, err)
			require.Equal(t, pi.ID, pi2.ID)
			require.Equal(t, pi.Status, pi2.Status)
			require.Equal(t, pi.Amount, pi2.Amount)
		})
		t.Run("List failed", func(t *testing.T) {
			stripeInvoices := satellite.API.Payments.StripeClient.Invoices()
			invoices := satellite.API.Payments.Accounts.Invoices()

			pi, err := invoices.Create(ctx, userID, price, desc)
			require.NoError(t, err)
			require.NotNil(t, pi)

			_, err = stripeInvoices.FinalizeInvoice(pi.ID, &stripe.InvoiceFinalizeInvoiceParams{Params: stripe.Params{Context: ctx}})
			require.NoError(t, err)

			inv, err := stripeInvoices.Get(pi.ID, &stripe.InvoiceParams{Params: stripe.Params{Context: ctx}})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)
			require.False(t, inv.Attempted)

			failed, err := invoices.ListFailed(ctx, &userID)
			require.NoError(t, err)
			require.Empty(t, failed)

			inv, err = stripeInvoices.Pay(pi.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPayFailure),
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)
			require.True(t, inv.Attempted)

			failed, err = invoices.ListFailed(ctx, &userID)
			require.NoError(t, err)
			require.Len(t, failed, 1)
			require.Equal(t, pi.ID, failed[0].ID)
		})
	})
}

func TestPayOverdueInvoices(t *testing.T) {
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

		// attempt to pay user invoices, CC should be used to cover remainder after token balance
		err = satellite.API.Payments.Accounts.Invoices().AttemptPayOverdueInvoices(ctx, userID)
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
	})
}
