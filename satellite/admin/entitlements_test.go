// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	admin "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

func TestAdmin_AccountEntitlements(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// The audit logger only registers its worker when enabled at startup.
				config.Admin.AuditLogger.Enabled = true
				config.Entitlements.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		authInfo := &admin.AuthInfo{Email: "admin@storj.io", Groups: []string{"admin"}}

		newUser := func(t *testing.T, email string) uuid.UUID {
			user, err := sat.AddUser(ctx, console.CreateUser{FullName: "Entitlements Test User", Email: email}, 1)
			require.NoError(t, err)
			return user.ID
		}

		t.Run("get with no entitlement", func(t *testing.T) {
			userID := newUser(t, "noentitlement@storj.test")

			got, apiErr := service.GetUserEntitlements(ctx, userID)
			require.NoError(t, apiErr.Err)
			require.Empty(t, got.InvoicePaymentMethods)
			require.Empty(t, got.InvoiceCollectionMethod)
			require.Zero(t, got.InvoiceDaysUntilDue)
		})

		t.Run("set and read back", func(t *testing.T) {
			userID := newUser(t, "setpayment@storj.test")

			set, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods:   []string{"customer_balance"},
				CollectionMethod: entitlements.CollectionMethodSendInvoice,
				DaysUntilDue:     30,
				Reason:           "customer pays by bank transfer",
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, []string{"customer_balance"}, set.InvoicePaymentMethods)
			require.Equal(t, entitlements.CollectionMethodSendInvoice, set.InvoiceCollectionMethod)
			require.Equal(t, 30, set.InvoiceDaysUntilDue)

			got, apiErr := service.GetUserEntitlements(ctx, userID)
			require.NoError(t, apiErr.Err)
			require.Equal(t, set.InvoicePaymentMethods, got.InvoicePaymentMethods)
			require.Equal(t, set.InvoiceCollectionMethod, got.InvoiceCollectionMethod)
			require.Equal(t, set.InvoiceDaysUntilDue, got.InvoiceDaysUntilDue)
		})

		t.Run("clearing restores the default", func(t *testing.T) {
			userID := newUser(t, "clearpayment@storj.test")

			_, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{"card"},
				Reason:         "pin to card",
			})
			require.NoError(t, apiErr.Err)

			cleared, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				Reason: "back to automatic selection",
			})
			require.NoError(t, apiErr.Err)
			require.Empty(t, cleared.InvoicePaymentMethods)
			require.Empty(t, cleared.InvoiceCollectionMethod)
		})

		t.Run("empty payment method is a bad request", func(t *testing.T) {
			userID := newUser(t, "badmethod@storj.test")

			_, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{"card", " "},
				Reason:         "empty method",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("payment method not in the config is a bad request", func(t *testing.T) {
			userID := newUser(t, "notallowedmethod@storj.test")

			_, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{"card", "paypal"},
				Reason:         "method not allowed",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			got, apiErr := service.GetUserEntitlements(ctx, userID)
			require.NoError(t, apiErr.Err)
			require.Empty(t, got.InvoicePaymentMethods)
		})

		t.Run("payment method is matched after normalization", func(t *testing.T) {
			userID := newUser(t, "normalizedmethod@storj.test")

			set, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{" SEPA_Debit "},
				Reason:         "normalized method",
			})
			require.NoError(t, apiErr.Err)
			require.Equal(t, []string{"sepa_debit"}, set.InvoicePaymentMethods)
		})

		t.Run("inconsistent collection method is a bad request", func(t *testing.T) {
			userID := newUser(t, "baddue@storj.test")

			_, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods:   []string{"customer_balance"},
				CollectionMethod: entitlements.CollectionMethodSendInvoice,
				DaysUntilDue:     -1,
				Reason:           "negative days until due",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			_, apiErr = service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods:   []string{"card"},
				CollectionMethod: entitlements.CollectionMethodChargeAutomatically,
				DaysUntilDue:     30,
				Reason:           "due period on an automatic charge",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("reason is required", func(t *testing.T) {
			userID := newUser(t, "noreason@storj.test")

			_, apiErr := service.SetUserInvoicePayment(ctx, authInfo, userID, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{"card"},
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("unknown user", func(t *testing.T) {
			unknown := testrand.UUID()

			_, apiErr := service.GetUserEntitlements(ctx, unknown)
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusNotFound, apiErr.Status)

			_, apiErr = service.SetUserInvoicePayment(ctx, authInfo, unknown, admin.SetInvoicePaymentRequest{
				PaymentMethods: []string{"card"},
				Reason:         "unknown user",
			})
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})
	})
}
