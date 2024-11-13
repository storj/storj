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

	tokenInfo, err := sat.API.Console.Service.GenerateSessionToken(ctx, user.ID, user.Email, "", "", nil)
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
		restKeys := satellite.API.REST.Keys

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		expires := 5 * time.Hour
		apiKey, _, err := restKeys.Create(ctx, user.ID, expires)
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

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)
		buf := bytes.NewBuffer(bodyBytes)

		url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/token"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
		require.NoError(t, err)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, response)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Contains(t, string(responseBody), "token")
		require.NoError(t, response.Body.Close())

		// enable SSO
		satellite.API.Console.Service.TestToggleSsoEnabled(true)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NotEmpty(t, response)
		require.Equal(t, http.StatusForbidden, response.StatusCode)

		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NotContains(t, string(responseBody), "token")
		require.NoError(t, response.Body.Close())
	})
}

func TestSsoUserForgotPassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
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

		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)
		buf := bytes.NewBuffer(bodyBytes)

		url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/forgot-password"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
		require.NoError(t, err)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())

		token, err := satellite.DB.Console().ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, token)

		err = satellite.DB.Console().ResetPasswordTokens().Delete(ctx, token.Secret)
		require.NoError(t, err)

		// enable SSO
		satellite.API.Console.Service.TestToggleSsoEnabled(true)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, response.StatusCode)
		require.NoError(t, response.Body.Close())

		token, err = satellite.DB.Console().ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
		require.Equal(t, sql.ErrNoRows, err)
		require.Nil(t, token)
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
			_, users, err := sat.DB.Console().Users().GetByEmailWithUnverified(ctx, email)
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
			_, users, err := sat.DB.Console().Users().GetByEmailWithUnverified(ctx, email)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Contains(t, body, users[0].ActivationCode)
		} else {
			require.Contains(t, body, "/activation")
		}

		// Registration attempts using existing and verified e-mail address should send account already exists e-mail.
		_, users, err := sat.DB.Console().Users().GetByEmailWithUnverified(ctx, email)
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
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		proUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test Pro User",
			Email:    "testpro@mail.test",
		}, 1)
		require.NoError(t, err)

		proUser.PaidTier = true
		require.NoError(t, sat.DB.Console().Users().Update(ctx, proUser.ID, console.UpdateUserRequest{PaidTier: &proUser.PaidTier}))

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
			_, status, err := doRequestWithAuth(ctx, t, sat, tt.user, http.MethodPatch, endpoint, bytes.NewBuffer([]byte(tt.input)))
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
			_, users, err := sat.DB.Console().Users().GetByEmailWithUnverified(ctx, user.Email)
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
		user, err := sat.DB.Console().Users().GetByEmail(ctx, email)
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
			Email:      "test@mail.com",
			FullName:   "Test User",
		}
		user, err := service.CreateSsoUser(ctx, createUser1)
		require.NoError(t, err)
		require.Equal(t, &createUser1.ExternalId, user.ExternalID)
		require.Equal(t, createUser1.Email, user.Email)
		require.Equal(t, createUser1.FullName, user.FullName)
		require.Equal(t, console.Active, user.Status)
		require.Empty(t, user.PasswordHash)

		user = createUserFn("test2@mail.com")
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

		user = createUserFn("test3@mail.com")
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
			Email: "some@mail.com",
			Name:  "some name",
		}, "provider", "", "")
		require.NoError(t, err)
		require.Equal(t, user.ID, ssoUser.ID)
		require.Equal(t, user.ExternalID, ssoUser.ExternalID)
		require.Equal(t, user.Email, ssoUser.Email)

		user = createUserFn("test4@mail.com")
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

		user = createUserFn("test5@mail.com")
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
			Email: "external@mail.com",
			Name:  "some name",
		}, provider, "", "")
		require.NoError(t, err)
		require.Equal(t, getExternalID("externalID"), *ssoUser.ExternalID)
		require.Equal(t, "external@mail.com", ssoUser.Email)
		require.Equal(t, console.Active, ssoUser.Status)
		require.Empty(t, ssoUser.PasswordHash)
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

		proUser.PaidTier = true
		proUser.MFAEnabled = true
		mfaSecret, err := console.NewMFASecretKey()
		require.NoError(t, err)
		proUser.MFASecretKey = mfaSecret
		mfaSecretKeyPtr := &proUser.MFASecretKey

		goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		require.NoError(t, sat.DB.Console().Users().Update(ctx, proUser.ID, console.UpdateUserRequest{
			PaidTier:     &proUser.PaidTier,
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
					invoice, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user, 1000, "test description")
					if err != nil {
						return err
					}
					_, err = sat.API.Payments.StripeClient.Invoices().FinalizeInvoice(invoice.ID, nil)
					return err
				},
				cleanup: func(user, project uuid.UUID) error {
					invoices, err := sat.API.Payments.Accounts.Invoices().List(ctx, user)
					if err != nil {
						return err
					}
					_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoices[0].ID)
					return err
				},
				req:                consoleapi.AccountActionData{Step: console.DeleteAccountInit, Data: ""},
				expectedResp:       &console.DeleteAccountResponse{OwnedProjects: 1, UnpaidInvoices: 1, AmountOwed: int64(1000)},
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
			if !u.PaidTier {
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

			_, err = sat.API.DB.Console().Users().GetByEmail(ctx, u.Email)
			require.Error(t, err)
			require.ErrorIs(t, err, sql.ErrNoRows)
		}
	})
}
