// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestOnboardingWizardBrowser(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.test"
		password := "qazwsx123"

		page := openPage(browser, signupPageURL)

		// first time User signup
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)
		confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// first time user log in
		page.MustElement("[href=\"/login\"]").MustClick()
		waitVueTick(page)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		// testing onboarding workflow browser
		wait := page.MustWaitRequestIdle()
		page.MustElementX("(//span[text()=\"Continue in web\"])").MustClick()
		wait()

		// Buckets Page
		bucketsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, bucketsTitle, "Buckets")
		page.Race().ElementR("p", "demo-bucket").MustHandle(func(el *rod.Element) {
			el.MustClick()
			waitVueTick(page)
		}).MustDo()

		// Passphrase screen
		encryptionTitle := page.MustElement("[aria-roledescription=objects-title]").MustText()
		require.Contains(t, encryptionTitle, "The object browser uses server side encryption.")
		customPassphrase := page.MustElement("[aria-roledescription=enter-passphrase-label]")
		customPassphraseLabel := customPassphrase.MustText()
		require.Contains(t, customPassphraseLabel, "Enter your own passphrase")
		customPassphrase.MustClick()
		waitVueTick(page)
		page.MustElement("[aria-roledescription=passphrase] input").MustInput("password123")
		page.MustElement(".checkmark").MustClick()
		waitVueTick(page)
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		waitVueTick(page)

		// Verify that browser component has loaded and that the dropzone is present
		page.MustElementR("p", "Drop Files Here to Upload")
	})
}
