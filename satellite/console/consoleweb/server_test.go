// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
)

func TestActivationRouting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user, err := service.CreateUser(ctx, console.CreateUser{
			FullName: "User",
			Email:    "u@mail.test",
			Password: "123a123",
		}, regToken.Secret)
		require.NoError(t, err)

		activationToken, err := service.GenerateActivationToken(ctx, user.ID, user.Email)
		require.NoError(t, err)

		checkActivationRedirect := func(testMsg, redirectURL string) {
			url := "http://" + sat.API.Console.Listener.Addr().String() + "/activation/?token=" + activationToken

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err, testMsg)

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err, testMsg)

			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode, testMsg)
			require.Equal(t, redirectURL, result.Header.Get("Location"), testMsg)
			require.NoError(t, result.Body.Close(), testMsg)
		}

		http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}

		loginURL := "http://" + sat.API.Console.Listener.Addr().String() + "/login"

		checkActivationRedirect("Activation - Fresh Token", loginURL+"?activated=true")
		checkActivationRedirect("Activation - Used Token", loginURL+"?activated=false")
	})
}
