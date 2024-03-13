// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
)

func TestPurchasePackage(t *testing.T) {
	partner := "partner1"

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
				config.Payments.StripeCoinPayments.StripeFreeTierCouponID = stripe.MockCouponID1
				config.Payments.PackagePlans.Packages = map[string]payments.PackagePlan{
					partner: {Credit: 2000, Price: 1000},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		validCardToken := "testValidCardToken"

		tests := []struct {
			name, cardToken, partner string
			expectedStatus           int
		}{
			{
				// "unknownPartner" does not exist in paymentsconfig.PackagePlans
				"No matching package plan for partner", validCardToken, "unknownPartner",
				http.StatusNotFound,
			},
			{
				"Add credit card fails", stripe.TestPaymentMethodsNewFailure, partner,
				http.StatusInternalServerError,
			},
			{
				"Purchase fails", stripe.MockInvoicesPayFailure, partner,
				http.StatusInternalServerError,
			},
			{
				"Success", validCardToken, partner,
				http.StatusOK,
			},
			{
				"Subsequent request succeeds", validCardToken, partner,
				http.StatusOK,
			},
		}
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName:  "test_name",
					ShortName: "",
					Email:     fmt.Sprintf("test%d@storj.test", i),
					UserAgent: []byte(tt.partner),
				}, 1)
				require.NoError(t, err)

				_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodPost, "payments/purchase-package", strings.NewReader(tt.cardToken))
				require.NoError(t, err)
				require.Equal(t, tt.expectedStatus, status)
			})
		}
	})
}

func TestPackageAvailable(t *testing.T) {
	const pkgPartner = "partner"

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.PackagePlans.Packages = map[string]payments.PackagePlan{
					pkgPartner: {Credit: 2000, Price: 1000},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		tests := []struct {
			name          string
			partner       string
			shouldHavePkg bool
		}{
			{"Partnered", pkgPartner, true},
			{"Unartnered", "", false},
		}

		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName:  "Test User",
					Email:     fmt.Sprintf("test%d@storj.test", i),
					UserAgent: []byte(tt.partner),
				}, 1)
				require.NoError(t, err)

				body, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, "payments/package-available", nil)
				require.NoError(t, err)

				require.Equal(t, http.StatusOK, status)

				var hasPackage bool
				err = json.Unmarshal(body, &hasPackage)
				require.NoError(t, err)
				require.Equal(t, tt.shouldHavePkg, hasPackage)
			})
		}
	})
}
