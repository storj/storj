// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/payments/stripe"
)

func doRequestWithAuth(
	ctx context.Context,
	t *testing.T,
	sat *testplanet.Satellite,
	user *console.User,
	method string,
	endpoint string,
	body io.Reader,
) (responseBody []byte, statusCode int, err error) {
	fullURL := "http://" + sat.API.Console.Listener.Addr().String() + "/api/v0/" + endpoint

	tokenInfo, err := sat.API.Console.Service.GenerateSessionToken(ctx, console.SessionTokenRequest{
		UserID:          user.ID,
		Email:           user.Email,
		IP:              "",
		UserAgent:       "",
		AnonymousID:     "",
		CustomDuration:  nil,
		HubspotObjectID: user.HubspotObjectID,
	})
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, 0, err
	}

	req.AddCookie(&http.Cookie{
		Name:    "_tokenKey",
		Path:    "/",
		Value:   tokenInfo.Token.String(),
		Expires: time.Now().AddDate(0, 0, 1),
	})

	result, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		err = errs.Combine(err, result.Body.Close())
	}()

	responseBody, err = io.ReadAll(result.Body)
	if err != nil {
		return nil, 0, err
	}

	return responseBody, result.StatusCode, nil
}

func TestAuth_Register(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
				config.Mail.AuthType = "nomail"
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
			func() {
				registerData := struct {
					FullName        string `json:"fullName"`
					ShortName       string `json:"shortName"`
					Email           string `json:"email"`
					Partner         string `json:"partner"`
					UserAgent       string `json:"userAgent"`
					Password        string `json:"password"`
					SecretInput     string `json:"secret"`
					ReferrerUserID  string `json:"referrerUserId"`
					IsProfessional  bool   `json:"isProfessional"`
					Position        string `json:"Position"`
					CompanyName     string `json:"CompanyName"`
					EmployeeCount   string `json:"EmployeeCount"`
					SignupPromoCode string `json:"signupPromoCode"`
				}{
					FullName:        "testuser" + strconv.Itoa(i),
					ShortName:       "test",
					Email:           "user@test" + strconv.Itoa(i) + ".test",
					Partner:         test.Partner,
					Password:        "password",
					IsProfessional:  true,
					Position:        "testposition",
					CompanyName:     "companytestname",
					EmployeeCount:   "0",
					SignupPromoCode: "STORJ50",
				}

				jsonBody, err := json.Marshal(registerData)
				require.NoError(t, err)

				url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				result, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer func() {
					err = result.Body.Close()
					require.NoError(t, err)
				}()
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Len(t, planet.Satellites, 1)
				// this works only because we configured 'nomail' above. Mail send simulator won't click to activation link.
				_, users, err := planet.Satellites[0].API.Console.Service.GetUserByEmailWithUnverified(ctx, registerData.Email)
				require.NoError(t, err)
				require.Len(t, users, 1)
				require.Equal(t, []byte(test.Partner), users[0].UserAgent)
			}()
		}
	})
}

func TestAuth_ChangeEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usrLogin := planet.Uplinks[0].User[sat.ID()]

		user, _, err := service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		doRequest := func(step console.AccountActionStep, data string) (responseBody []byte, status int) {
			body := &consoleapi.AccountActionData{
				Step: step,
				Data: data,
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)
			buf := bytes.NewBuffer(bodyBytes)

			responseBody, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, "auth/change-email", buf)
			require.NoError(t, err)

			return responseBody, status
		}

		_, status := doRequest(0, usrLogin.Password)
		require.Equal(t, http.StatusBadRequest, status)

		_, status = doRequest(console.VerifyAccountPasswordStep, "")
		require.Equal(t, http.StatusBadRequest, status)
	})
}

func TestAuth_InvalidateSession(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		sessionsDB := sat.DB.Console().WebappSessions()

		user, _, err := service.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		id, err := uuid.New()
		require.NoError(t, err)

		session, err := sessionsDB.Create(ctx, id, user.ID, "", "test", time.Now().Add(time.Hour))
		require.NoError(t, err)

		traitor, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Invalidate Session",
			Email:    "invalidate_session@mail.test",
		}, 1)
		require.NoError(t, err)

		_, status, err := doRequestWithAuth(ctx, t, sat, traitor, http.MethodPost, "auth/invalidate-session/"+session.ID.String(), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, status)

		_, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, "auth/invalidate-session/"+session.ID.String(), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		_, err = sessionsDB.GetBySessionID(ctx, session.ID)
		require.Error(t, err)
	})
}

func TestAuth_UpdateUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, _, err := service.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		newName := "new name"
		shortName := "NN"
		err = service.UpdateAccount(userCtx, newName, shortName)
		require.NoError(t, err)

		user, _, err = service.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.Equal(t, newName, user.FullName)
		require.Equal(t, shortName, user.ShortName)

		err = service.UpdateAccount(userCtx, newName, "")
		require.NoError(t, err)

		user, _, err = service.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.Equal(t, newName, user.FullName)
		require.Equal(t, "", user.ShortName)

		// empty full name not allowed
		err = service.UpdateAccount(userCtx, "", shortName)
		require.Error(t, err)
		require.True(t, console.ErrValidation.Has(err))
	})
}

func TestAuth_RegisterWithInvitation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
				config.Mail.AuthType = "nomail"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i := 0; i < 2; i++ {
			email := fmt.Sprintf("user%d@test.test", i)
			// test with nil and non-nil inviter ID to make sure nil pointer dereference doesn't occur
			// since nil ID is technically possible
			var inviter *uuid.UUID
			if i == 1 {
				id := planet.Uplinks[0].Projects[0].Owner.ID
				inviter = &id
			}
			_, err := planet.Satellites[0].API.DB.Console().ProjectInvitations().Upsert(ctx, &console.ProjectInvitation{
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Email:     email,
				InviterID: inviter,
			})
			require.NoError(t, err)

			registerData := struct {
				FullName        string `json:"fullName"`
				ShortName       string `json:"shortName"`
				Email           string `json:"email"`
				Partner         string `json:"partner"`
				UserAgent       string `json:"userAgent"`
				Password        string `json:"password"`
				SecretInput     string `json:"secret"`
				ReferrerUserID  string `json:"referrerUserId"`
				IsProfessional  bool   `json:"isProfessional"`
				Position        string `json:"Position"`
				CompanyName     string `json:"CompanyName"`
				EmployeeCount   string `json:"EmployeeCount"`
				SignupPromoCode string `json:"signupPromoCode"`
			}{
				FullName:        "testuser",
				ShortName:       "test",
				Email:           email,
				Password:        "password",
				IsProfessional:  true,
				Position:        "testposition",
				CompanyName:     "companytestname",
				EmployeeCount:   "0",
				SignupPromoCode: "STORJ50",
			}

			jsonBody, err := json.Marshal(registerData)
			require.NoError(t, err)

			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, result.Body.Close())
			require.Equal(t, http.StatusOK, result.StatusCode)
			require.Len(t, planet.Satellites, 1)
			// this works only because we configured 'nomail' above. Mail send simulator won't click to activation link.
			_, users, err := planet.Satellites[0].API.Console.Service.GetUserByEmailWithUnverified(ctx, registerData.Email)
			require.NoError(t, err)
			require.Len(t, users, 1)
		}
	})
}

func TestTokenByAPIKeyEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		restKeys := satellite.API.Console.RestKeys

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		expires := 5 * time.Hour
		apiKey, _, err := restKeys.CreateNoAuth(ctx, user.ID, &expires)
		require.NoError(t, err)
		require.NotEmpty(t, apiKey)

		url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/token-by-api-key"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, response)
		require.NoError(t, response.Body.Close())

		cookies := response.Cookies()
		require.NoError(t, err)
		require.Len(t, cookies, 1)
		require.Equal(t, "_tokenKey", cookies[0].Name)
		require.NotEmpty(t, cookies[0].Value)
	})
}

func TestSsoUserLoginWithPassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.SSO.Enabled = false
				config.SSO.MockSso = true
				config.SSO.MockEmail = "some@mail.test"
				config.SSO.OidcProviderInfos = sso.OidcProviderInfos{
					Values: map[string]sso.OidcProviderInfo{
						"fakeProvider": {},
					},
				}
				reg := regexp.MustCompile(`@mail.test`)
				require.NotNil(t, reg)
				config.SSO.EmailProviderMappings = sso.EmailProviderMappings{
					Values: map[string]regexp.Regexp{
						"fakeProvider": *reg,
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		require.NoError(t, satellite.API.Console.Service.UpdateExternalID(ctx, user, "test:1234"))

		login := func(expectedCode int) {
			body := console.AuthUser{
				Email:    user.Email,
				Password: user.FullName,
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)
			buf := bytes.NewBuffer(bodyBytes)

			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/token"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
			require.NoError(t, err)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NotEmpty(t, response)
			require.Equal(t, expectedCode, response.StatusCode)

			responseBody, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			if expectedCode == http.StatusOK {
				require.Contains(t, string(responseBody), "token")
			} else {
				require.NotContains(t, string(responseBody), "token")
			}
			require.NoError(t, response.Body.Close())
		}

		login(http.StatusOK)

		// enable SSO
		ssoService := sso.NewService(
			satellite.ConsoleURL(),
			satellite.API.Console.AuthTokens,
			satellite.Config.SSO,
		)
		err = ssoService.Initialize(ctx)
		require.NoError(t, err)
		satellite.API.Console.Service.TestToggleSsoEnabled(true, ssoService)

		login(http.StatusForbidden)

		// remove user's provider from config
		ssoConfig := satellite.Config.SSO
		ssoConfig.EmailProviderMappings = sso.EmailProviderMappings{}
		ssoService = sso.NewService(
			satellite.ConsoleURL(),
			satellite.API.Console.AuthTokens,
			ssoConfig,
		)
		err = ssoService.Initialize(ctx)
		require.NoError(t, err)
		satellite.API.Console.Service.TestToggleSsoEnabled(true, ssoService)

		// if sso is enabled but user's provider is no longer supported,
		// allow user to login with password.
		login(http.StatusOK)
	})
}

func TestSsoUserForgotPassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.SSO.Enabled = false
				config.SSO.MockSso = true
				config.SSO.MockEmail = "some@mail.test"
				config.SSO.OidcProviderInfos = sso.OidcProviderInfos{
					Values: map[string]sso.OidcProviderInfo{
						"fakeProvider": {},
					},
				}
				reg := regexp.MustCompile(`@mail.test`)
				require.NotNil(t, reg)
				config.SSO.EmailProviderMappings = sso.EmailProviderMappings{
					Values: map[string]regexp.Regexp{
						"fakeProvider": *reg,
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		require.NoError(t, satellite.API.Console.Service.UpdateExternalID(ctx, user, "test:1234"))

		body := console.AuthUser{
			Email:    user.Email,
			Password: user.FullName,
		}

		forgotPassword := func(expectedCode int) {
			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)
			buf := bytes.NewBuffer(bodyBytes)

			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/forgot-password"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
			require.NoError(t, err)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, expectedCode, response.StatusCode)
			require.NoError(t, response.Body.Close())
		}

		forgotPassword(http.StatusOK)

		token, err := satellite.DB.Console().ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, token)

		err = satellite.DB.Console().ResetPasswordTokens().Delete(ctx, token.Secret)
		require.NoError(t, err)

		// enable SSO
		ssoService := sso.NewService(
			satellite.ConsoleURL(),
			satellite.API.Console.AuthTokens,
			satellite.Config.SSO,
		)
		err = ssoService.Initialize(ctx)
		require.NoError(t, err)
		satellite.API.Console.Service.TestToggleSsoEnabled(true, ssoService)

		forgotPassword(http.StatusForbidden)

		token, err = satellite.DB.Console().ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
		require.Equal(t, sql.ErrNoRows, err)
		require.Nil(t, token)

		// remove user's provider from config
		ssoConfig := satellite.Config.SSO
		ssoConfig.EmailProviderMappings = sso.EmailProviderMappings{}
		ssoService = sso.NewService(
			satellite.ConsoleURL(),
			satellite.API.Console.AuthTokens,
			ssoConfig,
		)
		err = ssoService.Initialize(ctx)
		require.NoError(t, err)
		satellite.API.Console.Service.TestToggleSsoEnabled(true, ssoService)

		// if sso is enabled but user's provider is no longer supported,
		// allow user to reset password.
		forgotPassword(http.StatusOK)
	})
}

