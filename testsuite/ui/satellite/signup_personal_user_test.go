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

func TestPersonalUserCanSignUp(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// First time User signup
		page.MustElement("[aria-roledescription=name]").MustInput(fullName)
		page.MustElement("[aria-roledescription=email]").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password]").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
	})
}
