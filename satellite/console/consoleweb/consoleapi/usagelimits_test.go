// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
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
		token, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
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
			Value:   token,
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
