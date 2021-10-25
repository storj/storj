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

func TestRestartOnboardingWizard(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"

		page := openPage(browser, signupPageURL)

		// First time User signup
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// Login as first time User
		page.MustElement("[href=\"/login\"]").MustClick()
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		// Checking out skip of onboarding process
		page.MustElement("[href=\"/project-dashboard\"]").MustClick()
		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")

		// Testing restart tour functionality
		page.MustElementR("p", "Quick Start").MustClick()
		wait := page.MustWaitRequestIdle()
		page.MustElement("[href=\"/onboarding-tour/cli/api-key\"]").MustClick()
		wait()
		apiKeyGeneratedTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, apiKeyGeneratedTitle, "API Key Generated")
		page.Race().Element("[aria-roledescription=satellite-address]").MustHandle(func(el *rod.Element) {
			require.NotEmpty(t, el.MustText())
		}).MustDo()
		page.Race().Element("[aria-roledescription=api-key]").MustHandle(func(el *rod.Element) {
			require.NotEmpty(t, el.MustText())
		}).MustDo()
	})
}
