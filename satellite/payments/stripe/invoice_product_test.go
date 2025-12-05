// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	stripeSDK "github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripe"
)

func TestInvoiceByProduct(t *testing.T) {
	const (
		hoursPerMonth       = 24 * 30
		bytesPerMegabyte    = int64(memory.MB / memory.B)
		byteHoursPerMBMonth = hoursPerMonth * bytesPerMegabyte
	)

	// Define price models.
	var (
		defaultPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB: "1",
			EgressTB:  "2",
			Segment:   "3",
		}
		partnerPrice = paymentsconfig.ProjectUsagePrice{
			StorageTB:           "4",
			EgressTB:            "5",
			Segment:             "6",
			EgressDiscountRatio: 0.5,
		}
		partnerPlacement2Price = paymentsconfig.ProjectUsagePrice{
			StorageTB:           "7",
			EgressTB:            "8",
			Segment:             "9",
			EgressDiscountRatio: 0.5,
		}
	)

	// Set up products with prices.
	standardProduct1 := paymentsconfig.ProductUsagePrice{
		Name:              "Standard Product 1",
		ProjectUsagePrice: defaultPrice,
	}
	standardProduct2 := paymentsconfig.ProductUsagePrice{
		Name:              "Standard Product 2",
		ProjectUsagePrice: defaultPrice,
	}
	partnerProduct1 := paymentsconfig.ProductUsagePrice{
		Name:              "Partner Product 1",
		ProjectUsagePrice: partnerPrice,
	}
	partnerProduct2 := paymentsconfig.ProductUsagePrice{
		Name:              "Partner Product 2",
		ProjectUsagePrice: partnerPlacement2Price,
	}

	// Set up product ID mappings.
	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: standardProduct1,
		2: standardProduct2,
		3: partnerProduct1,
		4: partnerProduct2,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// expected behavior:
				// partner | placement | expected product ID
				// ""      | 0         | 1
				// ""      | 12        | 2
				// "part1" | 0         | 3
				// "part1" | 12        | 4
				// entitle | 0         | 2 // entitled project for any partner
				// entitle | 12        | 3 // entitled project for any partner

				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");12:annotation("location", "testplacement")`,
				}

				// Set up placement product map (maps placements to product IDs)
				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1,
					12: 2,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap

				// Set up partner placement product map (maps partners to placement->product maps)
				var part1Map paymentsconfig.PlacementProductMap
				part1Map.SetMap(map[int]int32{
					0:  3,
					12: 4,
				})
				partnersMap := make(map[string]paymentsconfig.PlacementProductMap)
				partnersMap["part1"] = part1Map

				var partnerPlacementProductMap paymentsconfig.PartnersPlacementProductMap
				partnerPlacementProductMap.SetMap(partnersMap)
				config.Payments.PartnersPlacementPriceOverrides = partnerPlacementProductMap
				config.Payments.Products = productOverrides

				config.Entitlements.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB
		stripeService := planet.Satellites[0].API.Payments.StripeService
		projectEntitlements := planet.Satellites[0].API.Entitlements.Service.Projects()

		period := time.Now().UTC()
		firstDayOfMonth := time.Date(
			period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(
			period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		defaultPlacement := storj.DefaultPlacement
		nonDefaultPlacement := storj.PlacementConstraint(12)

		testCases := []struct {
			name                string
			partner             string
			expectedProductIDs  []int32
			entitlementOverride bool
		}{
			{
				name:               "no partner",
				partner:            "",
				expectedProductIDs: []int32{1, 2},
			},
			{
				name:                "no partner - entitlement override",
				partner:             "entitlement-override",
				expectedProductIDs:  []int32{2, 3},
				entitlementOverride: true,
			},
			{
				name:               "with partner",
				partner:            "part1",
				expectedProductIDs: []int32{3, 4},
			},
			{
				name:                "with partner - entitlement override",
				partner:             "part1",
				expectedProductIDs:  []int32{2, 3},
				entitlementOverride: true,
			},
		}

		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		planet.Satellites[0].Accounting.Rollup.Loop.Pause()
		planet.Satellites[0].Accounting.RollupArchive.Loop.Pause()

		for _, testCase := range testCases {
			project1, err := db.Console().Projects().Insert(
				ctx, &console.Project{ID: testrand.UUID(), Name: "project 1"})
			require.NoError(t, err)
			project2, err := db.Console().Projects().Insert(
				ctx, &console.Project{ID: testrand.UUID(), Name: "project 2"})
			require.NoError(t, err)

			if testCase.entitlementOverride {
				mapping := entitlements.PlacementProductMappings{
					0:  2,
					12: 3,
				}
				err = projectEntitlements.SetPlacementProductMappingsByPublicID(ctx,
					project1.PublicID,
					mapping,
				)
				require.NoError(t, err)

				err = projectEntitlements.SetPlacementProductMappingsByPublicID(ctx,
					project2.PublicID,
					mapping,
				)
				require.NoError(t, err)
			}

			bucket1, err := db.Buckets().CreateBucket(
				ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket1", ProjectID: project1.ID, Placement: defaultPlacement})
			require.NoError(t, err)
			bucket2, err := db.Buckets().CreateBucket(
				ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket2", ProjectID: project1.ID, Placement: nonDefaultPlacement})
			require.NoError(t, err)
			bucket3, err := db.Buckets().CreateBucket(
				ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket3", ProjectID: project2.ID, Placement: defaultPlacement})
			require.NoError(t, err)
			bucket4, err := db.Buckets().CreateBucket(
				ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket4", ProjectID: project2.ID, Placement: nonDefaultPlacement})
			require.NoError(t, err)

			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project1.ID,
				BucketName: []byte(bucket1.Name),
				UserAgent:  []byte(testCase.partner),
				Placement:  &defaultPlacement,
			})
			require.NoError(t, err)
			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project1.ID,
				BucketName: []byte(bucket2.Name),
				UserAgent:  []byte(testCase.partner),
				Placement:  &nonDefaultPlacement,
			})
			require.NoError(t, err)
			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project2.ID,
				BucketName: []byte(bucket3.Name),
				UserAgent:  []byte(testCase.partner),
				Placement:  &defaultPlacement,
			})
			require.NoError(t, err)
			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project2.ID,
				BucketName: []byte(bucket4.Name),
				UserAgent:  []byte(testCase.partner),
				Placement:  &nonDefaultPlacement,
			})
			require.NoError(t, err)

			dataVal := int64(1000000)

			generateProjectUsage(ctx, t, db, project1.ID, firstDayOfMonth, lastDayOfMonth, bucket1.Name, dataVal, dataVal, dataVal)
			generateProjectUsage(ctx, t, db, project1.ID, firstDayOfMonth, lastDayOfMonth, bucket2.Name, dataVal, dataVal, dataVal)
			generateProjectUsage(ctx, t, db, project2.ID, firstDayOfMonth, lastDayOfMonth, bucket3.Name, dataVal, dataVal, dataVal)
			generateProjectUsage(ctx, t, db, project2.ID, firstDayOfMonth, lastDayOfMonth, bucket4.Name, dataVal, dataVal, dataVal)

			productUsages := make(map[int32]accounting.ProjectUsage)
			productInfos := make(map[int32]payments.ProductUsagePriceModel)

			start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

			records := []stripe.ProjectRecord{
				{ProjectID: project1.ID, ProjectPublicID: project1.PublicID, Storage: 1},
				{ProjectID: project2.ID, ProjectPublicID: project2.PublicID, Storage: 1},
			}

			for _, r := range records {
				skipped, err := stripeService.ProcessRecord(ctx, r, productUsages, productInfos, start, end)
				require.NoError(t, err)
				require.False(t, skipped)
			}
			require.Len(t, productUsages, 2)
			require.Len(t, productInfos, 2)

			var gotUsageProductIDs []int32
			for pr, usage := range productUsages {
				gotUsageProductIDs = append(gotUsageProductIDs, pr)

				require.Equal(t, dataVal*2, usage.Egress)
				require.Greater(t, usage.Storage, float64(0))
			}
			require.ElementsMatch(t, testCase.expectedProductIDs, gotUsageProductIDs)

			var gotInfoProductIDs []int32
			for pr := range productInfos {
				gotInfoProductIDs = append(gotInfoProductIDs, pr)
			}
			require.ElementsMatch(t, testCase.expectedProductIDs, gotInfoProductIDs)

			invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)
			require.Len(t, invoiceItems, len(testCase.expectedProductIDs)*3)

			// Verify each product's items.
			for i, productID := range testCase.expectedProductIDs {
				t.Run(fmt.Sprintf("Product %d Items", productID), func(t *testing.T) {
					priceModel := productInfos[productID].ProjectUsagePriceModel
					usage := productUsages[productID]

					discountedEgress := usage.Egress
					discountAmount := int64(math.Round(usage.Storage / hoursPerMonth * priceModel.EgressDiscountRatio))
					discountedEgress -= discountAmount
					if discountedEgress < 0 {
						discountedEgress = 0
					}

					expectedStorageQuantity := int64(math.Round(usage.Storage / float64(byteHoursPerMBMonth)))
					expectedEgressQuantity := int64(math.Round(float64(discountedEgress) / float64(bytesPerMegabyte)))
					expectedSegmentQuantity := int64(math.Round(usage.SegmentCount / hoursPerMonth))

					// Get the items for this product.
					storageItem := invoiceItems[i*3]
					egressItem := invoiceItems[i*3+1]
					segmentItem := invoiceItems[i*3+2]

					// Verify storage line item.
					require.NotNil(t, storageItem)
					require.Contains(t, *storageItem.Description, productInfos[productID].ProductName)
					require.Contains(t, *storageItem.Description, "Storage")
					require.Equal(t, expectedStorageQuantity, *storageItem.Quantity, "Storage quantity mismatch for product %d", productID)
					storagePrice, _ := priceModel.StorageMBMonthCents.Float64()
					require.Equal(t, storagePrice, *storageItem.UnitAmountDecimal, "Storage price mismatch for product %d", productID)

					// Verify egress line item.
					require.NotNil(t, egressItem)
					require.Contains(t, *egressItem.Description, productInfos[productID].ProductName)
					require.Contains(t, *egressItem.Description, "Egress")
					require.Equal(t, expectedEgressQuantity, *egressItem.Quantity, "Egress quantity mismatch for product %d", productID)
					egressPrice, _ := priceModel.EgressMBCents.Float64()
					require.Equal(t, egressPrice, *egressItem.UnitAmountDecimal, "Egress price mismatch for product %d", productID)

					// Verify segment line item.
					require.NotNil(t, segmentItem)
					require.Contains(t, *segmentItem.Description, productInfos[productID].ProductName)
					require.Contains(t, *segmentItem.Description, "Segment")
					require.Equal(t, expectedSegmentQuantity, *segmentItem.Quantity, "Segment quantity mismatch for product %d", productID)
					segmentPrice, _ := priceModel.SegmentMonthCents.Float64()
					require.Equal(t, segmentPrice, *segmentItem.UnitAmountDecimal, "Segment price mismatch for product %d", productID)
				})
			}
		}
	})
}

