// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

// TestBillingFreezeChore_SendInvoiceEntitlementNotFrozen covers the reason the invoice
// payment entitlement carries a collection method at all. An account entitled to pay by a
// method it has to settle itself will leave its invoice open and unpaid, which looks
// exactly like a failed card charge. The due date that send_invoice produces is what keeps
// such an account out of the billing warn and freeze pipeline.
func TestBillingFreezeChore_SendInvoiceEntitlementNotFrozen(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.Entitlements.Enabled = true
				config.Payments.StripeCoinPayments.ApplyInvoicePaymentEntitlement = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		stripeService := sat.API.Payments.StripeService
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		freezeService := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.AccountFreeze.BillingFreezeChore

		chore.Loop.Pause()
		chore.TestSetFreezeService(freezeService)
		chore.TestSetNow(time.Now)

		start := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.May, 31, 23, 59, 59, 0, time.UTC)

		// billUser gives the account the entitlement, invoices it, and lets the charge
		// fail: the customer has no usable payment method either way, so what separates
		// the two accounts below is only how the failure is classified.
		billUser := func(name, email string, feats entitlements.AccountFeatures) *console.User {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: name,
				Email:    email,
				Kind:     console.PaidUser,
			}, 1)
			require.NoError(t, err)

			require.NoError(t, sat.API.Entitlements.Service.Accounts().SetInvoicePayment(ctx, user.ID, feats))

			cusID, err := customerDB.GetCustomerID(ctx, user.ID)
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   stripe.Int64(10_000),
				Currency: stripe.String(string(stripe.CurrencyUSD)),
				Customer: stripe.String(cusID),
			})
			require.NoError(t, err)

			inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)
			require.NotNil(t, inv)

			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPayFailure),
			})
			require.Error(t, err)

			return user
		}

		// The send_invoice account is the one under test; the charge_automatically
		// account is the contrast that shows the due date is what does the work.
		sendUser := billUser("Bank Transfer User", "banktransfer@mail.test", entitlements.AccountFeatures{
			InvoicePaymentMethods:   []string{"customer_balance"},
			InvoiceCollectionMethod: entitlements.CollectionMethodSendInvoice,
			InvoiceDaysUntilDue:     30,
		})
		autoUser := billUser("Card User", "carduser@mail.test", entitlements.AccountFeatures{
			InvoicePaymentMethods:   []string{"card"},
			InvoiceCollectionMethod: entitlements.CollectionMethodChargeAutomatically,
		})

		// Both invoices are open and attempted, but only the automatically charged one
		// counts as a failed payment.
		failed, err := invoicesDB.ListFailed(ctx, &sendUser.ID)
		require.NoError(t, err)
		require.Empty(t, failed)

		autoFailed, err := invoicesDB.ListFailed(ctx, &autoUser.ID)
		require.NoError(t, err)
		require.Len(t, autoFailed, 1)

		chore.Loop.TriggerWait()

		freezes, err := freezeService.GetAll(ctx, sendUser.ID)
		require.NoError(t, err)
		require.Nil(t, freezes.BillingWarning)
		require.Nil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)

		autoFreezes, err := freezeService.GetAll(ctx, autoUser.ID)
		require.NoError(t, err)
		require.NotNil(t, autoFreezes.BillingWarning)
	})
}
