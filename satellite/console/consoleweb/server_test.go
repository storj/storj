// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
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
			Password: "password",
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

		kind := console.PaidUser
		err = sat.DB.Console().Users().Update(ctx, owner.ID, console.UpdateUserRequest{Kind: &kind})
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

func TestVarPartnerBlocker(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.VarPartners = []string{"partner1"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		makeRequest := func(route, method, token string, shouldForbid bool) {
			urlLink := "http://" + sat.API.Console.Listener.Addr().String() + "/api/v0/payments" + route

			req, err := http.NewRequestWithContext(ctx, method, urlLink, http.NoBody)
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
			if shouldForbid {
				require.Equal(t, http.StatusForbidden, result.StatusCode)
			} else {
				require.Equal(t, http.StatusOK, result.StatusCode)
			}
		}

		for _, i := range []int{1, 2} {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName:  fmt.Sprintf("var user%d", i),
				Email:     fmt.Sprintf("var%d@mail.test", i),
				UserAgent: []byte(fmt.Sprintf("partner%d", i)),
			}, 1)
			require.NoError(t, err)

			tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
			require.NoError(t, err)

			tokenStr := tokenInfo.Token.String()

			makeRequest("/wallet/payments", http.MethodGet, tokenStr, string(user.UserAgent) == "partner1")
			// account setup account endpoint should be allowed even for var partners
			makeRequest("/account", http.MethodPost, tokenStr, false)
		}
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

// TestGenCreateProjectProxy tests that having changed the prefix for the generated API,
// calling it with the new prefix works as expected. And calling an endpoint with the old prefix
// correctly proxies to the new prefix.
func TestGenCreateProjectProxy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.GeneratedAPIEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		restService := sat.Admin.REST.Keys
		consoleService := sat.API.Console.Service

		testU := planet.Uplinks[0].User[sat.ID()]

		user, _, err := consoleService.GetUserByEmailWithUnverified(ctx, testU.Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		dur := time.Hour
		apiKey, _, err := restService.CreateNoAuth(ctx, user.ID, &dur)
		require.NoError(t, err)

		client := http.Client{}
		newApiPrefix := "/public/v1"
		oldApiPrefix := "/api/v0"
		requestGen := func(method string, prefix string, resourcePath string, body map[string]interface{}) (*http.Response, error) {
			url := "http://" + path.Join(sat.API.Console.Listener.Addr().String(), prefix, resourcePath)

			jsonBody, err := json.Marshal(body)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
			require.NoError(t, err)

			req.Header = http.Header{
				"Authorization": []string{"Bearer " + apiKey},
			}

			return client.Do(req)
		}

		name := "a name"
		description := "a description"
		resp, err := requestGen(http.MethodPost, newApiPrefix, "/projects/create", map[string]interface{}{
			"name":        name,
			"description": description,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.NoError(t, resp.Body.Close())

		var createdProject console.ProjectInfo
		require.NoError(t, json.Unmarshal(bodyBytes, &createdProject))
		require.Equal(t, name, createdProject.Name)
		require.Equal(t, description, createdProject.Description)

		// using the old prefix should still work
		name += "2"
		resp, err = requestGen(http.MethodPost, oldApiPrefix, "/projects/create", map[string]interface{}{
			"name":        name,
			"description": description,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		bodyBytes, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.NoError(t, resp.Body.Close())

		require.NoError(t, json.Unmarshal(bodyBytes, &createdProject))
		require.Equal(t, name, createdProject.Name)
		require.Equal(t, description, createdProject.Description)
	})
}

func TestBrandingEndpoint(t *testing.T) {
	var (
		defaultName = "Storj"

		tenantID = "customer1"
		hostName = "customer1.example.com"
		name     = "Customer One"
		logoURLs = map[string]string{
			"full-dark":   "https://customer1.example.com/logo-full-dark.png",
			"full-light":  "https://customer1.example.com/logo-full-light.png",
			"small-dark":  "https://customer1.example.com/logo-small-dark.png",
			"small-light": "https://customer1.example.com/logo-small-light.png",
		}
		faviconURLs = map[string]string{
			"16x16":       "https://customer1.example.com/favicon-16x16.ico",
			"32x32":       "https://customer1.example.com/favicon-32x32.ico",
			"apple-touch": "https://customer1.example.com/apple-touch-icon.png",
		}
		supportURL     = "https://support.customer1.example.com"
		docsURL        = "https://docs.customer1.example.com"
		homepageURL    = "https://customer1.example.com"
		primaryColor   = "#FF0000"
		secondaryColor = "#00FF00"
		colors         = map[string]string{"primary": primaryColor, "secondary": secondaryColor}
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
					tenantID: {
						TenantID:    tenantID,
						HostName:    hostName,
						Name:        name,
						LogoURLs:    logoURLs,
						FaviconURLs: faviconURLs,
						Colors:      colors,
						SupportURL:  supportURL,
						DocsURL:     docsURL,
						HomepageURL: homepageURL,
					},
				}
				config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
					hostName: tenantID,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		addr := sat.API.Console.Listener.Addr().String()
		client := http.DefaultClient

		t.Run("Default Storj branding", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			require.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, defaultName, branding["name"])
		})

		t.Run("Customer branding", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = "customer1.example.com"

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			require.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, name, branding["name"])
			require.Equal(t, supportURL, branding["supportUrl"])
			require.Equal(t, docsURL, branding["docsUrl"])
			require.Equal(t, homepageURL, branding["homepageUrl"])

			gotLogoURLs, ok := branding["logoUrls"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, logoURLs["full-dark"], gotLogoURLs["full-dark"])
			require.Equal(t, logoURLs["full-light"], gotLogoURLs["full-light"])
			require.Equal(t, logoURLs["small-dark"], gotLogoURLs["small-dark"])
			require.Equal(t, logoURLs["small-light"], gotLogoURLs["small-light"])

			gotFaviconURLs, ok := branding["faviconUrls"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, faviconURLs["16x16"], gotFaviconURLs["16x16"])
			require.Equal(t, faviconURLs["32x32"], gotFaviconURLs["32x32"])
			require.Equal(t, faviconURLs["apple-touch"], gotFaviconURLs["apple-touch"])

			gotColors, ok := branding["colors"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, primaryColor, gotColors["primary"])
			require.Equal(t, secondaryColor, gotColors["secondary"])
		})

		t.Run("Unknown hostname returns default branding", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = "unknown.example.com"

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, defaultName, branding["name"])
		})
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
