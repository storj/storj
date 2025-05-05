// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestAdminProjectPlacementAPI(t *testing.T) {
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
			basePlacementURL := fmt.Sprintf("http://%s/api/projects/%s/placement", address, testCase.project)
			t.Log(basePlacementURL)

			t.Run(testCase.name, func(t *testing.T) {
				newPlacement := storj.PlacementConstraint(1)
				assertReq(ctx, t, basePlacementURL+"?id="+strconv.Itoa(int(newPlacement)), "PUT", "", testCase.status, testCase.body, sat.Config.Console.AuthToken)

				if testCase.status == http.StatusOK {
					t.Run("Set", func(t *testing.T) {
						project, err := sat.DB.Console().Projects().Get(ctx, testCase.project)
						require.NoError(t, err)
						require.Equal(t, newPlacement, project.DefaultPlacement)

						expected, err := json.Marshal(project)
						require.NoError(t, err, "failed to json encode expected bucket")

						assertGet(ctx, t, baseURL, string(expected), sat.Config.Console.AuthToken)
					})
				}
			})
		}
	})
}
