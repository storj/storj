// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestProject(t *testing.T) {
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

		projectid := planet.Uplinks[0].ProjectID[sat.ID()]
		link := "http://" + address.String() + "/project/" + projectid.String() + "/limit"

		t.Run("Get", func(t *testing.T) {
			assertGet(t, link, `{"usage":{"amount":"0 B","bytes":0},"rate":{"rps":0}}`)
		})

		t.Run("UpdateUsage", func(t *testing.T) {
			data := url.Values{"usage": []string{"1TiB"}}
			req, err := http.NewRequest(http.MethodPost, link, strings.NewReader(data.Encode()))
			require.NoError(t, err)
			req.Header.Set("Authorization", "very-secret-token")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, link, `{"usage":{"amount":"1.0 TiB","bytes":1099511627776},"rate":{"rps":0}}`)

			req, err = http.NewRequest(http.MethodPut, link+"?usage=1GB", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "very-secret-token")

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, link, `{"usage":{"amount":"1.0 GB","bytes":1000000000},"rate":{"rps":0}}`)
		})

		t.Run("UpdateRate", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPut, link+"?rate=100", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "very-secret-token")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, link, `{"usage":{"amount":"1.0 GB","bytes":1000000000},"rate":{"rps":100}}`)
		})
	})
}

func assertGet(t *testing.T, link string, expected string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, link, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "very-secret-token")

	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	data, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	require.Equal(t, expected, string(data))
}
