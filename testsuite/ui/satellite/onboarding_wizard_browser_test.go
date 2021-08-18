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

func TestOnboardingWizardBrowser(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"

		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// first time User signup
		page.MustElement("[placeholder=\"Enter Full Name\"]").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// first time user log in
		page.MustElement("[href=\"/login\"]").MustClick()
		page.MustElement("[type=text]").MustInput(emailAddress)
		page.MustElement("[type=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)

		// testing onboarding workflow browser
		page.MustElement(".label").MustClick()
		objectBrowserWarning := page.MustElement(".warning-view__container").MustText()
		require.Contains(t, objectBrowserWarning, "The object browser uses server side encryption.")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		encryptionPassphraseWarningTitle := page.MustElement(".encrypt__container__save__title").MustText()
		require.Contains(t, encryptionPassphraseWarningTitle, "Save your encryption passphrase")
		customPassphrase := page.MustElement(".encrypt__container__header__row__right__enter")
		customPassphraseLabel := customPassphrase.MustText()
		require.Contains(t, customPassphraseLabel, "Enter Your Own Passphrase")
		customPassphrase.MustClick()

		page.MustElement("[type=text]").MustInput("password123")
		page.MustElement(".label").MustClick()

		// Buckets Page
		bucketsTitle := page.MustElement(".buckets-view__title-area").MustText()
		require.Contains(t, bucketsTitle, "Buckets")
	})
}