func TestInvoiceByProduct_withPlaceholderItems(t *testing.T) {
	// Define price models.
	defaultPrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "1",
		EgressTB:  "2",
		Segment:   "3",
	}

	// Set up products with different placeholder fee configurations.
	productWithBothFees := paymentsconfig.ProductUsagePrice{
		Name:                "Product Both Fees",
		SmallObjectFee:      "0.10",
		MinimumRetentionFee: "0.05",
		ProjectUsagePrice:   defaultPrice,
	}
	productWithNoFees := paymentsconfig.ProductUsagePrice{
		Name:                "Product No Fees",
		SmallObjectFee:      "0",
		MinimumRetentionFee: "0",
		ProjectUsagePrice:   defaultPrice,
	}
	productWithSmallObjectFee := paymentsconfig.ProductUsagePrice{
		Name:                "Product Small Object Fee",
		SmallObjectFee:      "0.12",
		MinimumRetentionFee: "0",
		ProjectUsagePrice:   defaultPrice,
	}
	productWithRetentionFee := paymentsconfig.ProductUsagePrice{
		Name:                "Product Retention Fee",
		SmallObjectFee:      "0",
		MinimumRetentionFee: "0.07",
		ProjectUsagePrice:   defaultPrice,
	}

	// Set up product ID mappings.
	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: productWithBothFees,
		2: productWithNoFees,
		3: productWithSmallObjectFee,
		4: productWithRetentionFee,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// Simple placement configuration for testing placeholder items:
				// placement 0 -> product 1 (both fees)
				// placement 10 -> product 2 (no fees)
				// placement 20 -> product 3 (small object fee only)
				// placement 30 -> product 4 (retention fee only)

				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");10:annotation("location", "test1");20:annotation("location", "test2");30:annotation("location", "test3")`,
				}

				// Set up placement product map
				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1, // Both fees
					10: 2, // No fees
					20: 3, // Small object fee only
					30: 4, // Retention fee only
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB
		stripeService := planet.Satellites[0].API.Payments.StripeService

		period := time.Now().UTC()
		firstDayOfMonth := time.Date(
			period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(
			period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		placement0 := storj.DefaultPlacement         // Both fees
		placement10 := storj.PlacementConstraint(10) // No fees
		placement20 := storj.PlacementConstraint(20) // Small object fee only
		placement30 := storj.PlacementConstraint(30) // Retention fee only

		expectedInvoiceItemsPerProduct := map[int32]int{
			1: 5, // storage, egress, segment, small object fee, minimum retention fee
			2: 3, // storage, egress, segment (no fees)
			3: 4, // storage, egress, segment, small object fee only
			4: 4, // storage, egress, segment, minimum retention fee only
		}

		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		planet.Satellites[0].Accounting.Rollup.Loop.Pause()
		planet.Satellites[0].Accounting.RollupArchive.Loop.Pause()

		// Create one project with buckets for each placement to test different fee combinations.
		project, err := db.Console().Projects().Insert(
			ctx, &console.Project{ID: testrand.UUID(), Name: "test project"})
		require.NoError(t, err)

		bucket1, err := db.Buckets().CreateBucket(
			ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket1", ProjectID: project.ID, Placement: placement0})
		require.NoError(t, err)
		bucket2, err := db.Buckets().CreateBucket(
			ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket2", ProjectID: project.ID, Placement: placement10})
		require.NoError(t, err)
		bucket3, err := db.Buckets().CreateBucket(
			ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket3", ProjectID: project.ID, Placement: placement20})
		require.NoError(t, err)
		bucket4, err := db.Buckets().CreateBucket(
			ctx, buckets.Bucket{ID: testrand.UUID(), Name: "bucket4", ProjectID: project.ID, Placement: placement30})
		require.NoError(t, err)

		// Create attributions for each bucket (no partner needed)
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket1.Name),
			Placement:  &placement0,
		})
		require.NoError(t, err)
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket2.Name),
			Placement:  &placement10,
		})
		require.NoError(t, err)
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket3.Name),
			Placement:  &placement20,
		})
		require.NoError(t, err)
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket4.Name),
			Placement:  &placement30,
		})
		require.NoError(t, err)

		dataVal := int64(1000000)

		// Generate usage for each bucket.
		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket1.Name, dataVal, dataVal, dataVal)
		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket2.Name, dataVal, dataVal, dataVal)
		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket3.Name, dataVal, dataVal, dataVal)
		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket4.Name, dataVal, dataVal, dataVal)

		productUsages := make(map[int32]accounting.ProjectUsage)
		productInfos := make(map[int32]payments.ProductUsagePriceModel)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		records := []stripe.ProjectRecord{
			{ProjectID: project.ID, Storage: 1},
		}

		for _, r := range records {
			skipped, err := stripeService.ProcessRecord(ctx, r, productUsages, productInfos, start, end)
			require.NoError(t, err)
			require.False(t, skipped)
		}
		require.Len(t, productUsages, 4)
		require.Len(t, productInfos, 4)

		expectedProductIDs := []int32{1, 2, 3, 4}
		var gotUsageProductIDs []int32
		for pr, usage := range productUsages {
			gotUsageProductIDs = append(gotUsageProductIDs, pr)
			require.Equal(t, dataVal, usage.Egress)
			require.Greater(t, usage.Storage, float64(0))
		}
		require.ElementsMatch(t, expectedProductIDs, gotUsageProductIDs)

		var gotInfoProductIDs []int32
		for pr := range productInfos {
			gotInfoProductIDs = append(gotInfoProductIDs, pr)
		}
		require.ElementsMatch(t, expectedProductIDs, gotInfoProductIDs)

		invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

		// Calculate expected total invoice items.
		expectedTotalItems := 0
		for _, productID := range expectedProductIDs {
			expectedTotalItems += expectedInvoiceItemsPerProduct[productID]
		}
		require.Len(t, invoiceItems, expectedTotalItems)

		// Verify placeholder fees are included in invoice items.
		itemIndex := 0
		for _, productID := range expectedProductIDs {
			t.Run(fmt.Sprintf("Product %d Items", productID), func(t *testing.T) {
				productInfo := productInfos[productID]
				currentIndex := itemIndex + 3 // Skip storage, egress, segment items

				// Verify small object fee item if present
				if !productInfo.SmallObjectFeeCents.IsZero() {
					smallObjectFeeItem := invoiceItems[currentIndex]
					require.NotNil(t, smallObjectFeeItem)
					require.Contains(t, *smallObjectFeeItem.Description, productInfo.ProductName)
					require.Contains(t, *smallObjectFeeItem.Description, "Minimum Object Size Remainder")
					require.Equal(t, int64(0), *smallObjectFeeItem.Quantity)
					smallObjectFeePrice, _ := productInfo.SmallObjectFeeCents.Float64()
					require.Equal(t, smallObjectFeePrice, *smallObjectFeeItem.UnitAmountDecimal)
					currentIndex++
				}

				// Verify minimum retention fee item if present
				if !productInfo.MinimumRetentionFeeCents.IsZero() {
					minimumRetentionFeeItem := invoiceItems[currentIndex]
					require.NotNil(t, minimumRetentionFeeItem)
					require.Contains(t, *minimumRetentionFeeItem.Description, productInfo.ProductName)
					require.Contains(t, *minimumRetentionFeeItem.Description, "Minimum Storage Retention Remainder")
					require.Equal(t, int64(0), *minimumRetentionFeeItem.Quantity)
					minimumRetentionFeePrice, _ := productInfo.MinimumRetentionFeeCents.Float64()
					require.Equal(t, minimumRetentionFeePrice, *minimumRetentionFeeItem.UnitAmountDecimal)
				}
			})
			itemIndex += expectedInvoiceItemsPerProduct[productID]
		}
	})
}

