// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/integration/ui/uitest"
	"storj.io/storj/private/testplanet"
)

func TestOnboardingWizardBrowser(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		page := browser.Timeout(20 * time.Second).MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// first time User signup
		page.MustElement(".headerless-input").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
		page.MustElement(".register-area__content-area__login-container__link").MustClick()
		page.MustElement(".headerless-input").MustInput(emailAddress)
		page.MustElement("[type=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		// testing onboarding workflow browser
		page.MustElement(".label").MustClick()
		objectBrowserWarning := page.MustElement(".warning-view__container").MustText()
		require.Contains(t, objectBrowserWarning, "The object browser uses server side encryption.")
		page.MustElement(".container:nth-of-type(2)").MustClick()
		EncryptionPassphraseWarningTitle := page.MustElement(".generate-container__warning__title").MustText()
		require.Contains(t, EncryptionPassphraseWarningTitle, "Save Your Encryption Passphrase")
		customPassphrase := page.MustElement(".generate-container__choosing__right__option:nth-of-type(2)")
		customPassphrase.MustClick()
		existingPassphraseWarning := page.MustElement(".generate-container__enter-passphrase-box").MustText()
		require.Contains(t, existingPassphraseWarning, "Enter an Existing Passphrase")
		page.MustElement(".headered-input").MustInput("password123")
		page.MustElement(".label").MustClick()
		enterPasswordWarning := page.MustElement(".enter-pass__container__warning").MustText()
		require.Contains(t, enterPasswordWarning, "Would you like to access files in your browser?")
		page.MustElement(".enter-pass__container__textarea__input").MustInput("password123")
		page.MustElement(".label").MustClick()
		// Buckets Page
		bucketsTitle := page.MustElement(".buckets-view__title-area").MustText()
		require.Contains(t, bucketsTitle, "Buckets")
	})
}