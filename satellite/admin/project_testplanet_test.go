// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestAdminProjectGeofenceAPI(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplink := planet.Uplinks[0]
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project, err := sat.DB.Console().Projects().Get(ctx, uplink.Projects[0].ID)
		require.NoError(t, err)

		testCases := []struct {
			name    string
			project uuid.UUID
			// expectations
			status int
			body   string
		}{
			{
				name:    "project does not exist",
				project: uuid.NullUUID{}.UUID,
				status:  http.StatusNotFound,
				body:    `{"error":"project with specified uuid does not exist","detail":""}`,
			},
			{
				name:    "validated",
				project: project.ID,
				status:  http.StatusOK,
				body:    "",
			},
		}

		for _, testCase := range testCases {
			baseURL := fmt.Sprintf("http://%s/api/projects/%s", address, testCase.project)
			t.Log(baseURL)
			baseGeofenceURL := fmt.Sprintf("http://%s/api/projects/%s/geofence", address, testCase.project)
			t.Log(baseGeofenceURL)

			t.Run(testCase.name, func(t *testing.T) {
				assertReq(ctx, t, baseGeofenceURL+"?region=EU", "PUT", "", testCase.status, testCase.body, sat.Config.Console.AuthToken)

				if testCase.status == http.StatusOK {

					t.Run("Set", func(t *testing.T) {
						project, err := sat.DB.Console().Projects().Get(ctx, testCase.project)
						require.NoError(t, err)
						require.Equal(t, storj.EU, project.DefaultPlacement)

						expected, err := json.Marshal(project)
						require.NoError(t, err, "failed to json encode expected bucket")

						assertGet(ctx, t, baseURL, string(expected), sat.Config.Console.AuthToken)
					})
					t.Run("Delete", func(t *testing.T) {
						assertReq(ctx, t, baseGeofenceURL, "DELETE", "", testCase.status, testCase.body, sat.Config.Console.AuthToken)

						project, err := sat.DB.Console().Projects().Get(ctx, testCase.project)
						require.NoError(t, err)
						require.Equal(t, storj.DefaultPlacement, project.DefaultPlacement)

						expected, err := json.Marshal(project)
						require.NoError(t, err)

						assertGet(ctx, t, baseURL, string(expected), sat.Config.Console.AuthToken)
					})
				}
			})
		}
	})
}
