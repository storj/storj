// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
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

		analyticsService := analytics.NewService(log, analytics.Config{}, "test-satellite", sat.Config.Console.ExternalAddress)

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
		productPrices, err := pc.Products.ToModels()
		require.NoError(t, err)

		minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
		require.NoError(t, err)

		paymentsService, err := stripe.NewService(
			log.Named("payments.stripe:service"),
			stripe.NewStripeMock(
				db.StripeCoinPayments().Customers(),
				db.Console().Users(),
			),
			stripe.ServiceDependencies{
				DB:           db.StripeCoinPayments(),
				WalletsDB:    db.Wallets(),
				BillingDB:    db.Billing(),
				ProjectsDB:   db.Console().Projects(),
				UsersDB:      db.Console().Users(),
				UsageDB:      db.ProjectAccounting(),
				Analytics:    nil,
				Emission:     nil,
				Entitlements: sat.API.Entitlements.Service,
			},
			stripe.ServiceConfig{
				DeleteAccountEnabled:       false,
				DeleteProjectCostThreshold: pc.DeleteProjectCostThreshold,
				EntitlementsEnabled:        sat.Config.Entitlements.Enabled,
			},
			pc.StripeCoinPayments,
			stripe.PricingConfig{
				UsagePrices:         prices,
				UsagePriceOverrides: priceOverrides,
				ProductPriceMap:     productPrices,
				PartnerPlacementMap: pc.PartnersPlacementPriceOverrides.ToMap(),
				PlacementProductMap: pc.PlacementPriceOverrides.ToMap(),
				PackagePlans:        pc.PackagePlans.Packages,
				BonusRate:           pc.BonusRate,
				MinimumChargeAmount: pc.MinimumCharge.Amount,
				MinimumChargeDate:   minimumChargeDate,
			},
		)
		require.NoError(t, err)

		service, err := console.NewService(
			log.Named("console"),
			db.Console(),
			db.Console().RestApiKeys(),
			restkeys.NewService(db.OIDC().OAuthTokens(), sat.Config.Console.RestAPIKeys.DefaultExpiration),
			db.ProjectAccounting(),
			projectUsage,
			sat.API.Buckets.Service,
			db.Attribution(),
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
			nil,
			nil,
			"",
			"",
			sat.Config.Metainfo.ProjectLimits.MaxBuckets,
			false,
			nodeselection.NewPlacementDefinitions(),
			console.ObjectLockAndVersioningConfig{},
			nil,
			pc.MinimumCharge.Amount,
			minimumChargeDate,
			pc.PackagePlans.Packages,
			sat.Config.Entitlements,
			nil,
			pc.PlacementPriceOverrides.ToMap(),
			productPrices,
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
		taxParams := payments.AddTaxParams{Value: "123456789"}
		for _, tax := range payments.Taxes {
			if tax.CountryCode == "US" {
				taxParams.Type = string(tax.Code)
				break
			}
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
		newInfo, err := accounts.SaveBillingAddress(ctx, "", userID, address)
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
		newInfo, err = accounts.SaveBillingAddress(ctx, "", userID, address)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.Empty(t, newInfo.TaxIDs)
		require.Empty(t, newInfo.InvoiceReference)

		newInfo, err = accounts.AddTaxID(ctx, "", userID, taxParams)
		require.NoError(t, err)
		require.Equal(t, address, *newInfo.Address)
		require.NotEmpty(t, newInfo.TaxIDs)
		require.NotEmpty(t, newInfo.TaxIDs[0].ID)
		require.EqualValues(t, taxParams.Type, newInfo.TaxIDs[0].Tax.Code)
		require.Equal(t, taxParams.Value, newInfo.TaxIDs[0].Value)
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

func TestProductCharges(t *testing.T) {
	// Define price models for different products.
	defaultPrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "200",
		EgressTB:  "300",
		Segment:   "400",
	}
	partnerPrice := paymentsconfig.ProjectUsagePrice{
		StorageTB:           "500",
		EgressTB:            "600",
		Segment:             "700",
		EgressDiscountRatio: 0.25, // 25% discount = 75% charged, 25% included.
	}

	// Set up products with prices.
	standardProduct := paymentsconfig.ProductUsagePrice{
		Name:              "Standard Product",
		ProjectUsagePrice: defaultPrice,
	}
	partnerProduct := paymentsconfig.ProductUsagePrice{
		Name:              "Partner Product",
		ProjectUsagePrice: partnerPrice,
		EgressOverageMode: true,
	}

	// Set up product ID mappings.
	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: standardProduct,
		2: partnerProduct,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// Configure placements: 0 (default) -> product 1, 12 (custom) -> product 2.
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");12:annotation("location", "testplacement")`,
				}

				// Set up placement product map (maps placements to product IDs).
				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1,
					12: 2,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		planet.Satellites[0].Accounting.Rollup.Loop.Pause()
		planet.Satellites[0].Accounting.RollupArchive.Loop.Pause()

		accounts := planet.Satellites[0].API.Payments.Accounts
		user := planet.Uplinks[0].Projects[0].Owner
		projectID := planet.Uplinks[0].Projects[0].ID
		db := planet.Satellites[0].DB

		now := time.Now()
		since := now.AddDate(0, -1, 0) // One month ago

		// Set up payment account for the user.
		_, err := accounts.Setup(ctx, user.ID, user.Email, "")
		require.NoError(t, err)

		// Create buckets with different placements to generate different product IDs.
		defaultPlacement := storj.DefaultPlacement
		customPlacement := storj.PlacementConstraint(12)

		bucket1, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "test-bucket-default",
			ProjectID: projectID,
			Placement: defaultPlacement,
		})
		require.NoError(t, err)

		bucket2, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "test-bucket-custom",
			ProjectID: projectID,
			Placement: customPlacement,
		})
		require.NoError(t, err)

		// Create attribution records for both buckets.
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: []byte(bucket1.Name),
			Placement:  &defaultPlacement,
		})
		require.NoError(t, err)

		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: []byte(bucket2.Name),
			Placement:  &customPlacement,
		})
		require.NoError(t, err)

		// Generate usage data for the test period.
		dataVal := int64(1000000000)
		firstDayOfMonth := time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, time.UTC)
		secondDayOfMonth := time.Date(since.Year(), since.Month(), 2, 0, 0, 0, 0, time.UTC)
		lastDayOfMonth := time.Date(since.Year(), since.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

		// Generate usage for both buckets to create charges for different products.
		generateProjectUsage(ctx, t, db, projectID, firstDayOfMonth, secondDayOfMonth, bucket1.Name, dataVal, dataVal, dataVal)
		generateProjectUsage(ctx, t, db, projectID, firstDayOfMonth, secondDayOfMonth, bucket2.Name, dataVal, dataVal, dataVal)

		// Get charges with actual usage data.
		chargesWithUsage, err := accounts.ProductCharges(ctx, user.ID, firstDayOfMonth, lastDayOfMonth)
		require.NoError(t, err)
		require.NotNil(t, chargesWithUsage)

		// Verify we get charges.
		require.NotEmpty(t, chargesWithUsage, "ProductCharges should return charges when there's usage data")
		require.GreaterOrEqual(t, len(chargesWithUsage), 1, "Should have charges for at least one project")

		// Verify we get charges for multiple product IDs.
		foundMultipleProducts := false
		foundProduct1 := false
		foundProduct2 := false
		var product1Charge, product2Charge *payments.ProductCharge

		for returnedProjectID, projectCharges := range chargesWithUsage {
			require.NotEmpty(t, projectCharges, "Should have at least one product charge for project %s", returnedProjectID)

			// Check if this project has multiple product IDs.
			if len(projectCharges) >= 2 {
				foundMultipleProducts = true
				require.Contains(t, projectCharges, int32(1), "Should have product ID 1 (Standard Product)")
				require.Contains(t, projectCharges, int32(2), "Should have product ID 2 (Partner Product)")
			}

			for productID, charge := range projectCharges {
				require.Equal(t, firstDayOfMonth, charge.ProjectUsage.Since)
				require.Equal(t, lastDayOfMonth, charge.ProjectUsage.Before)

				// Verify all usage fields are positive.
				require.Greater(t, charge.ProjectUsage.Storage, float64(0), "Storage usage should be positive with test data")
				require.Greater(t, charge.ProjectUsage.Egress, int64(0), "Egress usage should be positive with test data")
				require.Greater(t, charge.ProjectUsage.SegmentCount, float64(0), "Segment count should be positive with test data")

				// Verify all charges are positive.
				require.Greater(t, charge.StorageMBMonthCents, int64(0), "Storage charges should be positive")
				require.Greater(t, charge.EgressMBCents, int64(0), "Egress charges should be positive")
				require.Greater(t, charge.SegmentMonthCents, int64(0), "Segment charges should be positive")

				// Verify specific product IDs and their properties.
				switch productID {
				case 1:
					foundProduct1 = true
					require.Equal(t, "Standard Product", charge.ProductName, "Product ID 1 should have correct name")
					product1Charge = &charge

					// Verify standard product does not have egress overage mode.
					require.False(t, charge.EgressOverageMode, "Product 1 should not have egress overage mode")
					require.Equal(t, int64(0), charge.ProjectUsage.IncludedEgress, "Included egress should be 0 when egress overage mode is disabled")

				case 2:
					foundProduct2 = true
					require.Equal(t, "Partner Product", charge.ProductName, "Product ID 2 should have correct name")
					product2Charge = &charge

					// Verify egress overage mode fields.
					require.True(t, charge.EgressOverageMode, "Product 2 should have egress overage mode enabled")
					require.Greater(t, charge.ProjectUsage.IncludedEgress, int64(0), "Included egress should be positive for egress overage mode")
					// Verify that included egress is calculated based on storage usage.
					// The formula is: IncludedEgress = Storage / hoursPerMonth * EgressDiscountRatio.
					const hoursPerMonth = 720
					expectedIncluded := int64(math.Round(charge.ProjectUsage.Storage / hoursPerMonth * 0.25))
					require.Equal(t, expectedIncluded, charge.ProjectUsage.IncludedEgress,
						"Included egress should equal storage-based discount (Storage/720 * 0.25)")

				default:
					require.FailNow(t, "Unexpected product ID found", "Got product ID %d, expected only 1 or 2", productID)
				}
			}
		}

		// Verify we found at least one project with multiple products.
		require.True(t, foundMultipleProducts, "Should have found at least one project with multiple product IDs")
		require.True(t, foundProduct1, "Should have found charges for Product ID 1 (Standard Product)")
		require.True(t, foundProduct2, "Should have found charges for Product ID 2 (Partner Product)")

		// Compare pricing between products to verify configuration.
		require.NotNil(t, product1Charge, "Should have captured Product 1 charge for comparison")
		require.NotNil(t, product2Charge, "Should have captured Product 2 charge for comparison")

		// Verify that Product 2 total pricing is higher than Product 1.
		product1Total := product1Charge.StorageMBMonthCents + product1Charge.EgressMBCents + product1Charge.SegmentMonthCents
		product2Total := product2Charge.StorageMBMonthCents + product2Charge.EgressMBCents + product2Charge.SegmentMonthCents
		require.Greater(t, product2Total, product1Total,
			"Product 2 (Partner) total pricing should be higher than Product 1 (Standard)")
	})
}

