// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

func Test_TotalUsageLimits(t *testing.T) {
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

		// we are using full name as a password
		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.Client{}

		req, err := http.NewRequestWithContext(
			ctx,
			"GET",
			"http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/projects/usage-limits",
			nil,
		)
		require.NoError(t, err)

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		req.AddCookie(&cookie)

		result, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err := ioutil.ReadAll(result.Body)
		require.NoError(t, err)

		var output console.ProjectUsageLimits

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, int64(0), output.BandwidthUsed)
		require.Equal(t, int64(0), output.StorageUsed)
		require.Equal(t, int64(expectedLimit*3), output.BandwidthLimit)
		require.Equal(t, int64(expectedLimit*3), output.StorageLimit)

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}

func Test_DailyUsage(t *testing.T) {
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

		_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID)
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

		// we are using full name as a password
		tokenInfo, err := satelliteSys.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		client := http.DefaultClient

		req, err := http.NewRequestWithContext(
			ctx,
			"GET",
			fmt.Sprintf("http://%s/api/v0/projects/%s/daily-usage?from=%s&to=%s", planet.Satellites[0].API.Console.Listener.Addr().String(), projectID.String(), since, before),
			nil,
		)
		require.NoError(t, err)

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   tokenInfo.Token.String(),
			Expires: expire,
		}

		req.AddCookie(&cookie)

		result, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err := ioutil.ReadAll(result.Body)
		require.NoError(t, err)

		var output accounting.ProjectDailyUsage

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.GreaterOrEqual(t, output.StorageUsage[0].Value, 15*memory.KiB)
		require.GreaterOrEqual(t, output.AllocatedBandwidthUsage[0].Value, 5*memory.KiB)
		require.GreaterOrEqual(t, output.SettledBandwidthUsage[0].Value, 5*memory.KiB)

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}
