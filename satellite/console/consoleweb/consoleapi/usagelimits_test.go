// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

func TestTotalUsageLimits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "Usage Limit Test",
			ShortName: "",
			Email:     "ul@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 3)
		require.NoError(t, err)

		project0, err := sat.AddProject(ctx, user.ID, "testProject0")
		require.NoError(t, err)

		project1, err := sat.AddProject(ctx, user.ID, "testProject1")
		require.NoError(t, err)

		project2, err := sat.AddProject(ctx, user.ID, "testProject2")
		require.NoError(t, err)

		const expectedLimit = 15

		err = sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, project0.ID, expectedLimit)
		require.NoError(t, err)
		err = sat.DB.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, project0.ID, expectedLimit)
		require.NoError(t, err)

		err = sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, project1.ID, expectedLimit)
		require.NoError(t, err)
		err = sat.DB.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, project1.ID, expectedLimit)
		require.NoError(t, err)

		err = sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, project2.ID, expectedLimit)
		require.NoError(t, err)
		err = sat.DB.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, project2.ID, expectedLimit)
		require.NoError(t, err)

		body, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, "projects/usage-limits", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		var output console.ProjectUsageLimits

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, int64(0), output.BandwidthUsed)
		require.Equal(t, int64(0), output.StorageUsed)
		require.Equal(t, int64(expectedLimit*3), output.BandwidthLimit)
		require.Equal(t, int64(expectedLimit*3), output.StorageLimit)
	})
}

func TestDailyUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "testbucket"
			firstPath  = "path"
			secondPath = "another_path"
		)

		now := time.Now()
		inFiveMinutes := time.Now().Add(5 * time.Minute)

		var (
			satelliteSys = planet.Satellites[0]
			uplink       = planet.Uplinks[0]
			projectID    = uplink.Projects[0].ID
			since        = strconv.FormatInt(now.Unix(), 10)
			before       = strconv.FormatInt(inFiveMinutes.Unix(), 10)
		)

		newUser := console.CreateUser{
			FullName:  "Daily Usage Test",
			ShortName: "",
			Email:     "du@test.test",
		}

		user, err := satelliteSys.AddUser(ctx, newUser, 3)
		require.NoError(t, err)

		_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID, console.RoleAdmin)
		require.NoError(t, err)

		planet.Satellites[0].Orders.Chore.Loop.Pause()
		satelliteSys.Accounting.Tally.Loop.Pause()

		usage, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
		require.NoError(t, err)
		require.Zero(t, len(usage.AllocatedBandwidthUsage))
		require.Zero(t, len(usage.SettledBandwidthUsage))
		require.Zero(t, len(usage.StorageUsage))

		firstSegment := testrand.Bytes(5 * memory.KiB)
		secondSegment := testrand.Bytes(10 * memory.KiB)

		err = uplink.Upload(ctx, satelliteSys, bucketName, firstPath, firstSegment)
		require.NoError(t, err)
		err = uplink.Upload(ctx, satelliteSys, bucketName, secondPath, secondSegment)
		require.NoError(t, err)

		_, err = uplink.Download(ctx, satelliteSys, bucketName, firstPath)
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
		tomorrow := time.Now().Add(24 * time.Hour)
		planet.StorageNodes[0].Storage2.Orders.SendOrders(ctx, tomorrow)

		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()
		satelliteSys.Accounting.Tally.Loop.TriggerWait()

		endpoint := fmt.Sprintf("projects/%s/daily-usage?from=%s&to=%s", projectID.String(), since, before)
		body, status, err := doRequestWithAuth(ctx, t, satelliteSys, user, http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		var output accounting.ProjectDailyUsage

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.GreaterOrEqual(t, output.StorageUsage[0].Value, 15*memory.KiB)
		require.GreaterOrEqual(t, output.AllocatedBandwidthUsage[0].Value, 5*memory.KiB)
		require.GreaterOrEqual(t, output.SettledBandwidthUsage[0].Value, 5*memory.KiB)
	})
}

