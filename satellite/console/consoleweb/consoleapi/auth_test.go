// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
)

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
					Password:        "abc123",
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

func TestDeleteAccount(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	// We do a black box testing because currently we don't allow to delete
	// accounts through the API hence we must always return an error response.

	config := &quick.Config{
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			// TODO: use or implement a better and thorough HTTP Request random generator

			var method string
			switch rnd.Intn(9) {
			case 0:
				method = http.MethodGet
			case 1:
				method = http.MethodHead
			case 2:
				method = http.MethodPost
			case 3:
				method = http.MethodPut
			case 4:
				method = http.MethodPatch
			case 5:
				method = http.MethodDelete
			case 6:
				method = http.MethodConnect
			case 7:
				method = http.MethodOptions
			case 8:
				method = http.MethodTrace
			default:
				t.Fatal("unexpected random value for HTTP method selection")
			}

			var path string
			{

				val, ok := quick.Value(reflect.TypeOf(""), rnd)
				require.True(t, ok, "quick.Values generator function couldn't generate a string")
				path = url.PathEscape(val.String())
			}

			var query string
			{
				nparams := rnd.Intn(27)
				params := make([]string, nparams)

				for i := 0; i < nparams; i++ {
					val, ok := quick.Value(reflect.TypeOf(""), rnd)
					require.True(t, ok, "quick.Values generator function couldn't generate a string")
					param := val.String()

					val, ok = quick.Value(reflect.TypeOf(""), rnd)
					require.True(t, ok, "quick.Values generator function couldn't generate a string")
					param += "=" + val.String()

					params[i] = param
				}

				query = url.QueryEscape(strings.Join(params, "&"))
			}

			var body io.Reader
			{
				val, ok := quick.Value(reflect.TypeOf([]byte(nil)), rnd)
				require.True(t, ok, "quick.Values generator function couldn't generate a byte slice")
				body = bytes.NewReader(val.Bytes())
			}

			withQuery := ""
			if len(query) > 0 {
				withQuery = "?"
			}

			reqURL, err := url.Parse("//storj.io/" + path + withQuery + query)
			require.NoError(t, err, "error when generating a random URL")
			req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
			require.NoError(t, err, "error when geneating a random request")
			values[0] = reflect.ValueOf(req)
		},
	}

	expectedHandler := func(_ *http.Request) (status int, body []byte) {
		return http.StatusNotImplemented, []byte("{\"error\":\"The server is incapable of fulfilling the request\"}\n")
	}

	actualHandler := func(r *http.Request) (status int, body []byte) {
		rr := httptest.NewRecorder()
		authController := consoleapi.NewAuth(log, nil, nil, nil, nil, nil, "", "", "", "", "", "")
		authController.DeleteAccount(rr, r)

		//nolint:bodyclose
		result := rr.Result()
		defer func() {
			err := result.Body.Close()
			require.NoError(t, err)
		}()

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)

		return result.StatusCode, body

	}

	err := quick.CheckEqual(expectedHandler, actualHandler, config)
	if err != nil {
		fmt.Printf("%+v\n", err)
		var cerr *quick.CheckEqualError
		require.True(t, errors.As(err, &cerr))

		t.Fatalf(`DeleteAccount handler has returned a different response:
round: %d
input args: %+v
expected response:
	status code: %d
	response body: %s
returned response:
	status code: %d
	response body: %s
`, cerr.Count, cerr.In, cerr.Out1[0], cerr.Out1[1], cerr.Out2[0], cerr.Out2[1])
	}
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

