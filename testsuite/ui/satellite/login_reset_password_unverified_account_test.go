// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
	"testing"
)

func TestForgotPasswordOnLoginPageUsingUnverifiedAccount(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		emailAddress := "unverified@andnonexistent.test"
		page := browser.MustPage(loginPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// Reset password link is clicked on login page
		page.MustElement(".login-area__content-area__reset-msg__link").MustClick()

		// Forgot password elements are checked to verify the page
		forgotPasswordHeader := page.MustElement(".forgot-area__content-area__container__title-area").MustText()
		require.Contains(t, forgotPasswordHeader, "Reset Password")
		emailAddressInput := page.MustElement(".headerless-input")
		require.Condition(t, emailAddressInput.MustVisible)

		// Tries resetting password for an account that does not exist or is not activated
		page.MustElement(".headerless-input").MustClick().MustInput(emailAddress)
		page.MustElement(".forgot-area__content-area__container__button").MustClick()
		passwordResetMessage := page.MustElement(".notification-wrap__text-area__message").MustText()
		require.Contains(t, passwordResetMessage, "There is no such email")

	})
}