func TestInvoiceByProduct_WithAndWithoutSegmentFee(t *testing.T) {
	// Test that products with zero segment fees do not generate segment invoice items.
	priceWithSegments := paymentsconfig.ProjectUsagePrice{
		StorageTB: "4",
		EgressTB:  "7",
		Segment:   "3", // Has segment fee
	}
	priceWithoutSegments := paymentsconfig.ProjectUsagePrice{
		StorageTB: "4",
		EgressTB:  "7",
		Segment:   "0", // Zero segment fee
	}

	productWithSegments := paymentsconfig.ProductUsagePrice{
		ID:                5,
		Name:              "Product With Segments",
		ProjectUsagePrice: priceWithSegments,
	}
	productWithoutSegments := paymentsconfig.ProductUsagePrice{
		ID:                6,
		Name:              "Product Without Segments",
		ProjectUsagePrice: priceWithoutSegments,
	}

	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		5: productWithSegments,
		6: productWithoutSegments,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `25:annotation("location", "withsegments");26:annotation("location", "withoutsegments")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					25: 5, // Product with segments
					26: 6, // Product without segments
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		stripeService := sat.API.Payments.StripeService

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		period := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)
		firstDayOfMonth := time.Date(period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		withSegmentsPlacement := storj.PlacementConstraint(25)
		withoutSegmentsPlacement := storj.PlacementConstraint(26)

		project, err := db.Console().Projects().Insert(ctx, &console.Project{ID: testrand.UUID(), Name: "segment fee test project"})
		require.NoError(t, err)

		bucket1, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "bucket-with-segments",
			ProjectID: project.ID,
			Placement: withSegmentsPlacement,
		})
		require.NoError(t, err)
		bucket2, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "bucket-without-segments",
			ProjectID: project.ID,
			Placement: withoutSegmentsPlacement,
		})
		require.NoError(t, err)

		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket1.Name),
			Placement:  &withSegmentsPlacement,
		})
		require.NoError(t, err)
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket2.Name),
			Placement:  &withoutSegmentsPlacement,
		})
		require.NoError(t, err)

		egressBytes := int64(1000 * memory.MB)
		storageBytes := int64(500 * memory.MB)
		segmentCount := int64(1000)

		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket1.Name, egressBytes, storageBytes, segmentCount)
		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket2.Name, egressBytes, storageBytes, segmentCount)

		productUsages := make(map[int32]accounting.ProjectUsage)
		productInfos := make(map[int32]payments.ProductUsagePriceModel)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		record := stripe.ProjectRecord{ProjectID: project.ID, Storage: 1}
		_, err = stripeService.ProcessRecord(ctx, record, productUsages, productInfos, start, end)
		require.NoError(t, err)

		invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

		// Verify that we have:
		// - Product with segments: 3 items (storage + egress + segment)
		// - Product without segments: 2 items (storage + egress only)
		// Total: 5 items
		require.Len(t, invoiceItems, 5, "Expected 5 invoice items total")

		var withSegmentsStorageItem, withSegmentsEgressItem, withSegmentsSegmentItem *stripeSDK.InvoiceItemParams
		var withoutSegmentsStorageItem, withoutSegmentsEgressItem *stripeSDK.InvoiceItemParams
		var foundWithoutSegmentsSegmentItem bool

		for _, item := range invoiceItems {
			desc := *item.Description
			if strings.Contains(desc, "Product With Segments") {
				if strings.Contains(desc, "Storage") {
					withSegmentsStorageItem = item
				} else if strings.Contains(desc, "Egress") {
					withSegmentsEgressItem = item
				} else if strings.Contains(desc, "Segment") {
					withSegmentsSegmentItem = item
				}
			} else if strings.Contains(desc, "Product Without Segments") {
				if strings.Contains(desc, "Storage") {
					withoutSegmentsStorageItem = item
				} else if strings.Contains(desc, "Egress") {
					withoutSegmentsEgressItem = item
				} else if strings.Contains(desc, "Segment") {
					foundWithoutSegmentsSegmentItem = true
				}
			}
		}

		// Verify product with segments has all 3 items.
		require.NotNil(t, withSegmentsStorageItem, "Product with segments should have storage item")
		require.NotNil(t, withSegmentsEgressItem, "Product with segments should have egress item")
		require.NotNil(t, withSegmentsSegmentItem, "Product with segments should have segment item")

		// Verify product without segments has only storage and egress items.
		require.NotNil(t, withoutSegmentsStorageItem, "Product without segments should have storage item")
		require.NotNil(t, withoutSegmentsEgressItem, "Product without segments should have egress item")
		require.False(t, foundWithoutSegmentsSegmentItem, "Product without segments should NOT have segment item when segment fee is zero")
	})
}

func TestEgressOverageFunctionality(t *testing.T) {
	defaultPlacement := storj.DefaultPlacement
	includedEgressDesc := "Included Egress"
	additionalEgressDesc := "Additional Egress"
	standardEgressDesc := "Egress Bandwidth"

	price := paymentsconfig.ProjectUsagePrice{
		StorageTB: "1",
		EgressTB:  "2",
		Segment:   "3",
	}

	priceWithDiscount := paymentsconfig.ProductUsagePrice{
		Name:              "Global",
		EgressOverageMode: false,
		ProjectUsagePrice: price,
	}
	priceWithDiscount.EgressDiscountRatio = 2.0 // 2X multiplier.

	testCases := []struct {
		name                   string
		productConfig          paymentsconfig.ProductUsagePrice
		egressUsage            int64
		storageUsage           int64
		expectIncludedEgress   bool
		expectAdditionalEgress bool
		expectStandardEgress   bool
		expectedDiscountRatio  string
	}{
		{
			name: "Overage mode with included and additional egress",
			productConfig: func() paymentsconfig.ProductUsagePrice {
				config := priceWithDiscount
				config.EgressOverageMode = true
				return config
			}(),
			egressUsage:            10000000, // 10MB.
			storageUsage:           1000000,  // 1MB storage -> 2MB included egress (2X ratio).
			expectIncludedEgress:   true,
			expectAdditionalEgress: true,
			expectStandardEgress:   false,
			expectedDiscountRatio:  "2X",
		},
		{
			name: "Overage mode with only included egress (no overage)",
			productConfig: func() paymentsconfig.ProductUsagePrice {
				config := priceWithDiscount
				config.EgressOverageMode = true
				return config
			}(),
			egressUsage:            1000000, // 1MB egress.
			storageUsage:           1000000, // 1MB storage -> 2MB included egress (2X ratio).
			expectIncludedEgress:   true,
			expectAdditionalEgress: false,
			expectStandardEgress:   false,
			expectedDiscountRatio:  "2X",
		},
		{
			name: "Overage mode with zero egress and storage",
			productConfig: func() paymentsconfig.ProductUsagePrice {
				config := priceWithDiscount
				config.EgressOverageMode = true
				return config
			}(),
			egressUsage:            0,
			storageUsage:           0,
			expectIncludedEgress:   false,
			expectAdditionalEgress: false,
			expectStandardEgress:   false,
		},
		{
			name: "Standard mode - should use traditional egress item",
			productConfig: func() paymentsconfig.ProductUsagePrice {
				config := priceWithDiscount
				config.EgressOverageMode = false
				return config
			}(),
			egressUsage:            10000000,
			storageUsage:           1000000,
			expectIncludedEgress:   false,
			expectAdditionalEgress: false,
			expectStandardEgress:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var productOverrides paymentsconfig.ProductPriceOverrides
			productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
				1: tc.productConfig,
			})

			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
				Reconfigure: testplanet.Reconfigure{
					Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
						config.Placement = nodeselection.ConfigurablePlacementRule{
							PlacementRules: `0:annotation("location", "test")`,
						}

						var placementProductMap paymentsconfig.PlacementProductMap
						placementProductMap.SetMap(map[int]int32{0: 1})
						config.Payments.PlacementPriceOverrides = placementProductMap
						config.Payments.Products = productOverrides
					},
				},
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
				sat := planet.Satellites[0]
				db := sat.DB
				stripeService := sat.API.Payments.StripeService

				sat.Accounting.Tally.Loop.Pause()
				sat.Accounting.Rollup.Loop.Pause()
				sat.Accounting.RollupArchive.Loop.Pause()

				// Create test data.
				project, err := db.Console().Projects().Insert(ctx, &console.Project{ID: testrand.UUID(), Name: "test project"})
				require.NoError(t, err)
				bucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{ID: testrand.UUID(), Name: "test-bucket", ProjectID: project.ID, Placement: storj.DefaultPlacement})
				require.NoError(t, err)

				_, err = db.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(bucket.Name),
					Placement:  &defaultPlacement,
				})
				require.NoError(t, err)

				period := time.Now().UTC()
				firstDayOfMonth := time.Date(period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
				lastDayOfMonth := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

				generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket.Name, tc.egressUsage, tc.storageUsage, 1000000)

				// Process through stripe service.
				productUsages := make(map[int32]accounting.ProjectUsage)
				productInfos := make(map[int32]payments.ProductUsagePriceModel)

				start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
				end := start.AddDate(0, 1, 0)

				record := stripe.ProjectRecord{ProjectID: project.ID, Storage: 1}
				_, err = stripeService.ProcessRecord(ctx, record, productUsages, productInfos, start, end)
				require.NoError(t, err)

				// Verify the product configuration.
				require.Len(t, productInfos, 1)
				productInfo := productInfos[1]
				require.Equal(t, tc.productConfig.EgressOverageMode, productInfo.EgressOverageMode)

				// Generate invoice items.
				invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

				// Check invoice items.
				var foundIncludedEgress, foundAdditionalEgress, foundStandardEgress bool
				var includedEgressItem, additionalEgressItem, standardEgressItem *stripeSDK.InvoiceItemParams

				for _, item := range invoiceItems {
					desc := *item.Description
					require.NotNil(t, desc)

					if strings.Contains(desc, includedEgressDesc) {
						foundIncludedEgress = true
						includedEgressItem = item
					} else if strings.Contains(desc, additionalEgressDesc) {
						foundAdditionalEgress = true
						additionalEgressItem = item
					} else if strings.Contains(desc, standardEgressDesc) {
						foundStandardEgress = true
						standardEgressItem = item
					}
				}

				// Validate expectations.
				require.Equal(t, tc.expectIncludedEgress, foundIncludedEgress)
				require.Equal(t, tc.expectAdditionalEgress, foundAdditionalEgress)
				require.Equal(t, tc.expectStandardEgress, foundStandardEgress)

				// Validate included egress item details.
				if tc.expectIncludedEgress {
					require.NotNil(t, includedEgressItem)
					require.Contains(t, *includedEgressItem.Description, tc.productConfig.Name)
					require.Contains(t, *includedEgressItem.Description, tc.expectedDiscountRatio)
					require.Equal(t, float64(0), *includedEgressItem.UnitAmountDecimal) // $0 price.
					require.Greater(t, *includedEgressItem.Quantity, int64(0))          // Should have quantity > 0.
				}

				// Validate additional egress item details.
				if tc.expectAdditionalEgress {
					require.NotNil(t, additionalEgressItem)
					require.Contains(t, *additionalEgressItem.Description, tc.productConfig.Name)
					require.Contains(t, *additionalEgressItem.Description, additionalEgressDesc)
					require.Greater(t, *additionalEgressItem.UnitAmountDecimal, float64(0)) // Should have price > $0.
					require.Greater(t, *additionalEgressItem.Quantity, int64(0))            // Should have quantity > 0.
				}

				// Validate standard egress item details.
				if tc.expectStandardEgress {
					require.NotNil(t, standardEgressItem)
					require.Contains(t, *standardEgressItem.Description, tc.productConfig.Name)
					require.Contains(t, *standardEgressItem.Description, standardEgressDesc)
				}
			})
		})
	}
}

func TestUnitsAdjustment_WithRoundingUp(t *testing.T) {
	basePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "4",
		EgressTB:  "7",
		Segment:   "1",
	}

	// Legacy product without GB units.
	legacyProduct := paymentsconfig.ProductUsagePrice{
		ID:                1,
		Name:              "Legacy Product",
		UseGBUnits:        false, // Use MB units (legacy behavior).
		ProjectUsagePrice: basePrice,
	}
	// New product with GB units.
	newProduct := paymentsconfig.ProductUsagePrice{
		ID:                2,
		Name:              "New Product",
		UseGBUnits:        true, // Use GB units instead of MB.
		ProjectUsagePrice: basePrice,
	}

	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: legacyProduct,
		2: newProduct,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");10:annotation("location", "newproduct")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1,
					10: 2,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
				config.Payments.StripeCoinPayments.RoundUpInvoiceUsage = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		stripeService := sat.API.Payments.StripeService

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		period := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC) // Fixed date for consistent testing.
		firstDayOfMonth := time.Date(period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		legacyPlacement := storj.DefaultPlacement
		newPlacement := storj.PlacementConstraint(10)

		testCases := []struct {
			name                    string
			egressBytes             int64
			storageBytes            int64
			expectedNewEgressGB     int64
			expectedNewStorageGB    int64
			expectedLegacyEgressMB  int64
			expectedLegacyStorageMB int64
		}{
			{
				name:                    "1 byte should round up to 1 GB",
				egressBytes:             1,
				storageBytes:            1,
				expectedNewEgressGB:     1,
				expectedNewStorageGB:    1,
				expectedLegacyEgressMB:  0, // Less than 1 MB
				expectedLegacyStorageMB: 0,
			},
			{
				name:                    "500 MB should round up to 1 GB",
				egressBytes:             int64(500 * memory.MB),
				storageBytes:            int64(100 * memory.MB),
				expectedNewEgressGB:     1,
				expectedNewStorageGB:    1,
				expectedLegacyEgressMB:  500,
				expectedLegacyStorageMB: 100,
			},
			{
				name:                    "2005 MB should round up to 3 GB",
				egressBytes:             int64(2005 * memory.MB),
				storageBytes:            int64(2005 * memory.MB),
				expectedNewEgressGB:     3, // 2005 MB egress -> 2.005 GB -> rounds up to 3 GB
				expectedNewStorageGB:    3, // Storage byte-hours converted to GB-Month -> ~2.002 GB (tallies time inconsistencies) -> rounds up to 3 GB
				expectedLegacyEgressMB:  2005,
				expectedLegacyStorageMB: 2002,
			},
			{
				name:                    "Exactly 2 GB should stay 2 GB",
				egressBytes:             int64(2 * memory.GB),
				storageBytes:            int64(2 * memory.GB),
				expectedNewEgressGB:     2, // 2000 MB egress -> exactly 2 GB
				expectedNewStorageGB:    2, // ~1997 MB storage (tallies time inconsistencies) -> rounds up to 2 GB
				expectedLegacyEgressMB:  2000,
				expectedLegacyStorageMB: 1997,
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				project, err := db.Console().Projects().Insert(ctx, &console.Project{ID: testrand.UUID(), Name: fmt.Sprintf("test project %d", i)})
				require.NoError(t, err)

				legacyBucketName := fmt.Sprintf("legacy-bucket-%d", i)
				newBucketName := fmt.Sprintf("new-bucket-%d", i)

				legacyBucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      legacyBucketName,
					ProjectID: project.ID,
					Placement: legacyPlacement,
				})
				require.NoError(t, err)

				newBucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      newBucketName,
					ProjectID: project.ID,
					Placement: newPlacement,
				})
				require.NoError(t, err)

				_, err = db.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(legacyBucket.Name),
					Placement:  &legacyPlacement,
				})
				require.NoError(t, err)

				_, err = db.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(newBucket.Name),
					Placement:  &newPlacement,
				})
				require.NoError(t, err)

				productUsages := make(map[int32]accounting.ProjectUsage)
				productInfos := make(map[int32]payments.ProductUsagePriceModel)

				generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, legacyBucket.Name, tc.egressBytes, tc.storageBytes, 1000)
				generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, newBucket.Name, tc.egressBytes, tc.storageBytes, 1000)

				start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
				end := start.AddDate(0, 1, 0)

				record := stripe.ProjectRecord{ProjectID: project.ID, Storage: 1}
				_, err = stripeService.ProcessRecord(ctx, record, productUsages, productInfos, start, end)
				require.NoError(t, err)

				invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

				var legacyStorageItem, legacyEgressItem, legacySegmentItem, newStorageItem, newEgressItem, newSegmentItem *stripeSDK.InvoiceItemParams

				for _, item := range invoiceItems {
					desc := *item.Description
					if strings.Contains(desc, "Legacy Product") {
						if strings.Contains(desc, "Storage") {
							legacyStorageItem = item
						} else if strings.Contains(desc, "Egress") {
							legacyEgressItem = item
						} else if strings.Contains(desc, "Segment") {
							legacySegmentItem = item
						}
					} else if strings.Contains(desc, "New Product") {
						if strings.Contains(desc, "Storage") {
							newStorageItem = item
						} else if strings.Contains(desc, "Egress") {
							newEgressItem = item
						} else if strings.Contains(desc, "Segment") {
							newSegmentItem = item
						}
					}
				}

				// Verify legacy product uses MB units.
				require.NotNil(t, legacyStorageItem)
				require.Contains(t, *legacyStorageItem.Description, "MB-Month")
				require.NotNil(t, legacyEgressItem)
				require.Contains(t, *legacyEgressItem.Description, "MB")

				// Verify new product uses GB units with rounding UP.
				require.NotNil(t, newStorageItem)
				require.Contains(t, *newStorageItem.Description, "GB-Month")
				require.Equal(t, tc.expectedNewStorageGB, *newStorageItem.Quantity, "Storage quantity mismatch")

				require.NotNil(t, newEgressItem)
				require.Contains(t, *newEgressItem.Description, "GB")
				require.Equal(t, tc.expectedNewEgressGB, *newEgressItem.Quantity, "Egress quantity mismatch")

				// Verify legacy quantities (MB units, no rounding).
				if tc.expectedLegacyStorageMB > 0 {
					require.Equal(t, tc.expectedLegacyStorageMB, *legacyStorageItem.Quantity, "Legacy storage quantity mismatch")
				}
				if tc.expectedLegacyEgressMB > 0 {
					require.Equal(t, tc.expectedLegacyEgressMB, *legacyEgressItem.Quantity, "Legacy egress quantity mismatch")
				}

				// Verify pricing is consistent across all test cases.
				// $4/TB = $0.004/GB = $0.000004/MB = 0.0004 cents/MB
				legacyStoragePrice := *legacyStorageItem.UnitAmountDecimal
				require.Equal(t, 0.0004, legacyStoragePrice) // 0.0004 cents per MB-Month

				// 0.0004 cents/MB * 1000 = 0.4 cents/GB
				newStoragePrice := *newStorageItem.UnitAmountDecimal
				require.Equal(t, 0.4, newStoragePrice) // 0.4 cents per GB-Month

				// $7/TB = $0.007/GB = $0.000007/MB = 0.0007 cents/MB
				legacyEgressPrice := *legacyEgressItem.UnitAmountDecimal
				require.Equal(t, 0.0007, legacyEgressPrice) // 0.0007 cents per MB

				// 0.0007 cents/MB * 1000 = 0.7 cents/GB
				newEgressPrice := *newEgressItem.UnitAmountDecimal
				require.Equal(t, 0.7, newEgressPrice) // 0.7 cents per GB

				// Verify segment line items are NOT affected by units adjustment.
				require.NotNil(t, legacySegmentItem)
				require.Contains(t, *legacySegmentItem.Description, "Segment-Month")
				require.Equal(t, 100.0, *legacySegmentItem.UnitAmountDecimal)
				require.NotNil(t, newSegmentItem)
				require.Contains(t, *newSegmentItem.Description, "Segment-Month")
				require.Equal(t, 100.0, *newSegmentItem.UnitAmountDecimal)

				// Verify segment quantities are identical (no rounding applied).
				require.Equal(t, *legacySegmentItem.Quantity, *newSegmentItem.Quantity, "Segment quantities should be identical")
			})
		}
	})
}

func TestUnitsAdjustment_WithoutRoundingUp(t *testing.T) {
	basePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "4",
		EgressTB:  "7",
		Segment:   "1",
	}

	// Legacy product without GB units.
	legacyProduct := paymentsconfig.ProductUsagePrice{
		ID:                1,
		Name:              "Legacy Product",
		UseGBUnits:        false, // Use MB units (legacy behavior).
		ProjectUsagePrice: basePrice,
	}
	// New product with GB units.
	newProduct := paymentsconfig.ProductUsagePrice{
		ID:                2,
		Name:              "New Product",
		UseGBUnits:        true, // Use GB units instead of MB.
		ProjectUsagePrice: basePrice,
	}

	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		1: legacyProduct,
		2: newProduct,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");10:annotation("location", "newproduct")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0:  1,
					10: 2,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
				config.Payments.StripeCoinPayments.RoundUpInvoiceUsage = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		stripeService := sat.API.Payments.StripeService

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		period := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC) // Fixed date for consistent testing.
		firstDayOfMonth := time.Date(period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		legacyPlacement := storj.DefaultPlacement
		newPlacement := storj.PlacementConstraint(10)

		testCases := []struct {
			name                    string
			egressBytes             int64
			storageBytes            int64
			expectedNewEgressGB     int64
			expectedNewStorageGB    int64
			expectedLegacyEgressMB  int64
			expectedLegacyStorageMB int64
		}{
			{
				name:                    "1 byte rounds to 0 GB (not 1)",
				egressBytes:             1,
				storageBytes:            1,
				expectedNewEgressGB:     0, // Rounds to 0 with round-to-nearest
				expectedNewStorageGB:    0, // Rounds to 0 with round-to-nearest
				expectedLegacyEgressMB:  0, // Less than 1 MB
				expectedLegacyStorageMB: 0,
			},
			{
				name:                    "500 MB (0.5 GB) rounds to 1 GB with Round(0)",
				egressBytes:             int64(500 * memory.MB),
				storageBytes:            int64(100 * memory.MB),
				expectedNewEgressGB:     1, // 0.5 rounds to 1 with Round(0) (rounds 0.5 up)
				expectedNewStorageGB:    0, // 0.1 rounds to 0 with round-to-nearest
				expectedLegacyEgressMB:  500,
				expectedLegacyStorageMB: 100,
			},
			{
				name:                    "2005 MB (2.005 GB) rounds to 2 GB (not 3)",
				egressBytes:             int64(2005 * memory.MB),
				storageBytes:            int64(2005 * memory.MB),
				expectedNewEgressGB:     2, // 2.005 rounds to 2 with round-to-nearest
				expectedNewStorageGB:    2, // ~2.002 rounds to 2 with round-to-nearest
				expectedLegacyEgressMB:  2005,
				expectedLegacyStorageMB: 2002,
			},
			{
				name:                    "Exactly 2 GB stays 2 GB",
				egressBytes:             int64(2 * memory.GB),
				storageBytes:            int64(2 * memory.GB),
				expectedNewEgressGB:     2, // 2000 MB egress -> exactly 2 GB
				expectedNewStorageGB:    2, // ~1997 MB storage -> rounds to 2 GB
				expectedLegacyEgressMB:  2000,
				expectedLegacyStorageMB: 1997,
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				project, err := db.Console().Projects().Insert(ctx, &console.Project{ID: testrand.UUID(), Name: fmt.Sprintf("test project %d", i)})
				require.NoError(t, err)

				legacyBucketName := fmt.Sprintf("legacy-bucket-%d", i)
				newBucketName := fmt.Sprintf("new-bucket-%d", i)

				legacyBucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      legacyBucketName,
					ProjectID: project.ID,
					Placement: legacyPlacement,
				})
				require.NoError(t, err)

				newBucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      newBucketName,
					ProjectID: project.ID,
					Placement: newPlacement,
				})
				require.NoError(t, err)

				_, err = db.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(legacyBucket.Name),
					Placement:  &legacyPlacement,
				})
				require.NoError(t, err)

				_, err = db.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(newBucket.Name),
					Placement:  &newPlacement,
				})
				require.NoError(t, err)

				productUsages := make(map[int32]accounting.ProjectUsage)
				productInfos := make(map[int32]payments.ProductUsagePriceModel)

				generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, legacyBucket.Name, tc.egressBytes, tc.storageBytes, 1000)
				generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, newBucket.Name, tc.egressBytes, tc.storageBytes, 1000)

				start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
				end := start.AddDate(0, 1, 0)

				record := stripe.ProjectRecord{ProjectID: project.ID, Storage: 1}
				_, err = stripeService.ProcessRecord(ctx, record, productUsages, productInfos, start, end)
				require.NoError(t, err)

				invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

				var legacyStorageItem, legacyEgressItem, newStorageItem, newEgressItem *stripeSDK.InvoiceItemParams

				for _, item := range invoiceItems {
					desc := *item.Description
					if strings.Contains(desc, "Legacy Product") {
						if strings.Contains(desc, "Storage") {
							legacyStorageItem = item
						} else if strings.Contains(desc, "Egress") {
							legacyEgressItem = item
						}
					} else if strings.Contains(desc, "New Product") {
						if strings.Contains(desc, "Storage") {
							newStorageItem = item
						} else if strings.Contains(desc, "Egress") {
							newEgressItem = item
						}
					}
				}

				// Verify new product uses GB units with round-to-NEAREST.
				require.NotNil(t, newStorageItem)
				require.Contains(t, *newStorageItem.Description, "GB-Month")
				require.Equal(t, tc.expectedNewStorageGB, *newStorageItem.Quantity, "Storage quantity mismatch")

				require.NotNil(t, newEgressItem)
				require.Contains(t, *newEgressItem.Description, "GB")
				require.Equal(t, tc.expectedNewEgressGB, *newEgressItem.Quantity, "Egress quantity mismatch")

				// Verify legacy quantities (MB units, round to nearest).
				if tc.expectedLegacyStorageMB > 0 {
					require.Equal(t, tc.expectedLegacyStorageMB, *legacyStorageItem.Quantity, "Legacy storage quantity mismatch")
				}
				if tc.expectedLegacyEgressMB > 0 {
					require.Equal(t, tc.expectedLegacyEgressMB, *legacyEgressItem.Quantity, "Legacy egress quantity mismatch")
				}
			})
		}
	})
}

func TestUnitsAdjustment_WithEgressOverageMode(t *testing.T) {
	basePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB:           "4", // $4 per TB
		EgressTB:            "7", // $7 per TB
		Segment:             "1",
		EgressDiscountRatio: 3.0, // 3X included egress
	}

	// Product with egress overage mode enabled and GB units.
	overageProduct := paymentsconfig.ProductUsagePrice{
		ID:                3,
		Name:              "Overage Product",
		UseGBUnits:        true, // Use GB units instead of MB.
		ProjectUsagePrice: basePrice,
		EgressOverageMode: true,
	}

	var productOverrides paymentsconfig.ProductPriceOverrides
	productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		3: overageProduct,
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `20:annotation("location", "overage")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					20: 3,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		stripeService := sat.API.Payments.StripeService

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		period := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC) // Fixed date for consistent testing.
		firstDayOfMonth := time.Date(period.Year(), period.Month(), 1, 1, 0, 0, 0, period.Location())
		lastDayOfMonth := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, period.Location()).AddDate(0, 1, -1)

		overagePlacement := storj.PlacementConstraint(20)

		project, err := db.Console().Projects().Insert(ctx, &console.Project{ID: testrand.UUID(), Name: "overage test project"})
		require.NoError(t, err)

		bucket, err := db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "overage-bucket",
			ProjectID: project.ID,
			Placement: overagePlacement,
		})
		require.NoError(t, err)

		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucket.Name),
			Placement:  &overagePlacement,
		})
		require.NoError(t, err)

		egressBytes := int64(7000 * memory.MB)
		storageBytes := int64(2100 * memory.MB)

		generateProjectUsage(ctx, t, db, project.ID, firstDayOfMonth, lastDayOfMonth, bucket.Name, egressBytes, storageBytes, 1000)

		productUsages := make(map[int32]accounting.ProjectUsage)
		productInfos := make(map[int32]payments.ProductUsagePriceModel)

		start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		record := stripe.ProjectRecord{ProjectID: project.ID, Storage: 1}
		_, err = stripeService.ProcessRecord(ctx, record, productUsages, productInfos, start, end)
		require.NoError(t, err)

		invoiceItems := stripeService.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)

		// Find included egress and additional (overage) egress items.
		var storageItem, includedEgressItem, additionalEgressItem, segmentItem *stripeSDK.InvoiceItemParams

		for _, item := range invoiceItems {
			desc := *item.Description
			if strings.Contains(desc, "Storage") {
				storageItem = item
			} else if strings.Contains(desc, "Included Egress") {
				includedEgressItem = item
			} else if strings.Contains(desc, "Additional Egress") {
				additionalEgressItem = item
			} else if strings.Contains(desc, "Segment") {
				segmentItem = item
			}
		}

		// Verify storage uses GB units with rounding UP.
		require.NotNil(t, storageItem)
		require.Contains(t, *storageItem.Description, "GB-Month")
		require.Equal(t, int64(3), *storageItem.Quantity)
		require.Equal(t, 0.4, *storageItem.UnitAmountDecimal) // 0.0004 * 1000 = 0.4 cents/GB

		// Verify included egress item exists with GB units and $0 price.
		require.NotNil(t, includedEgressItem)
		require.Contains(t, *includedEgressItem.Description, "3X Included Egress (GB)")
		require.Equal(t, int64(6), *includedEgressItem.Quantity)
		require.Equal(t, 0.0, *includedEgressItem.UnitAmountDecimal)

		// Verify additional egress item may exist.
		require.Contains(t, *additionalEgressItem.Description, "Additional Egress (GB)")
		require.GreaterOrEqual(t, int64(1), *additionalEgressItem.Quantity)
		require.Equal(t, 0.7, *additionalEgressItem.UnitAmountDecimal) // 0.0007 * 1000 = 0.7 cents/GB

		// Verify segment item exists but is not affected by units adjustment.
		require.NotNil(t, segmentItem)
		require.Contains(t, *segmentItem.Description, "Segment-Month")
		require.Equal(t, 100.0, *segmentItem.UnitAmountDecimal)
	})
}

func generateProjectUsage(ctx context.Context, tb testing.TB, db satellite.DB, projectID uuid.UUID, start, end time.Time, bucket string, egress, totalBytes, totalSegments int64) {
	err := db.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(bucket),
		pb.PieceAction_GET, egress, 0, start)
	require.NoError(tb, err)

	tallies := map[metabase.BucketLocation]*accounting.BucketTally{
		{}: {
			BucketLocation: metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: metabase.BucketName(bucket),
			},
			TotalBytes:    totalBytes,
			TotalSegments: totalSegments,
		},
	}
	err = db.ProjectAccounting().SaveTallies(ctx, start, tallies)
	require.NoError(tb, err)
	err = db.ProjectAccounting().SaveTallies(ctx, end, tallies)
	require.NoError(tb, err)
}
