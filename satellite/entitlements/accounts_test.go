// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestAccountEntitlements(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		entSvc := entitlements.NewService(zaptest.NewLogger(t), db.Console().Entitlements())
		accounts := entSvc.Accounts()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "accounts@storj.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		userID := user.ID

		t.Run("no entitlement yields the zero value", func(t *testing.T) {
			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.True(t, got.IsZero())
		})

		t.Run("round trip preserves order", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{"us_bank_account", "card"},
			})
			require.NoError(t, err)

			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, []string{"us_bank_account", "card"}, got.InvoicePaymentMethods)
			require.Empty(t, got.InvoiceCollectionMethod)
			require.Zero(t, got.InvoiceDaysUntilDue)
		})

		t.Run("methods are normalized and de-duplicated", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{" Card ", "CARD", "customer_balance"},
			})
			require.NoError(t, err)

			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, []string{"card", "customer_balance"}, got.InvoicePaymentMethods)
		})

		t.Run("empty method is rejected and nothing is written", func(t *testing.T) {
			before, err := accounts.Get(ctx, userID)
			require.NoError(t, err)

			err = accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{"card", "  "},
			})
			require.Error(t, err)
			require.True(t, entitlements.ErrInvalidInvoicePayment.Has(err))

			after, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, before.InvoicePaymentMethods, after.InvoicePaymentMethods)
		})

		t.Run("send_invoice requires a due period", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods:   []string{"customer_balance"},
				InvoiceCollectionMethod: entitlements.CollectionMethodSendInvoice,
			})
			require.Error(t, err)
			require.True(t, entitlements.ErrInvalidInvoicePayment.Has(err))
		})

		t.Run("charge_automatically rejects a due period", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods:   []string{"card"},
				InvoiceCollectionMethod: entitlements.CollectionMethodChargeAutomatically,
				InvoiceDaysUntilDue:     30,
			})
			require.Error(t, err)
			require.True(t, entitlements.ErrInvalidInvoicePayment.Has(err))
		})

		t.Run("unknown collection method is rejected", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoiceCollectionMethod: "wire_transfer",
			})
			require.Error(t, err)
			require.True(t, entitlements.ErrInvalidInvoicePayment.Has(err))
		})

		t.Run("send_invoice round trip", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods:   []string{"customer_balance"},
				InvoiceCollectionMethod: entitlements.CollectionMethodSendInvoice,
				InvoiceDaysUntilDue:     30,
			})
			require.NoError(t, err)

			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, []string{"customer_balance"}, got.InvoicePaymentMethods)
			require.Equal(t, entitlements.CollectionMethodSendInvoice, got.InvoiceCollectionMethod)
			require.Equal(t, 30, got.InvoiceDaysUntilDue)
		})

		t.Run("the zero value clears the entitlement", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{})
			require.NoError(t, err)

			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.True(t, got.IsZero())
		})

		t.Run("account and license scopes do not collide", func(t *testing.T) {
			err := accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{"card"},
			})
			require.NoError(t, err)

			err = entSvc.Licenses().Set(ctx, userID, entitlements.AccountLicenses{
				Licenses: []entitlements.AccountLicense{{Type: entitlements.OMLicenseType, Count: 3}},
			})
			require.NoError(t, err)

			// Writing licenses must not disturb the account features, and vice versa.
			gotAccount, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, []string{"card"}, gotAccount.InvoicePaymentMethods)

			err = accounts.SetInvoicePayment(ctx, userID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{"customer_balance"},
				// customer_balance is only collectable on an invoice the customer pays.
				InvoiceCollectionMethod: entitlements.CollectionMethodSendInvoice,
				InvoiceDaysUntilDue:     14,
			})
			require.NoError(t, err)

			gotLicenses, err := entSvc.Licenses().Get(ctx, userID)
			require.NoError(t, err)
			require.Len(t, gotLicenses.Licenses, 1)
			require.Equal(t, 3, gotLicenses.Licenses[0].Count)
		})

		t.Run("delete removes the account scope", func(t *testing.T) {
			require.NoError(t, accounts.DeleteByUserID(ctx, userID))

			got, err := accounts.Get(ctx, userID)
			require.NoError(t, err)
			require.True(t, got.IsZero())
		})
	})
}
