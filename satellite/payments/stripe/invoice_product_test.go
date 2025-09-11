// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
