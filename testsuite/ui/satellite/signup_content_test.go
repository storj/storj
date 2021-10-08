// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

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

		page := openPage(browser, signupPageURL)

		// Satellites dropdown
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()
		usSatLink := page.MustElement("[href=\"https://us1.storj.io/signup\"]").MustText()
		require.Contains(t, usSatLink, "US1")
		euSatLink := page.MustElement("[href=\"https://eu1.storj.io/signup\"]").MustText()
		require.Contains(t, euSatLink, "EU1")
		apSatLink := page.MustElement("[href=\"https://ap1.storj.io/signup\"]").MustText()
		require.Contains(t, apSatLink, "AP1")
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()
		waitVueTick(page)

		// User signup with invalid email
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=email] input").MustInput(invalidEmailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		invalidEmailMessage := page.MustElement("[aria-roledescription=email] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidEmailMessage, "Invalid Email")

		// User signup with no email or password
		page.MustElement("[aria-roledescription=email] input").MustSelectAllText().MustInput("")
		page.MustElement("[aria-roledescription=password] input").MustSelectAllText().MustInput("")
		page.MustElement("[aria-roledescription=retype-password] input").MustSelectAllText().MustInput("")
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		invalidEmailMessage1 := page.MustElement("[aria-roledescription=email] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidEmailMessage1, "Invalid Email")
		invalidPasswordMessage := page.MustElement("[aria-roledescription=password] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidPasswordMessage, "Invalid Password")
	})
}
