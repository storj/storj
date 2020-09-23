// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestAuth_Register(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		for i, test := range []struct {
			Partner      string
			ValidPartner bool
		}{
			{Partner: "minio", ValidPartner: true},
			{Partner: "Minio", ValidPartner: true},
			{Partner: "Raiden Network", ValidPartner: true},
			{Partner: "Raiden nEtwork", ValidPartner: true},
			{Partner: "invalid-name", ValidPartner: false},
		} {
			registerData := struct {
				FullName       string `json:"fullName"`
				ShortName      string `json:"shortName"`
				Email          string `json:"email"`
				Partner        string `json:"partner"`
				PartnerID      string `json:"partnerId"`
				Password       string `json:"password"`
				SecretInput    string `json:"secret"`
				ReferrerUserID string `json:"referrerUserId"`
			}{
				FullName:  "testuser" + strconv.Itoa(i),
				ShortName: "test",
				Email:     "user@test" + strconv.Itoa(i),
				Partner:   test.Partner,
				Password:  "abc123",
			}

			jsonBody, err := json.Marshal(registerData)
			require.NoError(t, err)

			result, err := http.Post("http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/auth/register", "application/json", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, result.StatusCode)

			defer func() {
				err = result.Body.Close()
				require.NoError(t, err)
			}()

			body, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)

			var userID uuid.UUID
			err = json.Unmarshal(body, &userID)
			require.NoError(t, err)

			user, err := planet.Satellites[0].API.Console.Service.GetUser(ctx, userID)
			require.NoError(t, err)

			if test.ValidPartner {
				info, err := planet.Satellites[0].API.Marketing.PartnersService.ByName(ctx, test.Partner)
				require.NoError(t, err)
				require.Equal(t, info.UUID, user.PartnerID)
			} else {
				require.Equal(t, uuid.UUID{}, user.PartnerID)
			}
		}
	})
}
