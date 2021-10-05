// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestLoginToAccount(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		user := planet.Uplinks[0].User[planet.Satellites[0].ID()]

		page := browser.Timeout(10 * time.Second).MustPage(loginPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		page.MustElement("[aria-roledescription=email] input").MustInput(user.Email)
		page.MustElement("[aria-roledescription=password] input").MustInput(user.Password)
		page.Keyboard.MustPress(input.Enter)

		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")
	})
}
