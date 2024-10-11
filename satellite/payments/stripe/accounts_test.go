// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripe"
)

func TestSignupCouponCodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		log := zaptest.NewLogger(t)

		analyticsService := analytics.NewService(log, analytics.Config{}, "test-satellite")

		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		cache, err := live.OpenCache(ctx, log.Named("cache"), live.Config{StorageBackend: "redis://" + redis.Addr() + "?db=0"})
		require.NoError(t, err)

		projectUsage := accounting.NewService(log, db.ProjectAccounting(), cache, *sat.API.Metainfo.Metabase, 5*time.Minute, 0, 0, 0, -10*time.Second)

		pc := paymentsconfig.Config{
			UsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "10",
				EgressTB:  "45",
				Segment:   "0.0000022",
			},
		}

		prices, err := pc.UsagePrice.ToModel()
		require.NoError(t, err)

		priceOverrides, err := pc.UsagePriceOverrides.ToModels()
		require.NoError(t, err)

		paymentsService, err := stripe.NewService(
			log.Named("payments.stripe:service"),
			stripe.NewStripeMock(
				db.StripeCoinPayments().Customers(),
				db.Console().Users(),
			),
			pc.StripeCoinPayments,
			db.StripeCoinPayments(),
			db.Wallets(),
			db.Billing(),
			db.Console().Projects(),
			db.Console().Users(),
			db.ProjectAccounting(),
			prices,
			priceOverrides,
			pc.PackagePlans.Packages,
			pc.BonusRate,
			nil,
			nil,
			false,
		)
		require.NoError(t, err)

		service, err := console.NewService(
			log.Named("console"),
			db.Console(),
			restkeys.NewService(db.OIDC().OAuthTokens(), planet.Satellites[0].Config.RESTKeys),
			db.ProjectAccounting(),
			projectUsage,
			sat.API.Buckets.Service,
			paymentsService.Accounts(),
			// TODO: do we need a payment deposit wallet here?
			nil,
			db.Billing(),
			analyticsService,
			consoleauth.NewService(consoleauth.Config{
				TokenExpirationTime: 24 * time.Hour,
			}, &consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")}),
			nil,
			nil,
			nil,
			nil,
			"",
			"",
			sat.Config.Metainfo.ProjectLimits.MaxBuckets,
			nodeselection.NewPlacementDefinitions(),
			console.ObjectLockAndVersioningConfig{},
			console.Config{PasswordCost: console.TestPasswordCost, DefaultProjectLimit: 5},
		)

		require.NoError(t, err)

		testCases := []struct {
			name               string
			email              string
			signupPromoCode    string
			expectedCouponType payments.CouponType
		}{
			{"good signup promo code", "test1@mail.test", "promo1", payments.SignupCoupon},
			{"bad signup promo code", "test2@mail.test", "badpromo", payments.NoCoupon},
		}

		for _, tt := range testCases {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				createUser := console.CreateUser{
					FullName:        "Fullname",
					ShortName:       "Shortname",
					Email:           tt.email,
					Password:        "password",
					SignupPromoCode: tt.signupPromoCode,
				}

				regToken, err := service.CreateRegToken(ctx, 1)
				require.NoError(t, err)

				rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret)
				require.NoError(t, err)

				couponType, err := paymentsService.Accounts().Setup(ctx, rootUser.ID, rootUser.Email, rootUser.SignupPromoCode)
				require.NoError(t, err)

				require.Equal(t, tt.expectedCouponType, couponType)
			})
		}
	})
}

func TestUpdateGetPackage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		accounts := planet.Satellites[0].API.Payments.Accounts
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		var packagePlan string
		var purchaseTime time.Time
		packagePlan = "package-plan-1"
		purchaseTime = time.Now()

		require.NoError(t, accounts.UpdatePackage(ctx, userID, &packagePlan, &purchaseTime))
		dbPackagePlan, dbPurchaseTime, err := accounts.GetPackageInfo(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, dbPackagePlan)
		require.NotNil(t, dbPurchaseTime)
		require.Equal(t, packagePlan, *dbPackagePlan)
		require.Zero(t, cmp.Diff(dbPurchaseTime.Truncate(time.Millisecond), purchaseTime.Truncate(time.Millisecond), cmpopts.EquateApproxTime(0)))

		require.NoError(t, accounts.UpdatePackage(ctx, userID, nil, nil))
		dbPackagePlan, dbPurchaseTime, err = accounts.GetPackageInfo(ctx, userID)
		require.NoError(t, err)
		require.Nil(t, dbPackagePlan)
		require.Nil(t, dbPurchaseTime)
	})
}

func TestBillingInformation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		accounts := planet.Satellites[0].API.Payments.Accounts
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		info, err := accounts.GetBillingInformation(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Zero(t, *info)

		var us payments.TaxCountry
		for _, country := range payments.TaxCountries {
			if country.Code == "US" {
				us = country
				break
			}
		}
		var usTax payments.Tax
		for _, tax := range payments.Taxes {
			if tax.CountryCode == "US" {
				usTax = tax
				break
			}
		}
		taxID := payments.TaxID{
			Tax:   usTax,
			Value: "123456789",
		}
		address := payments.BillingAddress{
			Name:       "Some Company",
			Line1:      "Some street",
			Line2:      "Some Apartment",
			City:       "Some city",
			PostalCode: "12345",
			State:      "Some state",
			Country:    us,
		}
		newInfo, err := accounts.SaveBillingAddress(ctx, userID, address)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Empty(t, newInfo.InvoiceReference)

		newInfo, err = accounts.GetBillingInformation(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Empty(t, newInfo.InvoiceReference)

		address.Name = "New Company"
		address.City = "New City"
		newInfo, err = accounts.SaveBillingAddress(ctx, userID, address)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Empty(t, newInfo.InvoiceReference)

		newInfo, err = accounts.AddTaxID(ctx, userID, taxID)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.NotEmpty(t, newInfo.TaxIDs)
		require.NotEmpty(t, newInfo.TaxIDs[0].ID)
		require.Equal(t, taxID.Tax.Code, newInfo.TaxIDs[0].Tax.Code)
		require.Equal(t, taxID.Value, newInfo.TaxIDs[0].Value)
		require.Empty(t, newInfo.InvoiceReference)

		newInfo, err = accounts.RemoveTaxID(ctx, userID, newInfo.TaxIDs[0].ID)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Empty(t, newInfo.InvoiceReference)

		reference := "Some reference"
		newInfo, err = accounts.AddDefaultInvoiceReference(ctx, userID, reference)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Equal(t, reference, newInfo.InvoiceReference)
	})
}
