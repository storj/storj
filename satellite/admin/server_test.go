// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/admin"
)

// TestBasic tests authorization behaviour without oauth.
func TestBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.StaticDir = "ui"
				config.Admin.BackOffice.StaticDir = "back-office/ui"
				config.Admin.BackOffice.BypassAuth = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		baseURL := "http://" + address.String()

		t.Run("UI", func(t *testing.T) {
			testUI := func(t *testing.T, requestUrl string, expectContentContains string) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl, nil)
				require.NoError(t, err)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				require.Equal(t, http.StatusOK, response.StatusCode)

				content, err := io.ReadAll(response.Body)
				require.NoError(t, response.Body.Close())
				require.NotEmpty(t, content)
				require.Contains(t, string(content), expectContentContains)
				require.NoError(t, err)
			}

			t.Run("current", func(t *testing.T) {
				testUI(t, baseURL+"/package.json", "{")
			})
			t.Run("back-office", func(t *testing.T) {
				// expect routed to back-office UI
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/back-office", nil)
				require.NoError(t, err)
				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusInternalServerError, response.StatusCode)
				require.Equal(t, "text/html; charset=UTF-8", response.Header.Get("Content-Type"))
				require.NoError(t, response.Body.Close())

				req, err = http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/back-office/child", nil)
				require.NoError(t, err)
				response, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusInternalServerError, response.StatusCode)
				require.Equal(t, "text/html; charset=UTF-8", response.Header.Get("Content-Type"))
				require.NoError(t, response.Body.Close())

				// expect routed to back-office API
				req, err = http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/back-office/api/v1/users/email/alice@storj.test", nil)
				require.NoError(t, err)
				response, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusNotFound, response.StatusCode)
				require.Equal(t, "application/json", response.Header.Get("Content-Type"))
				require.NoError(t, response.Body.Close())
			})
		})

		// Testing authorization behavior without Oauth from here on out.

		t.Run("NoAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/projects/some-id", nil)
			require.NoError(t, err)

			// This request is not through the Oauth proxy and has no authorization token, it should fail.
			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, `{"error":"Forbidden","detail":"required a valid authorization token"}`, string(body))
		})

		t.Run("WrongAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/users/alice@storj.test", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "wrong-key")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, `{"error":"Forbidden","detail":"required a valid authorization token"}`, string(body))
		})

		t.Run("WithAccess", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			// currently no main page so 404
			require.Equal(t, http.StatusNotFound, response.StatusCode)
			require.Equal(t, "text/plain; charset=utf-8", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Contains(t, string(body), "not found")
		})
	})
}

// TestWithOAuth tests authorization behaviour for requests coming through Oauth.
func TestWithOAuth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.StaticDir = "ui/build"
				config.Admin.Groups = admin.Groups{LimitUpdate: "LimitUpdate"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		address := sat.Admin.Admin.Listener.Addr().String()
		baseURL := "http://" + address

		// Make this admin server the AllowedOauthHost so withAuth thinks it's Oauth.
		sat.Admin.Admin.Server.SetAllowedOauthHost(address)

		// Requests that require full access should not be accessible through Oauth.
		t.Run("UnauthorizedThroughOauth", func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				fmt.Sprintf("%s/api/projects/%s/apikeys", baseURL, projectID.String()),
				nil,
			)
			require.NoError(t, err)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Contains(t, string(body), fmt.Sprintf(admin.UnauthorizedNotInGroup,
				[]string{planet.Satellites[0].Config.Admin.Groups.LimitUpdate}),
			)
		})

		//
		t.Run("RequireLimitUpdateAccess", func(t *testing.T) {
			targetURL := fmt.Sprintf("%s/api/projects/%s/limit", baseURL, projectID.String())
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			require.NoError(t, err)

			// this request does not have the {X-Forwarded-Groups: LimitUpdate} header. It should fail.
			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			errDetail := fmt.Sprintf(
				admin.UnauthorizedNotInGroup,
				[]string{planet.Satellites[0].Config.Admin.Groups.LimitUpdate},
			)
			require.Contains(t, string(body), errDetail)

			req, err = http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			require.NoError(t, err)

			// adding the header should allow this request.
			req.Header.Set("X-Forwarded-Groups", "LimitUpdate")

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			require.Equal(t, http.StatusOK, response.StatusCode)
		})

		// Requests of an operation through Oauth that requires the authorization token.
		t.Run("AuthorizedThroughOauthWithToken", func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				fmt.Sprintf("%s/api/projects/%s/apikeys", baseURL, projectID.String()),
				nil,
			)
			require.NoError(t, err)

			// adding the header should allow this request.
			req.Header.Set("X-Forwarded-Groups", "LimitUpdate")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Contains(
				t,
				string(body),
				"you are part of one of the authorized groups, but this operation requires a valid authorization token",
			)

			// with an invalid authorization token should happen the same than not providing it.
			req.Header.Set("Authorization", "invalid-token")

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))

			body, err = io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Contains(
				t,
				string(body),
				"you are part of one of the authorized groups, but this operation requires a valid authorization token",
			)

			// adding the authorization token should allow this request.
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			require.Equal(t, http.StatusOK, response.StatusCode)
		})

		// Test back-office API access through Oauth.
		t.Run("BackOfficeThroughOauth", func(t *testing.T) {
			t.Run("AllowedHost", func(t *testing.T) {
				sat.Admin.Admin.Service.TestSetAllowedHost(address)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/back-office/api/v1/settings/", nil)
				require.NoError(t, err)
				req.Header.Set("X-Forwarded-Groups", "LimitUpdate")
				req.Header.Add("X-Forwarded-Email", "test@example.com")

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())

				require.Equal(t, http.StatusOK, response.StatusCode)
			})

			t.Run("NotAllowedHost", func(t *testing.T) {
				sat.Admin.Admin.Service.TestSetAllowedHost("some-other-host")
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/back-office/api/v1/settings/", nil)
				require.NoError(t, err)
				req.Header.Set("X-Forwarded-Groups", "LimitUpdate")

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				body, err := io.ReadAll(response.Body)
				require.NoError(t, response.Body.Close())
				require.NoError(t, err)

				require.Equal(t, http.StatusForbidden, response.StatusCode)
				require.Contains(t, string(body), "forbidden host")
			})
		})
	})
}

// TestWithAuthNoToken tests when AuthToken config is set to an empty string (disabled authorization).
func TestWithAuthNoToken(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.StaticDir = "ui/build"
				// Disable authorization.
				config.Console.AuthToken = ""
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		address := sat.Admin.Admin.Listener.Addr()
		baseURL := "http://" + address.String()

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("%s/api/projects/%s/apikeys", baseURL, projectID.String()),
			nil,
		)
		require.NoError(t, err)

		// Authorization disabled, so this should fail.
		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusForbidden, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		body, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Contains(t, string(body), admin.AuthorizationNotEnabled)
	})
}
