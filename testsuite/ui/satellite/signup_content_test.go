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

func TestSignUpContent(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		invalidEmailAddress := "test@email"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// Satellites dropdown
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()
		usSatLink := page.MustElement("[aria-roledescription=satellite-option-US1]").MustText()
		require.Contains(t, usSatLink, "US1")
		euSatLink := page.MustElement("[aria-roledescription=satellite-option-EU1]").MustText()
		require.Contains(t, euSatLink, "EU1")
		apSatLink := page.MustElement("[aria-roledescription=satellite-option-AP1]").MustText()
		require.Contains(t, apSatLink, "AP1")
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()

		// User signup with invalid email
		page.MustElement("[aria-roledescription=name]").MustInput(fullName)
		page.MustElement("[aria-roledescription=email]").MustInput(invalidEmailAddress)
		page.MustElement("[aria-roledescription=password]").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		invalidEmailMessage := page.MustElement("[aria-roledescription=email-error]").MustText()
		require.Contains(t, invalidEmailMessage, "Invalid Email")

		// User signup with no email or password
		page.MustElement("[aria-roledescription=email]").MustSelectAllText().MustInput("")
		page.MustElement("[aria-roledescription=password]").MustSelectAllText().MustInput("")
		page.MustElement("[aria-roledescription=retype-password]").MustSelectAllText().MustInput("")
		page.Keyboard.MustPress(input.Enter)
		invalidEmailMessage1 := page.MustElement("[aria-roledescription=email-error]").MustText()
		require.Contains(t, invalidEmailMessage1, "Invalid Email")
		invalidPasswordMessage := page.MustElement("[aria-roledescription=password-error]").MustText()
		require.Contains(t, invalidPasswordMessage, "Invalid Password")
	})
}
