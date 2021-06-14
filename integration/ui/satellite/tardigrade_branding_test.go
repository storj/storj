// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/integration/ui/uitest"
	"storj.io/storj/private/testplanet"
)

func TestForTardigradeBranding(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		page := browser.Timeout(10 * time.Second).MustPage(loginPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// Check for "Reset Password" - It exists only on tardigrade branding login page
		resetPassword := page.MustElement("div.login-area div.login-area__content-area div.login-area__content-area div.login-area__content-area__forgot-container p.login-area__content-area__forgot-container__reset-msg > a.login-area__content-area__forgot-container__link").MustText()
		require.Contains(t, resetPassword, "Reset Password")
		// Check for "Need to create an account?" - It exists only on tardigrade branding login page
		createAccount := page.MustElement("div.login-area div.login-area__content-area div.login-area__content-area div.login-area__content-area__forgot-container > a.login-area__content-area__forgot-container__link.register-link").MustText()
		require.Contains(t, createAccount, "Need to create an account?")
		// Check for "Satellite Dropdown" - It exists only on tardigrade branding login page
		satellite := page.MustElement("div.login-area div.login-area__content-area div.login-area__content-area div.login-area__content-area__container div.login-area__content-area__container__title-area:nth-child(1) div.login-area__expand > span.login-area__expand__value").MustText()
		require.Contains(t, satellite, "")
	})
}
