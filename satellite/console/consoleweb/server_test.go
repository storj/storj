// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
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

		client := http.Client{}

		checkActivationRedirect := func(testMsg, redirectURL string, shouldHaveCookie bool) {
			url := "http://" + sat.API.Console.Listener.Addr().String() + "/activation?token=" + activationToken

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err, testMsg)

			result, err := client.Do(req)
			require.NoError(t, err, testMsg)

			// cookie should be set on successful activation
			hasCookie := false
			for _, c := range result.Cookies() {
				if c.Name == "_tokenKey" {
					hasCookie = true
					break
				}
			}
			require.Equal(t, shouldHaveCookie, hasCookie)

			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode, testMsg)
			require.Equal(t, redirectURL, result.Header.Get("Location"), testMsg)
			require.NoError(t, result.Body.Close(), testMsg)
		}

		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}

		baseURL := "http://" + sat.API.Console.Listener.Addr().String() + "/"
		loginURL := baseURL + "login"

		// successful activation should set cookie and redirect to home page.
		checkActivationRedirect("Activation - Fresh Token", baseURL, true)
		// unsuccessful redirect should not set cookie and redirect to login page.
		checkActivationRedirect("Activation - Used Token", loginURL+"?activated=false", false)
	})
}

func TestInvitedRouting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		invitedEmail := "invited@mail.test"

		owner, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Project Owner",
			Email:    "owner@mail.test",
		}, 1)
		require.NoError(t, err)

		paid := true
		err = sat.DB.Console().Users().Update(ctx, owner.ID, console.UpdateUserRequest{PaidTier: &paid})
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, owner.ID, "Test Project")
		require.NoError(t, err)

		client := http.Client{}

		checkInvitedRedirect := func(testMsg, redirectURL string, token string) {
			url := "http://" + sat.API.Console.Listener.Addr().String() + "/invited?invite=" + token

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err, testMsg)

			result, err := client.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode, testMsg)
			require.Equal(t, redirectURL, result.Header.Get("Location"), testMsg)
			require.NoError(t, result.Body.Close(), testMsg)
		}

		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}

		baseURL := "http://" + sat.API.Console.Listener.Addr().String() + "/"
		loginURL := baseURL + "login"
		invalidURL := loginURL + "?invite_invalid=true"

		tokenInvalidProj, err := service.CreateInviteToken(ctx, project.ID, invitedEmail, time.Now())
		require.NoError(t, err)

		token, err := service.CreateInviteToken(ctx, project.PublicID, invitedEmail, time.Now())
		require.NoError(t, err)

		checkInvitedRedirect("Invited - Invalid projectID", invalidURL, tokenInvalidProj)

		checkInvitedRedirect("Invited - User not invited", invalidURL, token)

		ownerCtx, err := sat.UserContext(ctx, owner.ID)
		require.NoError(t, err)
		_, err = service.InviteNewProjectMember(ownerCtx, project.ID, invitedEmail)
		require.NoError(t, err)

		// Valid invite for nonexistent user should redirect to registration page with
		// query parameters containing invitation information.
		params := "email=invited%40mail.test&inviter_email=owner%40mail.test"
		checkInvitedRedirect("Invited - Nonexistent user", baseURL+"signup?"+params, token)

		_, err = sat.AddUser(ctx, console.CreateUser{
			FullName: "Invited User",
			Email:    invitedEmail,
		}, 1)
		require.NoError(t, err)

		// valid invite should redirect to login page with email.
		checkInvitedRedirect("Invited - User invited", loginURL+"?email=invited%40mail.test", token)
	})
}

func TestUserIDRateLimiter(t *testing.T) {
	numLimits := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.NumLimits = numLimits
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		applyCouponStatus := func(token string) int {
			urlLink := "http://" + sat.API.Console.Listener.Addr().String() + "/api/v0/payments/coupon/apply"

			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, urlLink, bytes.NewBufferString("PROMO_CODE"))
			require.NoError(t, err)

			req.AddCookie(&http.Cookie{
				Name:    "_tokenKey",
				Path:    "/",
				Value:   token,
				Expires: time.Now().AddDate(0, 0, 1),
			})

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, result.Body.Close())

			return result.StatusCode
		}

		var firstToken string
		for userNum := 1; userNum <= numLimits+1; userNum++ {
			t.Run(fmt.Sprintf("TestUserIDRateLimit_%d", userNum), func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: fmt.Sprintf("Test User %d", userNum),
					Email:    fmt.Sprintf("test%d@mail.test", userNum),
				}, 1)
				require.NoError(t, err)

				// sat.AddUser sets password to full name.
				tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
				require.NoError(t, err)

				tokenStr := tokenInfo.Token.String()

				if userNum == 1 {
					firstToken = tokenStr
				}

				// Expect burst number of successes.
				for burstNum := 0; burstNum < sat.Config.Console.RateLimit.Burst; burstNum++ {
					require.NotEqual(t, http.StatusTooManyRequests, applyCouponStatus(tokenStr))
				}

				// Expect failure.
				require.Equal(t, http.StatusTooManyRequests, applyCouponStatus(tokenStr))
			})
		}

		// Expect original user to work again because numLimits == 2.
		for burstNum := 0; burstNum < sat.Config.Console.RateLimit.Burst; burstNum++ {
			require.NotEqual(t, http.StatusTooManyRequests, applyCouponStatus(firstToken))
		}
		require.Equal(t, http.StatusTooManyRequests, applyCouponStatus(firstToken))
	})
}

func TestConsoleBackendWithDisabledFrontEnd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.FrontendEnable = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiAddr := planet.Satellites[0].API.Console.Listener.Addr().String()
		uiAddr := planet.Satellites[0].UI.Console.Listener.Addr().String()

		testEndpoint(ctx, t, apiAddr, "/", http.StatusNotFound)
		testEndpoint(ctx, t, apiAddr, "/static/", http.StatusNotFound)

		testEndpoint(ctx, t, uiAddr, "/", http.StatusOK)
		testEndpoint(ctx, t, uiAddr, "/static/", http.StatusOK)
	})
}

func TestConsoleBackendWithEnabledFrontEnd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiAddr := planet.Satellites[0].API.Console.Listener.Addr().String()

		testEndpoint(ctx, t, apiAddr, "/", http.StatusOK)
		testEndpoint(ctx, t, apiAddr, "/static/", http.StatusOK)
	})
}

func testEndpoint(ctx context.Context, t *testing.T, addr, endpoint string, expectedStatus int) {
	client := http.Client{}
	url := "http://" + addr + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(t, err)

	result, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, expectedStatus, result.StatusCode)
	require.NoError(t, result.Body.Close())
}
