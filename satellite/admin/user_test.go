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

		projLimit, err := sat.DB.Console().Users().GetProjectLimit(ctx, project.Owner.ID)
		require.NoError(t, err)

		coupons, err := sat.DB.StripeCoinPayments().Coupons().ListByUserID(ctx, project.Owner.ID)
		require.NoError(t, err)

		couponsMarshaled, err := json.Marshal(coupons)
		require.NoError(t, err)

		t.Run("GetUser", func(t *testing.T) {
			userLink := "http://" + address.String() + "/api/user/" + project.Owner.Email
			expected := `{` +
				fmt.Sprintf(`"user":{"id":"%s","fullName":"User uplink0_0","email":"%s","projectLimit":%d},`, project.Owner.ID, project.Owner.Email, projLimit) +
				fmt.Sprintf(`"projects":[{"id":"%s","name":"uplink0_0","description":"","ownerId":"%s"}],`, project.ID, project.Owner.ID) +
				fmt.Sprintf(`"coupons":%s}`, couponsMarshaled)

			req, err := http.NewRequest(http.MethodGet, userLink, nil)
			require.NoError(t, err)

			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

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
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

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

func TestAddUserSameEmail(t *testing.T) {
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
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

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

		// Add same user again, this should fail
		body = strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"123a123"}`, email))
		req, err = http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/user", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)
		require.NoError(t, response.Body.Close())
	})
}

func TestUpdateUser(t *testing.T) {
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		body := strings.NewReader(`{"email":"alice+2@mail.test", "shortName":"Newbie"}`)
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://"+address.String()+"/api/user/%s", user.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Len(t, responseBody, 0)

		updatedUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "alice+2@mail.test", updatedUser.Email)
		require.Equal(t, user.FullName, updatedUser.FullName)
		require.NotEqual(t, "Newbie", user.ShortName)
		require.Equal(t, "Newbie", updatedUser.ShortName)
		require.Equal(t, user.ID, updatedUser.ID)
		require.Equal(t, user.Status, updatedUser.Status)
		require.Equal(t, user.ProjectLimit, updatedUser.ProjectLimit)
	})
}

func TestUpdateUserRateLimit(t *testing.T) {
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		newLimit := 50

		body := strings.NewReader(fmt.Sprintf(`{"projectLimit":%d}`, newLimit))
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://"+address.String()+"/api/user/%s", user.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Len(t, responseBody, 0)

		updatedUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, user.Email, updatedUser.Email)
		require.Equal(t, user.ID, updatedUser.ID)
		require.Equal(t, user.Status, updatedUser.Status)
		require.Equal(t, newLimit, updatedUser.ProjectLimit)
	})
}

func TestDeleteUser(t *testing.T) {
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		// Deleting the user should fail, as project exists
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/user/%s", user.Email), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Greater(t, len(responseBody), 0)

		err = planet.Satellites[0].DB.Console().Projects().Delete(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		// Deleting the user should pass, as no project exists for given user
		req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/user/%s", user.Email), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err = ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, len(responseBody), 0)
	})
}
