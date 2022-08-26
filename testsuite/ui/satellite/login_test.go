// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestLogin(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		user := planet.Uplinks[0].User[planet.Satellites[0].ID()]

		page := openPage(browser, loginPageURL)

		page.MustElement("[aria-roledescription=email] input").MustInput(user.Email)
		page.MustElement("[aria-roledescription=password] input").MustInput(user.Password)
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")
	})
}

func TestLogin_ForgotPassword_UnverifiedAccount(t *testing.T) {
	t.Skip("does not work")
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

func TestLogin_ForgotPassword_VerifiedAccount(t *testing.T) {
	t.Skip("does not work")
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "testacc@mail.test"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// First time User signup
		page.MustElement("[placeholder=\"Enter Full Name\"]").MustInput(fullName)
		page.MustElement("[placeholder=\"user@example.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// Go back to login page using login link
		page.MustElement("a.register-success-area__login-link").MustClick()

		// Reset password link is clicked on login page
		page.MustElement(".login-area__content-area__reset-msg__link").MustClick()

		// Forgot password elements are checked to verify the page
		forgotPasswordHeader := page.MustElement(".forgot-area__content-area__container__title-area").MustText()
		require.Contains(t, forgotPasswordHeader, "Reset Password")
		emailAddressInput := page.MustElement(".headerless-input")
		require.Condition(t, emailAddressInput.MustVisible)

		// Tries resetting password for account that exists and is activated
		page.MustElement(".headerless-input").MustClick().MustInput(emailAddress)
		page.MustElement(".forgot-area__content-area__container__button").MustClick()
		passwordResetMessage := page.MustElement(".notification-wrap__text-area__message").MustText()
		require.Contains(t, passwordResetMessage, "Please look for instructions at your email")

	})
}

func TestLogin_UnverifiedNonexistentAccount(t *testing.T) {
	t.Skip("does not work")
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		emailAddress := "unverified@andnonexistent.test"
		password := "qazwsx123"
		page := browser.MustPage(loginPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// login with unverified/nonexistent email
		page.MustElement("div.login-area__input-wrapper:nth-child(2)").MustClick().MustInput(emailAddress)
		page.MustElement("div.login-area__input-wrapper:nth-child(3)").MustClick().MustInput(password)
		page.Keyboard.MustPress(input.Enter)

		// check for error message for unverified/nonexistent
		invalidEmailPasswordMessage := page.MustElement(".notification-wrap__text-area__message").MustText()
		require.Contains(t, invalidEmailPasswordMessage, "Your email or password was incorrect, please try again")

	})
}