func TestMFAEndpoints(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.Burst = 20
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "MFA Test User",
			Email:    "mfauser@mail.test",
		}, 1)
		require.NoError(t, err)

		tokenInfo, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)
		require.NotEmpty(t, tokenInfo.Token)

		type data struct {
			Passcode     string `json:"passcode"`
			RecoveryCode string `json:"recoveryCode"`
		}

		doRequest := func(endpointSuffix string, passcode string, recoveryCode string) (responseBody []byte, status int) {
			body := &data{
				Passcode:     passcode,
				RecoveryCode: recoveryCode,
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)
			buf := bytes.NewBuffer(bodyBytes)

			responseBody, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, "auth/mfa"+endpointSuffix, buf)
			require.NoError(t, err)

			return responseBody, status
		}

		// Expect failure due to not having generated a secret key.
		_, status := doRequest("/enable", "123456", "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect success when generating a secret key.
		body, status := doRequest("/generate-secret-key", "", "")
		require.Equal(t, http.StatusOK, status)

		var key string
		err = json.Unmarshal(body, &key)
		require.NoError(t, err)

		// Expect failure due to prodiving empty passcode.
		_, status = doRequest("/enable", "", "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect failure due to providing invalid passcode.
		badCode, err := console.NewMFAPasscode(key, time.Now().Add(time.Hour))
		require.NoError(t, err)
		_, status = doRequest("/enable", badCode, "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect success when providing valid passcode.
		goodCode, err := console.NewMFAPasscode(key, time.Now())
		require.NoError(t, err)
		body, status = doRequest("/enable", goodCode, "")
		require.Equal(t, http.StatusOK, status)

		var codes []string
		err = json.Unmarshal(body, &codes)
		require.NoError(t, err)
		require.Len(t, codes, console.MFARecoveryCodeCount)

		// Expect no token due to missing passcode.
		newToken, err := sat.API.Console.Service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.True(t, console.ErrMFAMissing.Has(err))
		require.Empty(t, newToken)

		// Expect token when providing valid passcode.
		newToken, err = sat.API.Console.Service.Token(ctx, console.AuthUser{
			Email:       user.Email,
			Password:    user.FullName,
			MFAPasscode: goodCode,
		})
		require.NoError(t, err)
		require.NotEmpty(t, newToken)

		// Expect no token when providing invalid recovery code.
		newToken, err = sat.API.Console.Service.Token(ctx, console.AuthUser{
			Email:           user.Email,
			Password:        user.FullName,
			MFARecoveryCode: "BADCODE",
		})
		require.True(t, console.ErrMFARecoveryCode.Has(err))
		require.Empty(t, newToken)

		for _, code := range codes {
			opts := console.AuthUser{
				Email:           user.Email,
				Password:        user.FullName,
				MFARecoveryCode: code,
			}

			// Expect token when providing valid recovery code.
			newToken, err = sat.API.Console.Service.Token(ctx, opts)
			require.NoError(t, err)
			require.NotEmpty(t, newToken)

			// Expect error when providing expired recovery code.
			newToken, err = sat.API.Console.Service.Token(ctx, opts)
			require.True(t, console.ErrMFARecoveryCode.Has(err))
			require.Empty(t, newToken)
		}

		// Expect failure due to disabling MFA with no passcode.
		_, status = doRequest("/disable", "", "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect failure due to disabling MFA with invalid passcode.
		_, status = doRequest("/disable", badCode, "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect failure when regenerating without providing either passcode or recovery code.
		_, status = doRequest("/regenerate-recovery-codes", "", "")
		require.Equal(t, http.StatusBadRequest, status)

		// Expect failure when regenerating when providing both passcode and recovery code.
		_, status = doRequest("/regenerate-recovery-codes", goodCode, codes[0])
		require.Equal(t, http.StatusConflict, status)

		body, _ = doRequest("/regenerate-recovery-codes", goodCode, "")
		err = json.Unmarshal(body, &codes)
		require.NoError(t, err)

		// Expect failure when disabling due to providing both passcode and recovery code.
		_, status = doRequest("/disable", goodCode, codes[0])
		require.Equal(t, http.StatusConflict, status)

		// Expect success when disabling MFA with valid passcode.
		_, status = doRequest("/disable", goodCode, "")
		require.Equal(t, http.StatusOK, status)

		// Expect success when disabling MFA with valid recovery code.
		body, _ = doRequest("/generate-secret-key", "", "")
		err = json.Unmarshal(body, &key)
		require.NoError(t, err)

		goodCode, err = console.NewMFAPasscode(key, time.Now())
		require.NoError(t, err)
		body, _ = doRequest("/enable", goodCode, "")
		err = json.Unmarshal(body, &codes)
		require.NoError(t, err)

		_, status = doRequest("/disable", "", codes[0])
		require.Equal(t, http.StatusOK, status)
	})
}

func TestResetPasswordEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		newPass := user.FullName

		getNewResetToken := func() *console.ResetPasswordToken {
			token, err := sat.DB.Console().ResetPasswordTokens().Create(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, token)
			return token
		}

		tryPasswordReset := func(tokenStr, password, mfaPasscode, mfaRecoveryCode string) (int, bool) {
			url := sat.ConsoleURL() + "/api/v0/auth/reset-password"

			bodyBytes, err := json.Marshal(map[string]string{
				"password":        password,
				"token":           tokenStr,
				"mfaPasscode":     mfaPasscode,
				"mfaRecoveryCode": mfaRecoveryCode,
			})
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			var response struct {
				Code string `json:"code"`
			}

			if result.ContentLength > 0 {
				err = json.NewDecoder(result.Body).Decode(&response)
				require.NoError(t, err)
			}

			require.NoError(t, result.Body.Close())

			return result.StatusCode, response.Code == "mfa_required"
		}

		token := getNewResetToken()

		status, mfaError := tryPasswordReset("badToken", newPass, "", "")
		require.Equal(t, http.StatusUnauthorized, status)
		require.False(t, mfaError)

		status, mfaError = tryPasswordReset(token.Secret.String(), "bad", "", "")
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, mfaError)

		status, mfaError = tryPasswordReset(token.Secret.String(), string(testrand.RandAlphaNumeric(129)), "", "")
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, mfaError)

		status, mfaError = tryPasswordReset(token.Secret.String(), newPass, "", "")
		require.Equal(t, http.StatusOK, status)
		require.False(t, mfaError)
		token = getNewResetToken()

		// Enable MFA.
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		key, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		passcode, err := console.NewMFAPasscode(key, token.CreatedAt)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, passcode, token.CreatedAt)
		require.NoError(t, err)

		status, mfaError = tryPasswordReset(token.Secret.String(), newPass, "", "")
		require.Equal(t, http.StatusBadRequest, status)
		require.True(t, mfaError)

		status, mfaError = tryPasswordReset(token.Secret.String(), newPass, "", "")
		require.Equal(t, http.StatusBadRequest, status)
		require.True(t, mfaError)
	})
}

type EmailVerifier struct {
	Data    consoleapi.ContextChannel
	Context context.Context
}

func (v *EmailVerifier) SendEmail(ctx context.Context, msg *post.Message) error {
	body := ""
	for _, part := range msg.Parts {
		body += part.Content
	}
	return v.Data.Send(v.Context, body)
}

func (v *EmailVerifier) FromAddress() post.Address {
	return post.Address{}
}

func TestRegistrationEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		email := "test@mail.test"
		jsonBody, err := json.Marshal(map[string]interface{}{
			"fullName":  "Test User",
			"shortName": "Test",
			"email":     email,
			"password":  "password",
		})
		require.NoError(t, err)

		register := func() {
			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, result.StatusCode)
			require.NoError(t, result.Body.Close())
		}

		sender := &EmailVerifier{Context: ctx}
		sat.API.Mail.Service.Sender = sender

		// Registration attempts using new e-mail address should send activation e-mail.
		register()
		body, err := sender.Data.Get(ctx)
		require.NoError(t, err)
		if sat.Config.Console.SignupActivationCodeEnabled {
			_, users, err := sat.DB.Console().Users().GetByEmailAndTenantWithUnverified(ctx, email, nil)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Contains(t, body, users[0].ActivationCode)
		} else {
			require.Contains(t, body, "/activation")
		}

		// Registration attempts using existing but unverified e-mail address should send activation e-mail.
		register()
		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		if sat.Config.Console.SignupActivationCodeEnabled {
			_, users, err := sat.DB.Console().Users().GetByEmailAndTenantWithUnverified(ctx, email, nil)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Contains(t, body, users[0].ActivationCode)
		} else {
			require.Contains(t, body, "/activation")
		}

		// Registration attempts using existing and verified e-mail address should send account already exists e-mail.
		_, users, err := sat.DB.Console().Users().GetByEmailAndTenantWithUnverified(ctx, email, nil)
		require.NoError(t, err)

		users[0].Status = console.Active
		require.NoError(t, sat.DB.Console().Users().Update(ctx, users[0].ID, console.UpdateUserRequest{
			Status: &users[0].Status,
		}))

		register()
		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "/login")
		require.Contains(t, body, "/forgot-password")
	})
}

func TestRegistrationEmail_CodeEnabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SignupActivationCodeEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		email := "test@mail.test"

		sender := &EmailVerifier{Context: ctx}
		sat.API.Mail.Service.Sender = sender

		jsonBody, err := json.Marshal(map[string]interface{}{
			"fullName":  "Test User",
			"shortName": "Test",
			"email":     email,
			"password":  "password",
		})
		require.NoError(t, err)

		signupURL := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, signupURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())

		body, err := sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "code")
	})
}

func TestIncreaseLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		proUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test Pro User",
			Email:    "testpro@mail.test",
		}, 1)
		require.NoError(t, err)

		proUser.Kind = console.PaidUser
		require.NoError(t, sat.DB.Console().Users().Update(ctx, proUser.ID, console.UpdateUserRequest{Kind: &proUser.Kind}))

		freeUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test Free User",
			Email:    "testfree@mail.test",
		}, 1)
		require.NoError(t, err)

		endpoint := "auth/limit-increase"

		tests := []struct {
			user           *console.User
			input          string
			expectedStatus int
		}{
			{ // Happy path
				user: proUser, input: "10", expectedStatus: http.StatusOK,
			},
			{ // non-integer input
				user: proUser, input: "1000 projects please", expectedStatus: http.StatusBadRequest,
			},
			{ // other non-integer input
				user: proUser, input: "7.5", expectedStatus: http.StatusBadRequest,
			},
			{ // another non-integer input
				user: proUser, input: "7.0", expectedStatus: http.StatusBadRequest,
			},
			{ // requested limit zero
				user: proUser, input: "0", expectedStatus: http.StatusBadRequest,
			},
			{ // requested limit negative
				user: proUser, input: "-1", expectedStatus: http.StatusBadRequest,
			},
			{ // requested limit not greater than current limit
				user: proUser, input: "1", expectedStatus: http.StatusBadRequest,
			},
			{ // free tier user
				user: freeUser, input: "10", expectedStatus: http.StatusPaymentRequired,
			},
		}

		for _, tt := range tests {
			_, status, err := doRequestWithAuth(ctx, t, sat, tt.user, http.MethodPatch, endpoint, bytes.NewBufferString(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, status)
		}
	})
}

func TestResendActivationEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersRepo := sat.DB.Console().Users()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		resendEmail := func() {
			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/resend-email"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(fmt.Sprintf(`{"email":"%s"}`, user.Email)))
			require.NoError(t, err)

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, result.Body.Close())
			require.Equal(t, http.StatusOK, result.StatusCode)
		}

		sender := &EmailVerifier{Context: ctx}
		sat.API.Mail.Service.Sender = sender

		// Expect password reset e-mail to be sent when using verified e-mail address.
		resendEmail()
		body, err := sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "/password-recovery")

		// Expect activation e-mail to be sent when using unverified e-mail address.
		user.Status = console.Inactive
		require.NoError(t, usersRepo.Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &user.Status,
		}))

		resendEmail()
		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		if sat.Config.Console.SignupActivationCodeEnabled {
			_, users, err := sat.DB.Console().Users().GetByEmailAndTenantWithUnverified(ctx, user.Email, nil)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Contains(t, body, users[0].ActivationCode)
		} else {
			require.Contains(t, body, "/activation")
		}
	})
}

func TestResendActivationEmail_CodeEnabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SignupActivationCodeEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersRepo := sat.DB.Console().Users()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		// Expect activation e-mail to be sent when using unverified e-mail address.
		user.Status = console.Inactive
		require.NoError(t, usersRepo.Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &user.Status,
		}))

		sender := &EmailVerifier{Context: ctx}
		sat.API.Mail.Service.Sender = sender

		resendURL := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/resend-email"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendURL, bytes.NewBufferString(fmt.Sprintf(`{"email":"%s"}`, user.Email)))
		require.NoError(t, err)

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err := sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "code")

		regex := regexp.MustCompile(`(\d{6})\s*<\/h2>`)
		code := strings.Replace(regex.FindString(body.(string)), "</h2>", "", 1)
		code = strings.TrimSpace(code)
		require.Contains(t, body, code)

		// resending should send a new code.
		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "code")

		newCode := strings.Replace(regex.FindString(body.(string)), "</h2>", "", 1)
		newCode = strings.TrimSpace(newCode)
		require.NotEqual(t, code, newCode)
	})
}

func TestAuth_Register_ShortPartnerOrPromo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		type registerData struct {
			FullName        string `json:"fullName"`
			Email           string `json:"email"`
			Password        string `json:"password"`
			Partner         string `json:"partner"`
			SignupPromoCode string `json:"signupPromoCode"`
		}

		reqURL := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"

		jsonBodyCorrect, err := json.Marshal(&registerData{
			FullName:        "test",
			Email:           "user@mail.test",
			Password:        "password",
			Partner:         string(testrand.RandAlphaNumeric(100)),
			SignupPromoCode: string(testrand.RandAlphaNumeric(100)),
		})
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBodyCorrect))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		err = result.Body.Close()
		require.NoError(t, err)

		jsonBodyPartnerInvalid, err := json.Marshal(&registerData{
			FullName: "test",
			Email:    "user1@mail.test",
			Password: "password",
			Partner:  string(testrand.RandAlphaNumeric(101)),
		})
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBodyPartnerInvalid))
		require.NoError(t, err)

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, result.StatusCode)

		err = result.Body.Close()
		require.NoError(t, err)

		jsonBodyPromoInvalid, err := json.Marshal(&registerData{
			FullName:        "test",
			Email:           "user1@mail.test",
			Password:        "password",
			SignupPromoCode: string(testrand.RandAlphaNumeric(101)),
		})
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBodyPromoInvalid))
		require.NoError(t, err)

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, result.StatusCode)

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}

func TestAuth_Register_PasswordLength(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i, tt := range []struct {
			Name   string
			Length int
			Ok     bool
		}{
			{"Length below minimum must be rejected", 6, false},
			{"Length as minimum must be accepted", 8, true},
			{"Length as maximum must be accepted", 64, true},
			{"Length above maximum must be rejected", 65, false},
		} {
			tt := tt
			t.Run(tt.Name, func(t *testing.T) {
				jsonBody, err := json.Marshal(map[string]string{
					"fullName": "test",
					"email":    "user" + strconv.Itoa(i) + "@mail.test",
					"password": string(testrand.RandAlphaNumeric(tt.Length)),
				})
				require.NoError(t, err)

				url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
				require.NoError(t, err)

				result, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				err = result.Body.Close()
				require.NoError(t, err)

				status := http.StatusOK
				if !tt.Ok {
					status = http.StatusBadRequest
				}
				require.Equal(t, status, result.StatusCode)
			})
		}
	})
}

func TestAccountActivationWithCode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SignupActivationCodeEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		email := "test@mail.test"

		sender := &EmailVerifier{Context: ctx}
		sat.API.Mail.Service.Sender = sender

		jsonBody, err := json.Marshal(map[string]interface{}{
			"fullName":  "Test User",
			"shortName": "Test",
			"email":     email,
			"password":  "password",
		})
		require.NoError(t, err)

		signupURL := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/register"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, signupURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())

		body, err := sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "code")

		regex := regexp.MustCompile(`(\d{6})\s*<\/h2>`)
		code := strings.Replace(regex.FindString(body.(string)), "</h2>", "", 1)
		code = strings.TrimSpace(code)
		require.Contains(t, body, code)

		signupID := result.Header.Get("x-request-id")

		activateURL := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/code-activation"
		jsonBody, err = json.Marshal(map[string]interface{}{
			"email":    email,
			"code":     code,
			"signupId": "wrong id",
		})
		require.NoError(t, err)
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, activateURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, http.StatusUnauthorized, result.StatusCode)
		require.NoError(t, result.Body.Close())

		jsonBody, err = json.Marshal(map[string]interface{}{
			"email":    email,
			"code":     code,
			"signupId": signupID,
		})
		require.NoError(t, err)
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, activateURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())

		cookies := result.Cookies()
		require.NoError(t, err)
		require.Len(t, cookies, 1)
		require.Equal(t, "_tokenKey", cookies[0].Name)
		require.NotEmpty(t, cookies[0].Value)

		// trying to activate an activated account should send account already exists email
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, activateURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, http.StatusUnauthorized, result.StatusCode)
		require.NoError(t, result.Body.Close())

		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "/login")
		require.Contains(t, body, "/forgot-password")

		// trying to activate an account that is not "inactive" or "active" should result in an error
		user, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, email, nil)
		require.NoError(t, err)
		newStatus := console.PendingDeletion
		err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &newStatus,
		})
		require.NoError(t, err)
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, activateURL, bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, http.StatusNotFound, result.StatusCode)
		require.NoError(t, result.Body.Close())
	})
}

