// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/oidc"
)

func TestAdminOAuthAPI(t *testing.T) {
	id, err := uuid.New()
	require.NoError(t, err)

	userID, err := uuid.New()
	require.NoError(t, err)

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
		sat := planet.Satellites[0]

		address := sat.Admin.Admin.Listener.Addr()

		baseURL := fmt.Sprintf("http://%s/api/oauth/clients", address)
		empty := oidc.OAuthClient{}
		client := oidc.OAuthClient{ID: id, Secret: []byte("badadmin"), UserID: userID, RedirectURL: "http://localhost:1234"}
		updated := client
		updated.RedirectURL = "http://localhost:1235"

		testCases := []struct {
			name    string
			id      string
			request interface{}
			status  int
		}{
			{"create - bad request", "", empty, 400},
			{"create - success", "", client, 200},
			{"update - empty", id.String(), empty, 200},
			{"update - success", id.String(), updated, 200},
			{"delete", id.String(), nil, 200},
		}

		for _, testCase := range testCases {
			t.Log(testCase.name)

			method := http.MethodPost
			url := baseURL

			if testCase.request == nil {
				method = http.MethodDelete
				url += "/" + testCase.id
			} else if testCase.id != "" {
				method = http.MethodPut
				url += "/" + testCase.id
			}

			body := ""
			if testCase.request != nil {
				data, err := json.Marshal(testCase.request)
				require.NoError(t, err)
				if len(data) > 0 {
					body = string(data)
				}
			}

			assertReq(ctx, t,
				url, method, body,
				testCase.status, "",
				sat.Config.Console.AuthToken)
		}
	})
}
