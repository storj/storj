// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

// TestCreateInvoice_PaymentEntitlement covers how an account's invoice payment
// entitlement reaches the Stripe invoice, and that accounts without one are unaffected.
func TestCreateInvoice_PaymentEntitlement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Entitlements.Enabled = true
				config.Payments.StripeCoinPayments.ApplyInvoicePaymentEntitlement = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		start := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.May, 31, 23, 59, 59, 0, time.UTC)

		sat := planet.Satellites[0]
		stripeService := sat.API.Payments.StripeService
		stripeClient := sat.API.Payments.StripeClient
		accounts := sat.API.Entitlements.Service.Accounts()

		// newCustomer gives each subtest its own user and Stripe customer, so an
		// entitlement set by one cannot leak into another. The user is a real console
		// record because the invoicing paths look it up, and a paid one because free
		// accounts are billing exempt.
		newCustomer := func(t *testing.T) (*console.User, string) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Invoice Payment User",
				Email:    string(testrand.RandAlphaNumeric(12)) + "@storj.test",
				Kind:     console.PaidUser,
			}, 1)
			require.NoError(t, err)

			cusID, err := sat.API.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   stripe.Int64(10_000),
				Currency: stripe.String(string(stripe.CurrencyUSD)),
				Customer: stripe.String(cusID),
			})
			require.NoError(t, err)

			return user, cusID
		}

		sendInvoice := entitlements.AccountFeatures{
			InvoicePaymentMethods:   []string{"customer_balance"},
			InvoiceCollectionMethod: entitlements.CollectionMethodSendInvoice,
			InvoiceDaysUntilDue:     30,
		}

		methodTypes := func(inv *stripe.Invoice) []string {
			if inv.PaymentSettings == nil {
				return nil
			}
			out := make([]string, 0, len(inv.PaymentSettings.PaymentMethodTypes))
			for _, t := range inv.PaymentSettings.PaymentMethodTypes {
				out = append(out, string(t))
			}
			return out
		}

		t.Run("no entitlement leaves the invoice as it was before", func(t *testing.T) {
			user, cusID := newCustomer(t)

			inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)

			require.Nil(t, inv.PaymentSettings)
			require.Empty(t, string(inv.CollectionMethod))
			require.Zero(t, inv.DueDate)
		})

		for _, tt := range []struct {
			name           string
			feats          entitlements.AccountFeatures
			wantMethods    []string
			wantCollection stripe.InvoiceCollectionMethod
		}{
			{
				name:        "entitled methods are applied exactly, without card",
				feats:       entitlements.AccountFeatures{InvoicePaymentMethods: []string{"us_bank_account"}},
				wantMethods: []string{"us_bank_account"},
			},
			{
				// The set of methods Stripe accepts outgrows any pinned SDK version, so
				// a method the SDK does not know must still reach Stripe unchanged.
				name:        "a method unknown to the pinned SDK passes through",
				feats:       entitlements.AccountFeatures{InvoicePaymentMethods: []string{"blik"}},
				wantMethods: []string{"blik"},
			},
			{
				name:        "order is preserved",
				feats:       entitlements.AccountFeatures{InvoicePaymentMethods: []string{"us_bank_account", "card"}},
				wantMethods: []string{"us_bank_account", "card"},
			},
			{
				name:           "send_invoice carries a due date",
				feats:          sendInvoice,
				wantMethods:    []string{"customer_balance"},
				wantCollection: stripe.InvoiceCollectionMethodSendInvoice,
			},
			{
				name: "charge_automatically carries no due date",
				feats: entitlements.AccountFeatures{
					InvoicePaymentMethods:   []string{"card"},
					InvoiceCollectionMethod: entitlements.CollectionMethodChargeAutomatically,
				},
				wantMethods:    []string{"card"},
				wantCollection: stripe.InvoiceCollectionMethodChargeAutomatically,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				user, cusID := newCustomer(t)

				require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, tt.feats))

				inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
				require.NoError(t, err)

				require.Equal(t, tt.wantMethods, methodTypes(inv))
				require.Equal(t, tt.wantCollection, inv.CollectionMethod)
				if tt.wantCollection == stripe.InvoiceCollectionMethodSendInvoice {
					require.NotZero(t, inv.DueDate)
				} else {
					require.Zero(t, inv.DueDate)
				}
				// Creation never auto advances: every draft stays in the satellite's
				// finalize gate, and delivery is decided at finalization instead.
				require.False(t, inv.AutoAdvance)
			})
		}

		t.Run("kill switch off ignores a stored entitlement", func(t *testing.T) {
			user, cusID := newCustomer(t)

			require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, entitlements.AccountFeatures{
				InvoicePaymentMethods: []string{"us_bank_account"},
			}))

			stripeService.TestSetApplyInvoicePaymentEntitlement(false)
			defer stripeService.TestSetApplyInvoicePaymentEntitlement(true)

			inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)
			require.Nil(t, inv.PaymentSettings)
		})

		// Delivery and dunning are handed to Stripe only at finalization, and only while
		// the flag is on: with it off, a send_invoice draft, standing in for one created
		// outside the satellite, keeps being finalized silently.
		for _, apply := range []bool{true, false} {
			t.Run(fmt.Sprintf("send_invoice finalized with flag %t", apply), func(t *testing.T) {
				user, cusID := newCustomer(t)

				require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, sendInvoice))

				inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
				require.NoError(t, err)

				stripeService.TestSetApplyInvoicePaymentEntitlement(apply)
				defer stripeService.TestSetApplyInvoicePaymentEntitlement(true)

				require.NoError(t, stripeService.FinalizeInvoices(ctx))

				finalized, err := stripeClient.Invoices().Get(inv.ID, &stripe.InvoiceParams{
					Params: stripe.Params{Context: ctx},
				})
				require.NoError(t, err)
				require.Equal(t, stripe.InvoiceStatusOpen, finalized.Status)
				require.Equal(t, apply, finalized.AutoAdvance)
			})
		}

		t.Run("retrying overdue payments leaves a send_invoice invoice alone", func(t *testing.T) {
			user, cusID := newCustomer(t)

			require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, sendInvoice))

			inv, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)
			require.NoError(t, stripeService.FinalizeInvoices(ctx))

			// This is what the console's retry button and the billing freeze chore call.
			require.NoError(t, sat.API.Payments.Accounts.Invoices().AttemptPayOverdueInvoices(ctx, user.ID))

			got, err := stripeClient.Invoices().Get(inv.ID, &stripe.InvoiceParams{
				Params: stripe.Params{Context: ctx},
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, got.Status)
			require.False(t, got.Attempted)
		})

		t.Run("an existing draft is brought up to date", func(t *testing.T) {
			user, cusID := newCustomer(t)

			// The draft-reuse path only runs when a minimum charge applies.
			stripeService.TestSetMinimumChargeCfg(5_000, nil)
			defer stripeService.TestSetMinimumChargeCfg(0, nil)

			// First run: no entitlement yet, so the draft carries no payment settings.
			first, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)
			require.Nil(t, first.PaymentSettings)

			require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, sendInvoice))

			// Second run reuses that draft and must apply the entitlement granted since.
			second, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
			require.NoError(t, err)
			require.Equal(t, first.ID, second.ID)
			require.Equal(t, []string{"customer_balance"}, methodTypes(second))
			require.Equal(t, stripe.InvoiceCollectionMethodSendInvoice, second.CollectionMethod)
			require.NotZero(t, second.DueDate)
		})

		// A draft that got send_invoice on a first run must not keep it once the
		// entitlement is gone, or it is never charged nor counted as failed.
		for _, tt := range []struct {
			name        string
			feats       entitlements.AccountFeatures
			wantMethods []string
		}{
			{
				name: "an existing draft loses a revoked entitlement",
			},
			{
				name: "an existing draft is downgraded to charge_automatically",
				feats: entitlements.AccountFeatures{
					InvoicePaymentMethods:   []string{"card"},
					InvoiceCollectionMethod: entitlements.CollectionMethodChargeAutomatically,
				},
				wantMethods: []string{"card"},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				user, cusID := newCustomer(t)

				stripeService.TestSetMinimumChargeCfg(5_000, nil)
				defer stripeService.TestSetMinimumChargeCfg(0, nil)

				require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, sendInvoice))

				first, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
				require.NoError(t, err)
				require.NotZero(t, first.DueDate)

				require.NoError(t, accounts.SetInvoicePayment(ctx, user.ID, tt.feats))

				second, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
				require.NoError(t, err)
				require.Equal(t, first.ID, second.ID)
				require.Equal(t, tt.wantMethods, methodTypes(second))
				require.Equal(t, stripe.InvoiceCollectionMethodChargeAutomatically, second.CollectionMethod)
				require.Zero(t, second.DueDate)
			})
		}
	})
}