func TestAuth_SetupAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		ptr := func(s string) *string {
			return &s
		}

		tests := []console.SetUpAccountRequest{
			{
				FirstName:      ptr("Frodo"),
				LastName:       ptr("Baggins"),
				IsProfessional: true,
				Position:       ptr("Ringbearer"),
				CompanyName:    ptr("The Fellowship"),
				EmployeeCount:  ptr("9"), // subject to change
			},
			{
				FullName:       ptr("Bilbo Baggins"),
				IsProfessional: false,
			},
		}

		for i, tt := range tests {
			regToken, err := sat.API.Console.Service.CreateRegToken(ctx, 1)
			require.NoError(t, err)
			user, err := sat.API.Console.Service.CreateUser(ctx, console.CreateUser{
				FullName: "should be overwritten by setup",
				Email:    fmt.Sprintf("test%d@storj.test", i),
				Password: "password",
			}, regToken.Secret)
			require.NoError(t, err)
			activationToken, err := sat.API.Console.Service.GenerateActivationToken(ctx, user.ID, user.Email)
			require.NoError(t, err)
			_, err = sat.API.Console.Service.ActivateAccount(ctx, activationToken)
			require.NoError(t, err)

			payload, err := json.Marshal(tt)
			require.NoError(t, err)
			_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodPatch, "auth/account/setup", bytes.NewBuffer(payload))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			userAfterSetup, err := sat.DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)

			if tt.IsProfessional {
				require.Equal(t, *tt.FirstName+" "+*tt.LastName, userAfterSetup.FullName)
			} else {
				require.Equal(t, *tt.FullName, userAfterSetup.FullName)
			}
			require.Equal(t, tt.IsProfessional, userAfterSetup.IsProfessional)
			if tt.Position != nil {
				require.Equal(t, *tt.Position, userAfterSetup.Position)
			} else {
				require.Equal(t, "", userAfterSetup.Position)
			}
			if tt.CompanyName != nil {
				require.Equal(t, *tt.CompanyName, userAfterSetup.CompanyName)
			} else {
				require.Equal(t, "", userAfterSetup.CompanyName)
			}
			if tt.EmployeeCount != nil {
				require.Equal(t, *tt.EmployeeCount, userAfterSetup.EmployeeCount)
			} else {
				require.Equal(t, "", userAfterSetup.EmployeeCount)
			}
		}
	})
}

func TestSsoMethods(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		createUserFn := func(email string) *console.User {
			regToken, err := sat.API.Console.Service.CreateRegToken(ctx, 1)
			require.NoError(t, err)
			user, err := sat.API.Console.Service.CreateUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    email,
				Password: "password",
			}, regToken.Secret)
			require.NoError(t, err)
			return user
		}

		provider := "provider"
		getExternalID := func(sub string) string {
			return fmt.Sprintf("%s:%s", provider, sub)
		}

		createUser1 := console.CreateSsoUser{
			ExternalId: getExternalID("test"),
			Email:      "test@mail.test",
			FullName:   "Test User",
		}
		user, err := service.CreateSsoUser(ctx, createUser1)
		require.NoError(t, err)
		require.Equal(t, &createUser1.ExternalId, user.ExternalID)
		require.Equal(t, createUser1.Email, user.Email)
		require.Equal(t, createUser1.FullName, user.FullName)
		require.Equal(t, console.Active, user.Status)
		require.Empty(t, user.PasswordHash)

		user = createUserFn("test2@mail.test")
		require.Equal(t, console.Inactive, user.Status)
		require.Nil(t, user.ExternalID)

		// creating a sso user with the same email should return the existing user
		// associating it with the external ID
		createUser2 := console.CreateSsoUser{
			ExternalId: getExternalID("testy"),
			Email:      user.Email,
			FullName:   user.FullName,
		}
		user, err = service.CreateSsoUser(ctx, createUser2)
		require.NoError(t, err)
		require.Equal(t, &createUser2.ExternalId, user.ExternalID)
		require.Equal(t, createUser2.Email, user.Email)
		require.Equal(t, createUser2.FullName, user.FullName)
		require.Equal(t, console.Active, user.Status)
		require.NotEmpty(t, user.PasswordHash)

		user = createUserFn("test3@mail.test")
		require.NoError(t, err)
		require.Equal(t, console.Inactive, user.Status)
		require.Nil(t, user.ExternalID)

		err = service.UpdateExternalID(ctx, user, getExternalID("testy2"))
		require.NoError(t, err)

		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, getExternalID("testy2"), *user.ExternalID)
		require.Equal(t, console.Active, user.Status)

		// GetUserForSsoAuth should return the user if the external ID matches
		ssoUser, err := service.GetUserForSsoAuth(ctx, sso.OidcSsoClaims{
			Sub:   strings.TrimPrefix(*user.ExternalID, provider+":"),
			Email: "some@mail.test",
			Name:  "some name",
		}, "provider", "", "")
		require.NoError(t, err)
		require.Equal(t, user.ID, ssoUser.ID)
		require.Equal(t, user.ExternalID, ssoUser.ExternalID)
		require.Equal(t, user.Email, ssoUser.Email)

		user = createUserFn("test4@mail.test")
		require.NoError(t, err)
		require.Equal(t, console.Inactive, user.Status)

		// GetUserForSsoAuth should return the user if the email matches unverified user
		// activate it and associate the external ID with the user.
		ssoUser, err = service.GetUserForSsoAuth(ctx, sso.OidcSsoClaims{
			Sub:   "anotherID",
			Email: user.Email,
			Name:  "some name",
		}, provider, "", "")
		require.NoError(t, err)
		require.Equal(t, user.ID, ssoUser.ID)
		require.Equal(t, getExternalID("anotherID"), *ssoUser.ExternalID)
		require.Equal(t, user.Email, ssoUser.Email)
		require.Equal(t, console.Active, ssoUser.Status)

		// GetUserForSsoAuth should return the user if the email matches an existing user
		// with a different external ID and associate the new external ID with the user.
		ssoUser, err = service.GetUserForSsoAuth(ctx, sso.OidcSsoClaims{
			Sub:   "newID",
			Email: user.Email,
			Name:  "some name",
		}, provider, "", "")
		require.NoError(t, err)
		require.Equal(t, user.ID, ssoUser.ID)
		require.Equal(t, getExternalID("newID"), *ssoUser.ExternalID)

		user = createUserFn("test5@mail.test")
		require.NoError(t, err)

		err = service.SetAccountActive(ctx, user)
		require.NoError(t, err)

		// GetUserForSsoAuth should return the user if the email matches verified user
		// and associate the external ID with the user.
		ssoUser, err = service.GetUserForSsoAuth(ctx, sso.OidcSsoClaims{
			Sub:   "ID",
			Email: user.Email,
			Name:  "some name",
		}, provider, "", "")
		require.NoError(t, err)
		require.Equal(t, user.ID, ssoUser.ID)
		require.Equal(t, getExternalID("ID"), *ssoUser.ExternalID)
		require.Equal(t, user.Email, ssoUser.Email)

		// GetUserForSsoAuth should create a new user.
		ssoUser, err = service.GetUserForSsoAuth(ctx, sso.OidcSsoClaims{
			Sub:   "externalID",
			Email: "external@mail.test",
			Name:  "some name",
		}, provider, "", "")
		require.NoError(t, err)
		require.Equal(t, getExternalID("externalID"), *ssoUser.ExternalID)
		require.Equal(t, "external@mail.test", ssoUser.Email)
		require.Equal(t, console.Active, ssoUser.Status)
		require.Empty(t, ssoUser.PasswordHash)
	})
}