func TestMFAEndpoints(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
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

		doRequest := func(urlSuffix string, passcode string, recoveryCode string) *http.Response {
			urlLink := sat.ConsoleURL() + "/api/v0/auth/mfa" + urlSuffix
			var buf io.Reader

			body := &data{
				Passcode:     passcode,
				RecoveryCode: recoveryCode,
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)
			buf = bytes.NewBuffer(bodyBytes)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlLink, buf)
			require.NoError(t, err)

			req.AddCookie(&http.Cookie{
				Name:    "_tokenKey",
				Path:    "/",
				Value:   tokenInfo.Token.String(),
				Expires: time.Now().AddDate(0, 0, 1),
			})

			req.Header.Set("Content-Type", "application/json")

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			return result
		}

		// Expect failure because MFA is not enabled.
		result := doRequest("/generate-recovery-codes", "", "")
		require.Equal(t, http.StatusUnauthorized, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect failure due to not having generated a secret key.
		result = doRequest("/enable", "123456", "")
		require.Equal(t, http.StatusBadRequest, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect success when generating a secret key.
		result = doRequest("/generate-secret-key", "", "")
		require.Equal(t, http.StatusOK, result.StatusCode)

		var key string
		err = json.NewDecoder(result.Body).Decode(&key)
		require.NoError(t, err)

		require.NoError(t, result.Body.Close())

		// Expect failure due to prodiving empty passcode.
		result = doRequest("/enable", "", "")
		require.Equal(t, http.StatusBadRequest, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect failure due to providing invalid passcode.
		badCode, err := console.NewMFAPasscode(key, time.Now().Add(time.Hour))
		require.NoError(t, err)
		result = doRequest("/enable", badCode, "")
		require.Equal(t, http.StatusBadRequest, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect success when providing valid passcode.
		goodCode, err := console.NewMFAPasscode(key, time.Now())
		require.NoError(t, err)
		result = doRequest("/enable", goodCode, "")
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect 10 recovery codes to be generated.
		result = doRequest("/generate-recovery-codes", "", "")
		require.Equal(t, http.StatusOK, result.StatusCode)

		var codes []string
		err = json.NewDecoder(result.Body).Decode(&codes)
		require.NoError(t, err)
		require.Len(t, codes, console.MFARecoveryCodeCount)
		require.NoError(t, result.Body.Close())

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
		result = doRequest("/disable", "", "")
		require.Equal(t, http.StatusBadRequest, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect failure due to disabling MFA with invalid passcode.
		result = doRequest("/disable", badCode, "")
		require.Equal(t, http.StatusBadRequest, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect failure when disabling due to providing both passcode and recovery code.
		result = doRequest("/generate-recovery-codes", "", "")
		err = json.NewDecoder(result.Body).Decode(&codes)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())

		result = doRequest("/disable", goodCode, codes[0])
		require.Equal(t, http.StatusConflict, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect success when disabling MFA with valid passcode.
		result = doRequest("/disable", goodCode, "")
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())

		// Expect success when disabling MFA with valid recovery code.
		result = doRequest("/generate-secret-key", "", "")
		err = json.NewDecoder(result.Body).Decode(&key)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())

		goodCode, err = console.NewMFAPasscode(key, time.Now())
		require.NoError(t, err)
		result = doRequest("/enable", goodCode, "")
		require.NoError(t, result.Body.Close())

		result = doRequest("/generate-recovery-codes", "", "")
		err = json.NewDecoder(result.Body).Decode(&codes)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())

		result = doRequest("/disable", "", codes[0])
		require.Equal(t, http.StatusOK, result.StatusCode)
		require.NoError(t, result.Body.Close())
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
			"password":  "123a123",
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
		require.Contains(t, body, "/activation")

		// Registration attempts using existing but unverified e-mail address should send activation e-mail.
		register()
		body, err = sender.Data.Get(ctx)
		require.NoError(t, err)
		require.Contains(t, body, "/activation")

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
		require.Contains(t, body, "/signup")
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
			url := planet.Satellites[0].ConsoleURL() + "/api/v0/auth/resend-email/" + user.Email
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(user.Email))
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
		require.Contains(t, body, "/activation")
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
			Password:        "abc123",
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
			Password: "abc123",
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
			Password:        "abc123",
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
			{"Length below minimum must be rejected", 5, false},
			{"Length as minimum must be accepted", 6, true},
			{"Length as maximum must be accepted", 72, true},
			{"Length above maximum must be rejected", 73, false},
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
