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

func TestLoginUnverifiedNonexistentAccount(t *testing.T) {
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

		//check for error message for unverified/nonexistent
		invalidEmailPasswordMessage := page.MustElement(".notification-wrap__text-area__message").MustText()
		require.Contains(t, invalidEmailPasswordMessage, "Your email or password was incorrect, please try again")

	})
}
