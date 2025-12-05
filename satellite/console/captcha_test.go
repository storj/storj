// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

const validResponseToken = "myResponseToken"

type mockRecaptcha struct{}

func (r mockRecaptcha) Verify(ctx context.Context, responseToken string, userIP string) (bool, *float64, error) {
	score := 1.0
	return responseToken == validResponseToken, &score, nil
}

// TestRegistrationRecaptcha ensures that registration reCAPTCHA service is working properly.
func TestRegistrationRecaptcha(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Captcha.Registration.Recaptcha.Enabled = true
				config.Console.Captcha.Registration.Recaptcha.SecretKey = "mySecretKey"
				config.Console.Captcha.Registration.Recaptcha.SiteKey = "mySiteKey"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].API.Console.Service
		require.NotNil(t, service)
		service.TestSwapCaptchaHandler(mockRecaptcha{})

		valid, score, err := service.VerifyRegistrationCaptcha(ctx, validResponseToken, "127.0.0.1")
		require.NoError(t, err)
		require.True(t, valid)
		require.Equal(t, 1.0, *score)

		valid, _, _ = service.VerifyRegistrationCaptcha(ctx, "wrong", "127.0.0.1")
		require.False(t, valid)
	})
}

// TestLoginRecaptcha ensures that login reCAPTCHA service is working properly.
func TestLoginRecaptcha(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Captcha.Login.Recaptcha.Enabled = true
				config.Console.Captcha.Login.Recaptcha.SecretKey = "mySecretKey"
				config.Console.Captcha.Login.Recaptcha.SiteKey = "mySiteKey"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		require.NotNil(t, service)
		service.TestSwapCaptchaHandler(mockRecaptcha{})

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		email := "user@mail.test"
		password := "password"

		user, err := service.CreateUser(ctx, console.CreateUser{
			FullName:        "User",
			Email:           email,
			Password:        password,
			CaptchaResponse: validResponseToken,
		}, regToken.Secret)

		require.NotNil(t, user)
		require.NoError(t, err)

		activationToken, err := service.GenerateActivationToken(ctx, user.ID, user.Email)
		require.NoError(t, err)

		user, err = service.ActivateAccount(ctx, activationToken)
		require.NotNil(t, user)
		require.NoError(t, err)

		token, err := service.Token(ctx, console.AuthUser{
			Email:           email,
			Password:        password,
			CaptchaResponse: validResponseToken,
		})

		require.NotEmpty(t, token)
		require.NoError(t, err)

		token, err = service.Token(ctx, console.AuthUser{
			Email:           email,
			Password:        password,
			CaptchaResponse: "wrong",
		})

		require.Empty(t, token)
		require.True(t, console.ErrCaptcha.Has(err))

		// testing that captcha should not be checked when verifying MFA
		// enable MFA
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		key, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		now := time.Now()
		passcode, err := console.NewMFAPasscode(key, now)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, passcode, now)
		require.NoError(t, err)

		// bypassing captcha check should work for accounts with MFA.
		_, err = service.Token(ctx, console.AuthUser{
			Email:           email,
			Password:        password,
			CaptchaResponse: "wrong-should-be-ignored",
			MFAPasscode:     passcode,
		})
		require.NoError(t, err)

		// disable MFA
		passcode, err = console.NewMFAPasscode(key, now)
		require.NoError(t, err)

		err = service.DisableUserMFA(userCtx, passcode, now, "")
		require.NoError(t, err)

		// bypassing captcha check should not work for
		// accounts without MFA.
		_, err = service.Token(ctx, console.AuthUser{
			Email:           email,
			Password:        password,
			CaptchaResponse: "wrong-should-not-be-ignored",
			MFAPasscode:     passcode,
		})
		require.True(t, console.ErrCaptcha.Has(err))
	})
}

// TestForgotPasswordRecaptcha ensures that the forgot password reCAPTCHA service is working properly.
func TestForgotPasswordRecaptcha(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Captcha.Login.Recaptcha.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		service.TestSwapCaptchaHandler(mockRecaptcha{})

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName:        "Test User",
			Email:           "user@mail.test",
			CaptchaResponse: validResponseToken,
		}, 1)
		require.NoError(t, err)

		sendEmail := func(captchaResponse string) int {
			url := sat.ConsoleURL() + "/api/v0/auth/forgot-password"
			jsonBody := []byte(fmt.Sprintf(`{"email":"%s","captchaResponse":"%s"}`, user.Email, captchaResponse))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
			require.NoError(t, err)

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			t.Log(string(bodyBytes))

			require.NoError(t, result.Body.Close())

			return result.StatusCode
		}

		require.Equal(t, http.StatusBadRequest, sendEmail("wrong"))
		require.Equal(t, http.StatusOK, sendEmail(validResponseToken))
		service.TestSwapCaptchaHandler(nil)
		require.Equal(t, http.StatusOK, sendEmail("wrong"))
	})
}

func TestResendEmailRecaptcha(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Captcha.Registration.Recaptcha.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		service.TestSwapCaptchaHandler(mockRecaptcha{})

		user, err := sat.DB.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test user",
			Email:        "user@mail.test",
			PasswordHash: []byte("passwordhash"),
			Status:       console.Inactive,
		})
		require.NoError(t, err)

		sendEmail := func(captchaResponse string) int {
			url := sat.ConsoleURL() + "/api/v0/auth/resend-email"
			jsonBody := []byte(fmt.Sprintf(`{"email":"%s","captchaResponse":"%s"}`, user.Email, captchaResponse))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
			require.NoError(t, err)

			result, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			t.Log(string(bodyBytes))

			require.NoError(t, result.Body.Close())

			return result.StatusCode
		}

		require.Equal(t, http.StatusBadRequest, sendEmail("wrong"))
		require.Equal(t, http.StatusOK, sendEmail(validResponseToken))
		service.TestSwapCaptchaHandler(nil)
		require.Equal(t, http.StatusOK, sendEmail("wrong"))
	})
}