func TestProductCharges_WithRounding(t *testing.T) {
	productWithRounding := paymentsconfig.ProductUsagePrice{
		Name: "Product with GB Rounding",
		ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
			StorageTB:           "7000",  // $7 per GB-Month (7000 cents per 1000 MB-Month = 7 cents per MB-Month * 1000)
			EgressTB:            "10000", // $10 per GB (10000 cents per 1000 MB = 10 cents per MB * 1000)
			Segment:             "0",
			EgressDiscountRatio: 0.25, // 25% discount = 75% charged, 25% included
		},
		UseGBUnits:        true, // Use GB units instead of MB
		EgressOverageMode: true,
	}

	productWithoutRounding := paymentsconfig.ProductUsagePrice{
		Name: "Product without Rounding",
		ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
			StorageTB: "7000",
			EgressTB:  "10000",
			Segment:   "0",
		},
		UseGBUnits: false, // Use MB units (legacy behavior)
	}

	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: productWithRounding,
		2: productWithoutRounding,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");12:annotation("location", "testplacement")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1, // Default placement uses product 1 (with rounding)
					12: 2, // Custom placement uses product 2 (without rounding)
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides

				config.Payments.StripeCoinPayments.RoundUpInvoiceUsage = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		accounts := sat.API.Payments.Accounts
		user := planet.Uplinks[0].Projects[0].Owner
		projectID := planet.Uplinks[0].Projects[0].ID
		db := sat.DB

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		now := time.Now()
		since := now.AddDate(0, -1, 0)

		_, err := accounts.Setup(ctx, user.ID, user.Email, "")
		require.NoError(t, err)

		defaultPlacement := storj.DefaultPlacement
		customPlacement := storj.PlacementConstraint(12)

		bucket1, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "test-bucket-with-rounding",
			ProjectID: projectID,
			Placement: defaultPlacement,
		})
		require.NoError(t, err)

		bucket2, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "test-bucket-without-rounding",
			ProjectID: projectID,
			Placement: customPlacement,
		})
		require.NoError(t, err)

		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: []byte(bucket1.Name),
			Placement:  &defaultPlacement,
		})
		require.NoError(t, err)

		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: []byte(bucket2.Name),
			Placement:  &customPlacement,
		})
		require.NoError(t, err)

		firstDayOfMonth := time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, time.UTC)
		secondDayOfMonth := time.Date(since.Year(), since.Month(), 2, 0, 0, 0, 0, time.UTC)
		lastDayOfMonth := time.Date(since.Year(), since.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

		// Use small amounts that will be rounded up to 1 GB.
		// For example: 500 MB of egress should be rounded up to 1 GB (1,000,000,000 bytes).
		smallEgressBytes := int64(500_000_000)  // 500 MB in bytes
		smallStorageBytes := int64(100_000_000) // 100 MB in bytes
		smallSegments := int64(100)
		const hoursPerMonth = 720

		generateProjectUsage(ctx, t, db, projectID, firstDayOfMonth, secondDayOfMonth, bucket1.Name, smallEgressBytes, smallStorageBytes, smallSegments)
		generateProjectUsage(ctx, t, db, projectID, firstDayOfMonth, secondDayOfMonth, bucket2.Name, smallEgressBytes, smallStorageBytes, smallSegments)

		charges, err := accounts.ProductCharges(ctx, user.ID, firstDayOfMonth, lastDayOfMonth)
		require.NoError(t, err)
		require.NotEmpty(t, charges)

		var product1Charge, product2Charge *payments.ProductCharge

		for _, projectCharges := range charges {
			for productID, charge := range projectCharges {
				switch productID {
				case 1:
					product1Charge = &charge
				case 2:
					product2Charge = &charge
				}
			}
		}

		require.NotNil(t, product1Charge, "Should have charges for product 1 (with rounding)")
		require.NotNil(t, product2Charge, "Should have charges for product 2 (without rounding)")

		// Test rounding behavior for product 1 (with rounding enabled).
		// Storage: Small usage should be rounded up to 1 GB-Month worth of byte-hours.
		// 1 GB-Month = 1000 MB-Month = 1000 * 1e6 bytes * 720 hours = 720,000,000,000 byte-hours
		expectedRoundedStorageByteHours := float64(1000 * 1e6 * hoursPerMonth) // 1 GB-Month in byte-hours
		require.Equal(t, expectedRoundedStorageByteHours, product1Charge.ProjectUsage.Storage,
			"Product 1 storage should be rounded up to 1 GB-Month (in byte-hours)")

		// Egress: Small usage should be rounded up to 1 GB worth of bytes.
		// 1 GB = 1000 MB = 1000 * 1e6 bytes = 1,000,000,000 bytes
		expectedRoundedEgressBytes := int64(1000 * 1e6) // 1 GB in bytes
		require.Equal(t, expectedRoundedEgressBytes, product1Charge.ProjectUsage.Egress,
			"Product 1 egress should be rounded up to 1 GB (in bytes)")

		// Verify included egress is also rounded for overage mode.
		require.Equal(t, expectedRoundedEgressBytes, product1Charge.ProjectUsage.IncludedEgress,
			"Product 1 should have included egress in overage mode")

		// Test that product 2 (without rounding) uses original values.
		// Without rounding, the values should be much smaller than the rounded values.
		require.Less(t, product2Charge.ProjectUsage.Storage, product1Charge.ProjectUsage.Storage,
			"Product 2 storage should be less than product 1 (no rounding applied)")
		require.Less(t, product2Charge.ProjectUsage.Egress, product1Charge.ProjectUsage.Egress,
			"Product 2 egress should be less than product 1 (no rounding applied)")
	})
}
