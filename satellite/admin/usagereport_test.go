// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/base64"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

func TestUsageReport(t *testing.T) {
	const (
		standardProductID int32 = 1
		advancedProductID int32 = 2
		advancedPlacement       = storj.PlacementConstraint(12)
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "standard");12:annotation("location", "advanced")`,
				}
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					standardProductID: {
						Name:                     "standard tier",
						SmallObjectFee:           "7",
						MinimumRetentionFee:      "7",
						MinimumRetentionDuration: "720h",
						Discount:                 "25",
						ProjectUsagePrice:        paymentsconfig.ProjectUsagePrice{StorageTB: "7", EgressTB: "7"},
					},
					advancedProductID: {
						Name:              "advanced tier",
						ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{StorageTB: "10", EgressTB: "10"},
					},
				})
				config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{
					int(storj.DefaultPlacement): standardProductID,
					int(advancedPlacement):      advancedProductID,
				})
				config.Payments.StripeCoinPayments.PopulateMinObjectSizeInvoiceLineItem = true
				config.Payments.StripeCoinPayments.PopulateMinRetentionInvoiceLineItem = true
				// The test inserts accounting data in the past, so the chores that purge
				// tallies and archive rollups must not run.
				config.Tally.RetentionDays = 0
				config.RollupArchive.Enabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		since := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		const period = "2025-09"

		tenantID := "sns-trio"
		newUser := func(email string, tenantID *string) *console.User {
			user, err := sat.AddUser(ctx, console.CreateUser{FullName: email, Email: email}, 10)
			require.NoError(t, err)
			require.NoError(t, sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{TenantID: &tenantID}))
			return user
		}
		newProject := func(user *console.User, name string) *console.Project {
			project, err := sat.AddProject(ctx, user.ID, name)
			require.NoError(t, err)
			return project
		}
		// addStorage adds two hours of storage of a bucket of totalBytes, of which remainderBytes is
		// the small object remainder.
		addStorage := func(project *console.Project, bucketName string, placement storj.PlacementConstraint, totalBytes, remainderBytes int64) {
			_, err := sat.DB.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project.ID,
				BucketName: []byte(bucketName),
				Placement:  &placement,
			})
			require.NoError(t, err)

			for i := 1; i <= 3; i++ {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:     bucketName,
					ProjectID:      project.ID,
					IntervalStart:  since.Add(time.Duration(i) * time.Hour),
					TotalBytes:     totalBytes,
					RemainderBytes: remainderBytes,
				}))
			}
		}
		addEgress := func(project *console.Project, bucketName string, bytes int64) {
			require.NoError(t, sat.DB.Orders().UpdateBandwidthBatch(ctx, []orders.BucketBandwidthRollup{{
				ProjectID:     project.ID,
				BucketName:    bucketName,
				IntervalStart: since.Add(time.Hour),
				Action:        pb.PieceAction_GET,
				Settled:       bytes,
			}}))
		}

		// 2 hours of 720TB are 2000GB-months.
		const bytes2000GBMonth = 720e12

		userA := newUser("a@sns.test", &tenantID)
		userB := newUser("b@other.test", nil)

		// the name is escaped in the CSV so that spreadsheets don't read it as a formula.
		projectA1 := newProject(userA, "=a1")
		addStorage(projectA1, "bucket", storj.DefaultPlacement, bytes2000GBMonth, bytes2000GBMonth/10)
		addEgress(projectA1, "bucket", 100e9)
		// retention remainder charges are reported even when they are already billed.
		require.NoError(t, sat.DB.RetentionRemainderCharges().Upsert(ctx, accounting.RetentionRemainderCharge{
			ProjectID:          projectA1.ID,
			BucketName:         "bucket",
			DeletedAt:          since,
			RemainderByteHours: 10 * 720e9,
			ProductID:          standardProductID,
			Billed:             true,
		}))

		projectA2 := newProject(userA, "a2")
		addStorage(projectA2, "advanced", advancedPlacement, bytes2000GBMonth, 0)
		addEgress(projectA2, "standard", 100e9)

		// projects without usage aren't reported.
		newProject(userA, "a3")

		projectB1 := newProject(userB, "b1")
		addEgress(projectB1, "bucket", 100e9)

		charge := func(standard, discount int64) backoffice.UsageReportCharge {
			return backoffice.UsageReportCharge{StandardCents: standard, DiscountCents: discount, BillableCents: standard - discount}
		}

		// $7/TB of 1800GB-months of storage, 100GB of egress, 200GB-months of small object remainder and
		// 10GB-months of retention remainder, discounted by 25%.
		itemA1 := backoffice.UsageReportItem{
			UserID:               userA.ID,
			UserEmail:            userA.Email,
			ProjectID:            projectA1.PublicID,
			ProjectName:          projectA1.Name,
			Period:               period,
			ProductID:            standardProductID,
			ProductName:          "standard tier",
			DiscountPercent:      "25",
			StorageGBMonth:       1800,
			EgressGB:             100,
			MinObjectSizeGBMonth: 200,
			MinRetentionGBMonth:  10,
			Storage:              charge(1260, 315),
			Egress:               charge(70, 18),
			MinObjectSize:        charge(140, 35),
			MinRetention:         charge(7, 2),
			Total:                charge(1477, 370),
		}
		itemA2Standard := backoffice.UsageReportItem{
			UserID:          userA.ID,
			UserEmail:       userA.Email,
			ProjectID:       projectA2.PublicID,
			ProjectName:     projectA2.Name,
			Period:          period,
			ProductID:       standardProductID,
			ProductName:     "standard tier",
			DiscountPercent: "25",
			EgressGB:        100,
			Egress:          charge(70, 18),
			Total:           charge(70, 18),
		}
		// $10/TB of 2000GB-months of storage without discount.
		itemA2Advanced := backoffice.UsageReportItem{
			UserID:          userA.ID,
			UserEmail:       userA.Email,
			ProjectID:       projectA2.PublicID,
			ProjectName:     projectA2.Name,
			Period:          period,
			ProductID:       advancedProductID,
			ProductName:     "advanced tier",
			DiscountPercent: "0",
			StorageGBMonth:  2000,
			Storage:         charge(2000, 0),
			Total:           charge(2000, 0),
		}

		requireItemsEqual := func(t *testing.T, expected, actual []backoffice.UsageReportItem) {
			require.Len(t, actual, len(expected))
			for i := range expected {
				require.InDelta(t, expected[i].StorageGBMonth, actual[i].StorageGBMonth, 1e-6)
				require.InDelta(t, expected[i].EgressGB, actual[i].EgressGB, 1e-6)
				require.InDelta(t, expected[i].MinObjectSizeGBMonth, actual[i].MinObjectSizeGBMonth, 1e-6)
				require.InDelta(t, expected[i].MinRetentionGBMonth, actual[i].MinRetentionGBMonth, 1e-6)
				expected[i].StorageGBMonth, actual[i].StorageGBMonth = 0, 0
				expected[i].EgressGB, actual[i].EgressGB = 0, 0
				expected[i].MinObjectSizeGBMonth, actual[i].MinObjectSizeGBMonth = 0, 0
				expected[i].MinRetentionGBMonth, actual[i].MinRetentionGBMonth = 0, 0
				require.Equal(t, expected[i], actual[i])
			}
		}

		service.TestSetTenantID(&tenantID)
		defer service.TestSetTenantID(nil)

		t.Run("pages", func(t *testing.T) {
			page, apiErr := service.GetUsageReport(ctx, period, "1", "")
			require.NoError(t, apiErr.Err)
			requireItemsEqual(t, []backoffice.UsageReportItem{itemA1}, page.Items)
			require.NotEmpty(t, page.NextCursor)

			page, apiErr = service.GetUsageReport(ctx, period, "1", page.NextCursor)
			require.NoError(t, apiErr.Err)
			requireItemsEqual(t, []backoffice.UsageReportItem{itemA2Standard, itemA2Advanced}, page.Items)
			require.Empty(t, page.NextCursor)

			page, apiErr = service.GetUsageReport(ctx, period, "10", "")
			require.NoError(t, apiErr.Err)
			require.Len(t, page.Items, 3)
			require.Empty(t, page.NextCursor)

			// other months have no usage.
			page, apiErr = service.GetUsageReport(ctx, "2025-08", "10", "")
			require.NoError(t, apiErr.Err)
			require.Empty(t, page.Items)
			require.Empty(t, page.NextCursor)
		})

		t.Run("all tenants", func(t *testing.T) {
			service.TestSetTenantID(nil)
			defer service.TestSetTenantID(&tenantID)

			page, apiErr := service.GetUsageReport(ctx, period, "10", "")
			require.NoError(t, apiErr.Err)
			require.Len(t, page.Items, 4)
			require.Equal(t, projectB1.PublicID, page.Items[3].ProjectID)
		})

		t.Run("invalid parameters", func(t *testing.T) {
			for _, period := range []string{"", "2025-13", "2025-09-01", time.Now().AddDate(0, 1, 0).Format("2006-01")} {
				_, apiErr := service.GetUsageReport(ctx, period, "10", "")
				require.Equal(t, http.StatusBadRequest, apiErr.Status, period)

				apiErr = service.DownloadUsageReport(ctx, httptest.NewRecorder(), period)
				require.Equal(t, http.StatusBadRequest, apiErr.Status, period)
			}

			_, apiErr := service.GetUsageReport(ctx, period, "a", "")
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			for _, cursor := range []string{
				"not-a-cursor",
				base64.RawURLEncoding.EncodeToString([]byte(`{"e":"A@SNS.TEST"}`)),
				// cursors without the owner ID are from before the projects were ordered by owner.
				base64.RawURLEncoding.EncodeToString([]byte(`{"e":"A@SNS.TEST","n":"a1","p":"` + projectA1.PublicID.String() + `"}`)),
			} {
				_, apiErr = service.GetUsageReport(ctx, period, "10", cursor)
				require.Equal(t, http.StatusBadRequest, apiErr.Status, cursor)
			}
		})

		t.Run("csv", func(t *testing.T) {
			// the CSV file is written page by page.
			service.TestSetUsageReportCSVPageLimit(1)
			defer service.TestSetUsageReportCSVPageLimit(100)

			recorder := httptest.NewRecorder()
			apiErr := service.DownloadUsageReport(ctx, recorder, period)
			require.NoError(t, apiErr.Err)
			require.Equal(t, "attachment; filename=usage-report-2025-09.csv", recorder.Header().Get("Content-Disposition"))

			records, err := csv.NewReader(recorder.Body).ReadAll()
			require.NoError(t, err)

			require.Equal(t, []string{
				"email", "project_name", "public_id", "month", "tier",
				"storage_gb_month", "egress_gb", "segment_month", "min_file_size_gb_month", "min_retention_gb_month",
				"min_storage_gb_month", "storage_overage_gb_month",
				"storage standard price", "storage discount", "storage billable price",
				"egress standard price", "egress discount", "egress billable price",
				"segment standard price", "segment discount", "segment billable price",
				"min_file_size standard price", "min_file_size discount", "min_file_size billable price",
				"min_retention standard price", "min_retention discount", "min_retention billable price",
				"min_storage standard price", "min_storage discount", "min_storage billable price",
				"storage_overage standard price", "storage_overage discount", "storage_overage billable price",
				"Grand Total",
			}, records[0])

			publicID := func(project *console.Project) string { return project.PublicID.String() }
			require.Equal(t, [][]string{
				{
					"a@sns.test", "'=a1", publicID(projectA1), period, "standard tier",
					"1800", "100", "0", "200", "10", "0", "0",
					"12.60", "-3.15", "9.45",
					"0.70", "-0.18", "0.52",
					"0.00", "0.00", "0.00",
					"1.40", "-0.35", "1.05",
					"0.07", "-0.02", "0.05",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"11.07",
				},
				{
					"a@sns.test", "a2", publicID(projectA2), period, "standard tier",
					"0", "100", "0", "0", "0", "0", "0",
					"0.00", "0.00", "0.00",
					"0.70", "-0.18", "0.52",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.52",
				},
				{
					"a@sns.test", "a2", publicID(projectA2), period, "advanced tier",
					"2000", "0", "0", "0", "0", "0", "0",
					"20.00", "0.00", "20.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"20.00",
				},
				{
					"", "", "", "", "",
					"3800", "200", "0", "200", "10", "0", "0",
					"32.60", "-3.15", "29.45",
					"1.40", "-0.36", "1.04",
					"0.00", "0.00", "0.00",
					"1.40", "-0.35", "1.05",
					"0.07", "-0.02", "0.05",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"31.59",
				},
				append([]string{"Standard billing amount", "35.47"}, make([]string, 32)...),
				append([]string{"Discount", "-3.88"}, make([]string, 32)...),
				append([]string{"Total amount", "31.59"}, make([]string, 32)...),
			}, records[1:])
		})
	})
}

func TestUsageReport_MinimumStorage(t *testing.T) {
	const productID int32 = 1

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					productID: {
						Name:              "committed tier",
						UseGBUnits:        true,
						MinimumStorage:    "5TB",
						StorageOverage:    "7",
						Discount:          "20",
						ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{StorageTB: "6", EgressTB: "7"},
					},
				})
				config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{int(storj.DefaultPlacement): productID})
				// The test inserts accounting data in the past, so the chores that purge
				// tallies and archive rollups must not run.
				config.Tally.RetentionDays = 0
				config.RollupArchive.Enabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		since := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		const period = "2025-09"

		// addProject adds a project with 2 hours of 720TB of storage, which are 2000GB-months.
		addProject := func(user *console.User, name string) *console.Project {
			project, err := sat.AddProject(ctx, user.ID, name)
			require.NoError(t, err)

			placement := storj.DefaultPlacement
			_, err = sat.DB.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project.ID,
				BucketName: []byte("bucket"),
				Placement:  &placement,
			})
			require.NoError(t, err)
			for i := 1; i <= 3; i++ {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:    "bucket",
					ProjectID:     project.ID,
					IntervalStart: since.Add(time.Duration(i) * time.Hour),
					TotalBytes:    720e12,
				}))
			}
			return project
		}
		newUser := func(email string, kind console.UserKind) *console.User {
			user, err := sat.AddUser(ctx, console.CreateUser{FullName: email, Email: email, Kind: kind}, 10)
			require.NoError(t, err)
			return user
		}

		// userA stores 4000GB-months, 1000GB-months short of the 5TB minimum.
		userA := newUser("a@minimum.test", console.PaidUser)
		projectA1 := addProject(userA, "a1")
		projectA2 := addProject(userA, "a2")
		// userB stores 6000GB-months, 1000GB-months above the 5TB minimum.
		userB := newUser("b@minimum.test", console.PaidUser)
		projectB1 := addProject(userB, "b1")
		projectB2 := addProject(userB, "b2")
		projectB3 := addProject(userB, "b3")
		// userC stores 2000GB-months, short of the minimum, but free users aren't invoiced, so they
		// have no adjustments.
		userC := newUser("c@minimum.test", console.FreeUser)
		projectC1 := addProject(userC, "c1")

		charge := func(standard, discount int64) backoffice.UsageReportCharge {
			return backoffice.UsageReportCharge{StandardCents: standard, DiscountCents: discount, BillableCents: standard - discount}
		}
		// $6/TB of 2000GB-months of storage, discounted by 20%.
		projectItem := func(user *console.User, project *console.Project) backoffice.UsageReportItem {
			return backoffice.UsageReportItem{
				UserID:              user.ID,
				UserEmail:           user.Email,
				ProjectID:           project.PublicID,
				ProjectName:         project.Name,
				Period:              period,
				ProductID:           productID,
				ProductName:         "committed tier",
				DiscountPercent:     "20",
				MinimumStorageBytes: 5e12,
				StorageGBMonth:      2000,
				Storage:             charge(1200, 240),
				Total:               charge(1200, 240),
			}
		}
		requireItemsEqual := func(t *testing.T, expected, actual []backoffice.UsageReportItem) {
			require.Len(t, actual, len(expected))
			for i := range expected {
				require.InDelta(t, expected[i].StorageGBMonth, actual[i].StorageGBMonth, 1e-6)
				expected[i].StorageGBMonth, actual[i].StorageGBMonth = 0, 0
				require.Equal(t, expected[i], actual[i])
			}
		}

		t.Run("pages", func(t *testing.T) {
			// the pages only have the items of the projects, the adjustments are only in the CSV.
			page, apiErr := service.GetUsageReport(ctx, period, "10", "")
			require.NoError(t, apiErr.Err)
			requireItemsEqual(t, []backoffice.UsageReportItem{
				projectItem(userA, projectA1), projectItem(userA, projectA2),
				projectItem(userB, projectB1), projectItem(userB, projectB2), projectItem(userB, projectB3),
				projectItem(userC, projectC1),
			}, page.Items)
		})

		t.Run("csv", func(t *testing.T) {
			// with 1 project in each page, the storage of a user's projects is added up across the pages.
			service.TestSetUsageReportCSVPageLimit(1)
			defer service.TestSetUsageReportCSVPageLimit(100)

			recorder := httptest.NewRecorder()
			apiErr := service.DownloadUsageReport(ctx, recorder, period)
			require.NoError(t, apiErr.Err)

			records, err := csv.NewReader(recorder.Body).ReadAll()
			require.NoError(t, err)

			projectRow := func(user *console.User, project *console.Project) []string {
				return []string{
					user.Email, project.Name, project.PublicID.String(), period, "committed tier",
					"2000", "0", "0", "0", "0", "0", "0",
					"12.00", "-2.40", "9.60",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"9.60",
				}
			}
			require.Equal(t, [][]string{
				projectRow(userA, projectA1),
				projectRow(userA, projectA2),
				{
					userA.Email, "", "", period, "committed tier",
					"0", "0", "0", "0", "0", "1000", "0",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"6.00", "-1.20", "4.80",
					"0.00", "0.00", "0.00",
					"4.80",
				},
				projectRow(userB, projectB1),
				projectRow(userB, projectB2),
				projectRow(userB, projectB3),
				{
					userB.Email, "", "", period, "committed tier",
					"0", "0", "0", "0", "0", "0", "1000",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"1.00", "-0.20", "0.80",
					"0.80",
				},
				projectRow(userC, projectC1),
				{
					"", "", "", "", "",
					"12000", "0", "0", "0", "0", "1000", "1000",
					"72.00", "-14.40", "57.60",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"0.00", "0.00", "0.00",
					"6.00", "-1.20", "4.80",
					"1.00", "-0.20", "0.80",
					"63.20",
				},
				// the invoices are $30.00 of storage with a $6.00 discount for userA, and $30.00 of storage
				// and $7.00 of overage storage with a $7.40 discount for userB. Like every report row, userC's
				// project row is there although free users aren't invoiced.
				append([]string{"Standard billing amount", "79.00"}, make([]string, 32)...),
				append([]string{"Discount", "-15.80"}, make([]string, 32)...),
				append([]string{"Total amount", "63.20"}, make([]string, 32)...),
			}, records[1:])
		})
	})
}
