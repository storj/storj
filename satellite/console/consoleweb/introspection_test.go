// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func VerifySchema(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()
		test.login(user.email, user.password)

		{ // Register User
			_ = test.registerUser("user@mail.test", "#$Rnkl12i3nkljfds")
		}

		{ // Login_GetToken_Fail
			resp, body := test.request(
				http.MethodPost, "/auth/token",
				strings.NewReader(`{"email":"wrong@invalid.test","password":"wrong"}`))
			require.Nil(t, findCookie(resp, "_tokenKey"))
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			_ = body
			// TODO: require.Contains(t, body, "unauthorized")
		}

		{ // Login_GetToken_Pass
			test.login(user.email, user.password)
		}

		{ // repeated login attempts should end in too many requests
			hitRateLimiter := false
			for i := 0; i < 30; i++ {
				resp, _ := test.request(
					http.MethodPost, "/auth/token",
					strings.NewReader(`{"email":"wrong@invalid.test","password":"wrong"}`))
				require.Nil(t, findCookie(resp, "_tokenKey"))
				if resp.StatusCode != http.StatusUnauthorized {
					require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
					hitRateLimiter = true
					break
				}
			}
			require.True(t, hitRateLimiter, "did not hit rate limiter")
		}
	})
}
