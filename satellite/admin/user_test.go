// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestGetUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]

		t.Run("GetUser", func(t *testing.T) {
			userLink := "http://" + address.String() + "/api/user/" + project.Owner.Email
			expected := `{` +
				fmt.Sprintf(`"user":{"id":"%s","fullName":"User uplink0_0","email":"%s"},`, project.Owner.ID, project.Owner.Email) +
				fmt.Sprintf(`"projects":[{"id":"%s","name":"uplink0_0","description":"","ownerId":"%s"}],`, project.ID, project.Owner.ID) +
				`"coupons":[]}`

			req, err := http.NewRequest(http.MethodGet, userLink, nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", "very-secret-token")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			data, err := ioutil.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			require.Equal(t, http.StatusOK, response.StatusCode, string(data))
			require.Equal(t, expected, string(data))
		})
	})
}

func TestAddUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		email := "alice+2@mail.test"

		body := strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"123a123"}`, email))
		req, err := http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/user", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", "very-secret-token")

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output console.User

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		user, err := planet.Satellites[0].DB.Console().Users().Get(ctx, output.ID)
		require.NoError(t, err)
		require.Equal(t, email, user.Email)
	})
}