func TestSsoFlow(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.SSO.Enabled = true
				config.SSO.MockSso = true
				config.SSO.MockEmail = "some@fake.test"
				config.Console.RateLimit.Burst = 50
				config.SSO.OidcProviderInfos = sso.OidcProviderInfos{
					Values: map[string]sso.OidcProviderInfo{
						"fakeProvider": {},
					},
				}
				reg := regexp.MustCompile(`@fake.test`)
				require.NotNil(t, reg)
				config.SSO.EmailProviderMappings = sso.EmailProviderMappings{
					Values: map[string]regexp.Regexp{
						"fakeProvider": *reg,
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
		require.NoError(t, err)

		client := &http.Client{Jar: jar}

		getSsoURL := func(email string, expectedCode int) string {
			ssoRootURL, err := url.JoinPath(sat.ConsoleURL(), "/sso")
			require.NoError(t, err)

			providerUrl, err := url.Parse(ssoRootURL)
			require.NoError(t, err)

			providerUrl = providerUrl.JoinPath("url")
			q := providerUrl.Query()
			q.Add("email", email)
			providerUrl.RawQuery = q.Encode()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, providerUrl.String(), nil)
			require.NoError(t, err)

			result, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, expectedCode, result.StatusCode)

			if expectedCode != http.StatusOK {
				return ""
			}

			body, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			require.NoError(t, result.Body.Close())

			providerUrl, err = url.Parse(string(body))
			require.NoError(t, err)

			q = providerUrl.Query()
			q.Add("email", email)
			providerUrl.RawQuery = q.Encode()

			return providerUrl.String()
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, getSsoURL("some@fake.test", http.StatusOK), nil)
		require.NoError(t, err)

		result, err := client.Do(req)
		require.NoError(t, err)
		// success should redirect to the satellite UI
		require.Equal(t, sat.ConsoleURL()+"/", result.Request.URL.String())
		require.NoError(t, result.Body.Close())

		// user should be created
		user, err := sat.API.DB.Console().Users().GetByEmailAndTenant(ctx, "some@fake.test", nil)
		require.NoError(t, err)
		// session should be created
		sessions, err := sat.API.DB.Console().WebappSessions().GetAllByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, sessions, 1)

		// try sso with an unknown email. this should fail
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, getSsoURL("another@fake.test", http.StatusOK), nil)
		require.NoError(t, err)

		result, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		// failure should redirect to the login page with an error
		require.Contains(t, result.Request.URL.String(), "login?sso_failed=true")
		require.NoError(t, result.Body.Close())

		// getting the sso url for an email that doesn't match the provider should fail
		getSsoURL("another@who.test", http.StatusNotFound)
	})
}

