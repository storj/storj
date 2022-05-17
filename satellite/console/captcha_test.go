// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

const validResponseToken = "myResponseToken"

type mockRecaptcha struct{}

func (r mockRecaptcha) Verify(ctx context.Context, responseToken string, userIP string) (bool, error) {
	return responseToken == validResponseToken, nil
}

// TestRecaptcha ensures the reCAPTCHA service is working properly.
func TestRecaptcha(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Recaptcha.Enabled = true
				config.Console.Recaptcha.SecretKey = "mySecretKey"
				config.Console.Recaptcha.SiteKey = "mySiteKey"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].API.Console.Service
		require.NotNil(t, service)
		service.TestSwapCaptchaHandler(mockRecaptcha{})

		regToken1, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user, err := service.CreateUser(ctx, console.CreateUser{
			FullName:        "User",
			Email:           "u@mail.test",
			Password:        "password",
			CaptchaResponse: validResponseToken,
		}, regToken1.Secret)

		require.NotNil(t, user)
		require.NoError(t, err)

		regToken2, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user, err = service.CreateUser(ctx, console.CreateUser{
			FullName:        "User2",
			Email:           "u2@mail.test",
			Password:        "password",
			CaptchaResponse: "wrong",
		}, regToken2.Secret)

		require.Nil(t, user)
		require.True(t, console.ErrCaptcha.Has(err))
	})
}