func TestTotalUsageReport(t *testing.T) {
	var (
		productID    = int32(1)
		productPrice = paymentsconfig.ProductUsagePrice{
			Name: "product1",
			ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "200000",
				EgressTB:  "200000",
				Segment:   "2",
			},
		}
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
				config.Placement = nodeselection.ConfigurablePlacementRule{PlacementRules: `0:annotation("location", "defaultPlacement")`}
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{productID: productPrice})
				config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{int(storj.DefaultPlacement): productID})
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			satelliteSys      = planet.Satellites[0]
			service           = satelliteSys.API.Console.Service
			now               = time.Now()
			inFiveMinutes     = now.Add(5 * time.Minute)
			inAnHour          = now.Add(1 * time.Hour)
			since             = strconv.FormatInt(now.Unix(), 10)
			notAllowedBefore  = strconv.FormatInt(now.Add(satelliteSys.Config.Console.AllowedUsageReportDateRange+1*time.Second).Unix(), 10)
			expectedCSVValue  = fmt.Sprintf("%f", 0.0)
			expectedCSVValue2 = fmt.Sprintf("%.2f", 0.0)
		)

		newUser := console.CreateUser{
			FullName:  "Total Usage Report Test",
			ShortName: "",
			Email:     "ur@test.test",
		}

		user, err := satelliteSys.AddUser(ctx, newUser, 3)
		require.NoError(t, err)

		endpoint := fmt.Sprintf("projects/usage-report?since=%s&before=%s&projectID=", since, notAllowedBefore)
		_, status, err := doRequestWithAuth(ctx, t, satelliteSys, user, http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, status)

		createApiKey := func(project *console.Project) *macaroon.APIKey {
			secret, err := macaroon.NewSecret()
			require.NoError(t, err)

			key, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			keyInfo := console.APIKeyInfo{
				Name:      project.Name,
				ProjectID: project.ID,
				Secret:    secret,
			}
			_, err = satelliteSys.DB.Console().APIKeys().Create(ctx, key.Head(), keyInfo)
			require.NoError(t, err)

			return key
		}

		project1, err := satelliteSys.AddProject(ctx, user.ID, "testProject1")
		require.NoError(t, err)
		createdAPIKey := createApiKey(project1)

		project2, err := satelliteSys.AddProject(ctx, user.ID, "testProject2")
		require.NoError(t, err)
		createdAPIKey2 := createApiKey(project2)

		bucketName := "bucket"
		_, err = satelliteSys.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{ApiKey: createdAPIKey.SerializeRaw()},
			Name:   []byte(bucketName),
		})
		require.NoError(t, err)

		_, err = satelliteSys.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{ApiKey: createdAPIKey2.SerializeRaw()},
			Name:   []byte(bucketName),
		})
		require.NoError(t, err)

		bucketLoc1 := metabase.BucketLocation{
			ProjectID:  project1.ID,
			BucketName: metabase.BucketName(bucketName),
		}
		tally1 := &accounting.BucketTally{
			BucketLocation: bucketLoc1,
		}

		bucketLoc2 := metabase.BucketLocation{
			ProjectID:  project2.ID,
			BucketName: metabase.BucketName(bucketName),
		}
		tally2 := &accounting.BucketTally{
			BucketLocation: bucketLoc2,
		}

		bucketTallies := map[metabase.BucketLocation]*accounting.BucketTally{
			bucketLoc1: tally1,
			bucketLoc2: tally2,
		}

		err = satelliteSys.DB.ProjectAccounting().SaveTallies(ctx, inFiveMinutes, bucketTallies)
		require.NoError(t, err)

		checkCSVContent := func(para console.GetUsageReportParam, newReportEnabled bool) {
			endpoint = fmt.Sprintf(
				"projects/usage-report?project-summary=%v&cost=%v&since=%s&before=%s&projectID=%s",
				para.GroupByProject, para.IncludeCost,
				strconv.FormatInt(para.Since.Unix(), 10),
				strconv.FormatInt(para.Before.Unix(), 10),
				para.ProjectID,
			)
			body, status, err := doRequestWithAuth(ctx, t, satelliteSys, user, http.MethodGet, endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			reader := csv.NewReader(strings.NewReader(string(body)))
			records, err := reader.ReadAll()
			require.NoError(t, err)
			if newReportEnabled && para.IncludeCost {
				require.Len(t, records, 3)
				require.Contains(t, records[0][0], "Disclaimer")
			} else {
				require.Len(t, records, 2)
			}

			disclaimerRow, expectedHeaders := service.GetUsageReportHeaders(para)
			if newReportEnabled && para.IncludeCost {
				require.NotEmpty(t, disclaimerRow)
			} else {
				require.Empty(t, disclaimerRow)
			}

			disclaimerIndex := -1
			if newReportEnabled && para.IncludeCost {
				disclaimerIndex = 0
			}
			for i, header := range expectedHeaders {
				require.Equal(t, header, records[disclaimerIndex+1][i])
			}

			require.Equal(t, project1.Name, records[disclaimerIndex+2][0])
			require.Equal(t, project1.PublicID.String(), records[disclaimerIndex+2][1])
			if !newReportEnabled {
				require.Equal(t, bucketName, records[disclaimerIndex+2][2])
				return
			}
			if !para.GroupByProject {
				require.Equal(t, bucketName, records[disclaimerIndex+2][2])
			} else {
				require.Equal(t, expectedCSVValue, records[disclaimerIndex+2][2])
			}

			startIndex := 3
			if para.GroupByProject {
				startIndex = 2
			}
			for i := startIndex; i < len(expectedHeaders)-3; i++ {
				value := records[disclaimerIndex+2][i]
				require.True(t, expectedCSVValue == value || expectedCSVValue2 == value)
			}
		}

		reportTestConf := []struct {
			name             string
			newReportEnabled bool
		}{
			{
				name:             "NewReportDisabled",
				newReportEnabled: false,
			},
			{
				name:             "NewReportEnabled",
				newReportEnabled: true,
			},
		}
		for _, conf := range reportTestConf {
			t.Run(conf.name, func(t *testing.T) {
				service.TestSetNewUsageReportEnabled(conf.newReportEnabled)

				param := console.GetUsageReportParam{
					Since:  now,
					Before: inAnHour,
				}

				// test without projectID
				endpoint = fmt.Sprintf(
					"projects/usage-report?since=%s&before=%s&projectID=",
					strconv.FormatInt(param.Since.Unix(), 10),
					strconv.FormatInt(param.Before.Unix(), 10),
				)
				body, status, err := doRequestWithAuth(ctx, t, satelliteSys, user, http.MethodGet, endpoint, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, status)

				reader := csv.NewReader(strings.NewReader(string(body)))
				records, err := reader.ReadAll()
				require.NoError(t, err)
				require.Len(t, records, 3)

				disclaimerRow, expectedHeaders := service.GetUsageReportHeaders(param)
				require.Empty(t, disclaimerRow)
				for i, header := range expectedHeaders {
					require.Equal(t, header, records[0][i])
				}

				require.Equal(t, project1.Name, records[1][0])
				require.Equal(t, project2.Name, records[2][0])
				require.Equal(t, project1.PublicID.String(), records[1][1])
				require.Equal(t, project2.PublicID.String(), records[2][1])
				require.Equal(t, bucketName, records[1][2])
				require.Equal(t, bucketName, records[2][2])
				for i := 3; i < 7; i++ {
					require.Equal(t, expectedCSVValue, records[1][i])
					require.Equal(t, expectedCSVValue, records[2][i])
				}

				// test with projectID
				param.ProjectID = project1.ID

				checkCSVContent(param, conf.newReportEnabled)

				param.GroupByProject = true
				checkCSVContent(param, conf.newReportEnabled)

				param.IncludeCost = true
				checkCSVContent(param, conf.newReportEnabled)
			})
		}
	})
}