func TestRegister_WithMemberInvitation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.MemberAccountsEnabled = true
				config.Console.RateLimit.Burst = 10
				config.Mail.AuthType = "nomail"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		invitationsDB := sat.DB.Console().ProjectInvitations()
		membersDB := sat.DB.Console().ProjectMembers()
		projectsDB := sat.DB.Console().Projects()

		type registerData struct {
			FullName     string `json:"fullName"`
			ShortName    string `json:"shortName"`
			Email        string `json:"email"`
			Password     string `json:"password"`
			InviterEmail string `json:"inviterEmail"`
		}

		makeRequest := func(data registerData) (int, string) {
			jsonBody, err := json.Marshal(data)
			require.NoError(t, err)

			endpoint := sat.ConsoleURL() + "/api/v0/auth/register"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, result.Body.Close())
			}()

			body, err := io.ReadAll(result.Body)
			require.NoError(t, err)

			return result.StatusCode, string(body)
		}

		setupInviterAndProject := func(t *testing.T, emailSuffix string) (*console.User, *console.Project) {
			inviter, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Inviter User",
				Email:    "inviter-" + emailSuffix + "@example.com",
			}, 1)
			require.NoError(t, err)

			project, err := projectsDB.Insert(ctx, &console.Project{
				ID:       testrand.UUID(),
				PublicID: testrand.UUID(),
				OwnerID:  inviter.ID,
			})
			require.NoError(t, err)

			return inviter, project
		}

		t.Run("Valid invitation with inviter email", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "valid")
			inviteeEmail := "invitee-valid@example.com"

			_, err := invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     inviteeEmail,
				InviterID: &inviter.ID,
			})
			require.NoError(t, err)

			status, _ := makeRequest(registerData{
				FullName:     "Invitee User",
				ShortName:    "Invitee",
				Email:        inviteeEmail,
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusOK, status)

			// Verify user was created as MemberUser.
			_, users, err := service.GetUserByEmailWithUnverified(ctx, inviteeEmail)
			require.NoError(t, err)
			require.Len(t, users, 1)
			user := &users[0]
			require.Equal(t, console.MemberUser, user.Kind)
			require.True(t, user.TrialExpiration == nil || user.TrialExpiration.IsZero(), "Trial expiration should not be set")

			// Verify user was added to project.
			member, err := membersDB.GetByMemberIDAndProjectID(ctx, user.ID, project.ID)
			require.NoError(t, err)
			require.NotNil(t, member)
			require.Equal(t, console.RoleMember, member.Role)

			// Verify invitation was deleted.
			invites, err := invitationsDB.GetByProjectID(ctx, project.ID)
			require.NoError(t, err)
			for _, inv := range invites {
				require.NotEqual(t, inviteeEmail, inv.Email, "Invitation should be deleted after successful registration")
			}
		})

		t.Run("Invalid inviter email format", func(t *testing.T) {
			status, body := makeRequest(registerData{
				FullName:     "Test User",
				Email:        "test-invalid-format@example.com",
				Password:     "password123",
				InviterEmail: "invalid-email",
			})
			require.Equal(t, http.StatusBadRequest, status)
			require.Contains(t, body, "Invalid inviter email")
		})

		t.Run("No invitation found", func(t *testing.T) {
			inviter, _ := setupInviterAndProject(t, "noinvite")
			status, body := makeRequest(registerData{
				FullName:     "No Invite User",
				Email:        "noinvite@example.com",
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusForbidden, status)
			require.Contains(t, body, "no valid invitation found")
		})

		t.Run("Inviter does not exist", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "noninviter")
			inviteeEmail := "invitee-noninviter@example.com"

			_, err := invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     inviteeEmail,
				InviterID: &inviter.ID,
			})
			require.NoError(t, err)

			status, body := makeRequest(registerData{
				FullName:     "Test User",
				Email:        inviteeEmail,
				Password:     "password123",
				InviterEmail: "nonexistent@example.com",
			})
			require.Equal(t, http.StatusForbidden, status)
			require.Contains(t, body, "error getting inviter info")
		})

		t.Run("Invitation from different inviter", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "different")
			otherInviter, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Other Inviter",
				Email:    "otherinviter-different@example.com",
			}, 1)
			require.NoError(t, err)

			inviteeEmail := "invitee-different@example.com"

			_, err = invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     inviteeEmail,
				InviterID: &otherInviter.ID,
			})
			require.NoError(t, err)

			status, body := makeRequest(registerData{
				FullName:     "Test User",
				Email:        inviteeEmail,
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusForbidden, status)
			require.Contains(t, body, "no valid invitation found")
		})

		t.Run("Expired invitation", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "expired")
			inviteeEmail := "invitee-expired@example.com"

			invite, err := invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     inviteeEmail,
				InviterID: &inviter.ID,
			})
			require.NoError(t, err)

			// Manually update the CreatedAt to make it expired.
			db := sat.DB.Testing()
			expiredDate := time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration - time.Hour)
			result, err := db.RawDB().ExecContext(ctx,
				db.Rebind("UPDATE project_invitations SET created_at = ? WHERE project_id = ? AND email = ?"),
				expiredDate, invite.ProjectID, strings.ToUpper(invite.Email),
			)
			require.NoError(t, err)

			count, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, count)

			status, body := makeRequest(registerData{
				FullName:     "Test User",
				Email:        inviteeEmail,
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusForbidden, status)
			require.Contains(t, body, "invitation has expired")
		})

		t.Run("Disabled project", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "disabled")
			inviteeEmail := "invitee-disabled@example.com"

			_, err := invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     inviteeEmail,
				InviterID: &inviter.ID,
			})
			require.NoError(t, err)

			err = projectsDB.UpdateStatus(ctx, project.ID, console.ProjectDisabled)
			require.NoError(t, err)

			status, body := makeRequest(registerData{
				FullName:     "Test User",
				Email:        inviteeEmail,
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusForbidden, status)
			require.Contains(t, body, "project you were invited to no longer exists")
		})

		t.Run("Normal registration without inviter email", func(t *testing.T) {
			normalUserEmail := "normaluser@example.com"

			status, _ := makeRequest(registerData{
				FullName:  "Normal User",
				ShortName: "Normal",
				Email:     normalUserEmail,
				Password:  "password123",
			})
			require.Equal(t, http.StatusOK, status)

			_, users, err := service.GetUserByEmailWithUnverified(ctx, normalUserEmail)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.NotEqual(t, console.MemberUser, users[0].Kind)
		})

		t.Run("Existing unverified user with invitation", func(t *testing.T) {
			inviter, project := setupInviterAndProject(t, "existing")
			existingEmail := "existing@example.com"

			status, _ := makeRequest(registerData{
				FullName: "Existing User",
				Email:    existingEmail,
				Password: "password123",
			})
			require.Equal(t, http.StatusOK, status)

			_, err := invitationsDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     existingEmail,
				InviterID: &inviter.ID,
			})
			require.NoError(t, err)

			_, usersBefore, err := service.GetUserByEmailWithUnverified(ctx, existingEmail)
			require.NoError(t, err)
			require.Len(t, usersBefore, 1)
			userBefore := &usersBefore[0]

			// Re-register with inviter email.
			status, _ = makeRequest(registerData{
				FullName:     "Existing User Updated",
				Email:        existingEmail,
				Password:     "password123",
				InviterEmail: inviter.Email,
			})
			require.Equal(t, http.StatusOK, status)

			// Verify user was updated to MemberUser.
			_, usersAfter, err := service.GetUserByEmailWithUnverified(ctx, existingEmail)
			require.NoError(t, err)
			require.Len(t, usersAfter, 1)
			userAfter := &usersAfter[0]
			require.Equal(t, userBefore.ID, userAfter.ID)
			require.Equal(t, console.MemberUser, userAfter.Kind)

			// Verify user was added to project.
			member, err := membersDB.GetByMemberIDAndProjectID(ctx, userAfter.ID, project.ID)
			require.NoError(t, err)
			require.NotNil(t, member)
		})
	})
}

