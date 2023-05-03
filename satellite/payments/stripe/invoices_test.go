// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
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

			confirmedPI, err := satellite.API.Payments.Accounts.Invoices().Pay(ctx, pi.ID, "test_payment_method")
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
		})
	})
}