func TestAuth_DeleteAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SelfServeAccountDeleteEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()

		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

		sat.API.Console.Service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})

		proUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "test user",
			Email:    "testpro@mail.test",
		}, 1)
		require.NoError(t, err)

		proUser.Kind = console.PaidUser
		proUser.MFAEnabled = true
		mfaSecret, err := console.NewMFASecretKey()
		require.NoError(t, err)
		proUser.MFASecretKey = mfaSecret
		mfaSecretKeyPtr := &proUser.MFASecretKey

		goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		require.NoError(t, sat.DB.Console().Users().Update(ctx, proUser.ID, console.UpdateUserRequest{
			Kind:         &proUser.Kind,
			MFAEnabled:   &proUser.MFAEnabled,
			MFASecretKey: &mfaSecretKeyPtr,
		}))

		proUserProject, err := sat.DB.Console().Projects().Insert(ctx, &console.Project{
			ID:       testrand.UUID(),
			PublicID: testrand.UUID(),
			OwnerID:  proUser.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, proUserProject)

		freeUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "test user",
			Email:    "testfree@mail.test",
		}, 1)
		require.NoError(t, err)

		freeUser.MFAEnabled = true
		require.NoError(t, sat.DB.Console().Users().Update(ctx, freeUser.ID, console.UpdateUserRequest{
			MFAEnabled:   &freeUser.MFAEnabled,
			MFASecretKey: &mfaSecretKeyPtr,
		}))

		freeUserProject, err := sat.DB.Console().Projects().Insert(ctx, &console.Project{
			ID:       testrand.UUID(),
			PublicID: testrand.UUID(),
			OwnerID:  freeUser.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, freeUserProject)

		endpoint := "auth/account"

		tests := []struct {
			name               string
			prepare            func(user, project uuid.UUID) error
			cleanup            func(user, project uuid.UUID) error
			req                consoleapi.AccountActionData
			expectedResp       *console.DeleteAccountResponse
			proUserHttpStatus  int
			freeUserHttpStatus int
		}{
			{
				name:               "account delete step out of range: lesser",
				req:                consoleapi.AccountActionData{Step: -1, Data: ""},
				proUserHttpStatus:  http.StatusBadRequest,
				freeUserHttpStatus: http.StatusBadRequest,
			},
			{
				name:               "account delete step out of range: greater",
				req:                consoleapi.AccountActionData{Step: 100, Data: ""},
				proUserHttpStatus:  http.StatusBadRequest,
				freeUserHttpStatus: http.StatusBadRequest,
			},
			{
				name:               "data can't be empty if step is verifying input",
				req:                consoleapi.AccountActionData{Step: console.VerifyAccountPasswordStep, Data: ""},
				proUserHttpStatus:  http.StatusBadRequest,
				freeUserHttpStatus: http.StatusBadRequest,
			},
			{ // N.B. the DeleteAccount handler returns 403 if user is legal hold, but actually it is wrapped in the withAuth handler which returns 401.
				name: "legal hold can't be deleted",
				prepare: func(user, project uuid.UUID) error {
					status := console.LegalHold
					return sat.DB.Console().Users().Update(ctx, user, console.UpdateUserRequest{Status: &status})
				},
				cleanup: func(user, project uuid.UUID) error {
					status := console.Active
					return sat.DB.Console().Users().Update(ctx, user, console.UpdateUserRequest{Status: &status})
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				proUserHttpStatus:  http.StatusUnauthorized,
				freeUserHttpStatus: http.StatusUnauthorized,
			},
			{
				name: "locked out user can't be deleted",
				req:  consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				prepare: func(user, project uuid.UUID) error {
					expires := timestamp.Add(24 * time.Hour)
					ptr := &expires
					return sat.DB.Console().Users().Update(ctx, user, console.UpdateUserRequest{LoginLockoutExpiration: &ptr})
				},
				cleanup: func(user, project uuid.UUID) error {
					var ptr *time.Time
					return sat.DB.Console().Users().Update(ctx, user, console.UpdateUserRequest{LoginLockoutExpiration: &ptr})
				},
				proUserHttpStatus:  http.StatusUnauthorized,
				freeUserHttpStatus: http.StatusUnauthorized,
			},
			{
				name: "has buckets",
				prepare: func(user, project uuid.UUID) error {
					_, err := sat.API.Buckets.Service.CreateBucket(ctx, buckets.Bucket{
						ID:        testrand.UUID(),
						Name:      "testbucket",
						ProjectID: project,
					})
					return err
				},
				cleanup: func(user, project uuid.UUID) error {
					return sat.API.Buckets.Service.DeleteBucket(ctx, []byte("testbucket"), project)
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, Buckets: 1},
				proUserHttpStatus:  http.StatusConflict,
				freeUserHttpStatus: http.StatusConflict,
			},
			{
				name: "has api keys",
				prepare: func(user, project uuid.UUID) error {
					_, err := sat.API.DB.Console().APIKeys().Create(ctx, []byte("testapikey"), console.APIKeyInfo{
						ID:              testrand.UUID(),
						ProjectID:       project,
						ProjectPublicID: project,
						Secret:          []byte("super-secret-secret"),
					})
					return err
				},
				cleanup: func(user, project uuid.UUID) error {
					return sat.API.DB.Console().APIKeys().DeleteAllByProjectID(ctx, project)
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, ApiKeys: 1},
				proUserHttpStatus:  http.StatusConflict,
				freeUserHttpStatus: http.StatusConflict,
			},
			{
				name: "has unpaid invoices",
				prepare: func(user, project uuid.UUID) error {
					invoice1, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user, 1000, "open invoice")
					if err != nil {
						return err
					}
					_, err = sat.API.Payments.Accounts.Invoices().Create(ctx, user, 1000, "draft invoice")
					if err != nil {
						return err
					}
					_, err = sat.API.Payments.StripeClient.Invoices().FinalizeInvoice(invoice1.ID, nil)
					return err
				},
				cleanup: func(user, project uuid.UUID) error {
					invoices, err := sat.API.Payments.Accounts.Invoices().List(ctx, user)
					if err != nil {
						return err
					}
					for _, invoice := range invoices {
						_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoice.ID)
						if err != nil {
							return err
						}
					}
					return nil
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, UnpaidInvoices: 2, AmountOwed: 2 * int64(1000)},
				proUserHttpStatus:  http.StatusConflict,
				freeUserHttpStatus: http.StatusConflict,
			},
			{
				name: "has current usage",
				prepare: func(user, project uuid.UUID) error {
					return sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, project, []byte("testbucket"), pb.PieceAction_GET, 1000000, 0, timestamp.Add(-time.Minute))
				},
				cleanup: func(user, project uuid.UUID) error {
					_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 100)
					return err
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, CurrentUsage: true},
				proUserHttpStatus:  http.StatusConflict,
				freeUserHttpStatus: http.StatusOK,
			},
			{
				name: "last month's usage not invoiced yet",
				prepare: func(user, project uuid.UUID) error {
					return sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, project, []byte("testbucket"), pb.PieceAction_GET, 1000000, 0, lastMonth)
				},
				cleanup: func(user, project uuid.UUID) error {
					return sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
						ProjectID: project,
						Egress:    1000000,
					}}, lastMonth, thisMonth)
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, InvoicingIncomplete: true},
				proUserHttpStatus:  http.StatusConflict,
				freeUserHttpStatus: http.StatusOK,
			},
			{ // N.B. the testplanet.Satellite.AddUser method sets password to the user's full name. At the beginning of this test we set the free user's name to the same as pro user.
				name:               "verify password",
				req:                consoleapi.AccountActionData{Step: console.VerifyAccountPasswordStep, Data: proUser.FullName},
				proUserHttpStatus:  http.StatusOK,
				freeUserHttpStatus: http.StatusOK,
			},
			{
				name:               "verify mfa",
				req:                consoleapi.AccountActionData{Step: console.VerifyAccountMfaStep, Data: goodCode},
				proUserHttpStatus:  http.StatusOK,
				freeUserHttpStatus: http.StatusOK,
			},
			{
				name: "verify email",
				prepare: func(user, project uuid.UUID) error {
					code := "123456"
					return sat.API.DB.Console().Users().Update(ctx, user, console.UpdateUserRequest{
						ActivationCode: &code,
					})
				},
				req:                consoleapi.AccountActionData{Step: console.VerifyAccountEmailStep, Data: "123456"},
				proUserHttpStatus:  http.StatusOK,
				freeUserHttpStatus: http.StatusOK,
			},
			{
				name:               "successfully deleted",
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountStep, Data: ""},
				proUserHttpStatus:  http.StatusOK,
				freeUserHttpStatus: http.StatusOK,
			},
		}

		for _, u := range []*console.User{proUser, freeUser} {
			projects, err := sat.API.DB.Console().Projects().GetOwn(ctx, u.ID)
			require.NoError(t, err)

			userType := "pro_user_"
			if u.IsFree() {
				userType = "free_user_"
			}

			for _, tt := range tests {
				t.Run(userType+tt.name, func(t *testing.T) {
					if tt.prepare != nil {
						require.NoError(t, tt.prepare(u.ID, projects[0].ID))
					}
					payload, err := json.Marshal(tt.req)
					require.NoError(t, err)

					resp, status, err := doRequestWithAuth(ctx, t, sat, u, http.MethodDelete, endpoint, bytes.NewBuffer(payload))
					require.NoError(t, err)

					switch u.ID {
					case freeUser.ID:
						require.Equal(t, tt.freeUserHttpStatus, status)
					case proUser.ID:
						require.Equal(t, tt.proUserHttpStatus, status)
					default:
						t.FailNow()
					}

					if u.ID == freeUser.ID {
						require.Equal(t, tt.freeUserHttpStatus, status)
					} else {
						require.Equal(t, tt.proUserHttpStatus, status)
					}

					if status != http.StatusOK {
						require.NotNil(t, resp)

						if status == http.StatusConflict {
							var data console.DeleteAccountResponse
							require.NoError(t, json.Unmarshal(resp, &data))

							require.Equal(t, *tt.expectedResp, data)
						} else {
							var data struct {
								Error string `json:"error"`
							}
							require.NoError(t, json.Unmarshal(resp, &data))
						}
					}

					if tt.cleanup != nil {
						require.NoError(t, tt.cleanup(u.ID, projects[0].ID))
					}
				})
			}

			_, err = sat.API.DB.Console().Users().GetByEmailAndTenant(ctx, u.Email, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, sql.ErrNoRows)
		}
	})
}
