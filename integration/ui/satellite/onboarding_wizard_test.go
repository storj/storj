// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/integration/ui/uitest"
	"storj.io/storj/private/testplanet"
)

func TestOnboardingWizard(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// First time User signup
		page.MustElement(".headerless-input").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
		// Login as first time User
		page.MustElement(".register-area__content-area__login-container__link").MustClick()
		page.MustElement(".headerless-input").MustInput(emailAddress)
		page.MustElement("[type=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		// Checking out onboarding process for "Rclone" and other "Integrations"
		wait := page.MustWaitOpen()
		page.MustElement("a.overview-area__path-section__button").MustClick()
		syncFilesWithRclonePage := wait()
		require.Equal(t, "https://docs.storj.io/how-tos/sync-files-with-rclone", syncFilesWithRclonePage.MustInfo().URL)
		syncFilesWithRclonePage.MustClose()
		wait2 := page.MustWaitOpen()
		page.MustElement(".overview-area__integrations-button").MustClick()
		storjOtherIntegrationsPage := wait2()
		storjIntegrationsTitle := storjOtherIntegrationsPage.MustElement(".blog-heading").MustText()
		require.Contains(t, storjIntegrationsTitle, "Integrations")
		featuredIntegrationsSubHeader := storjOtherIntegrationsPage.MustElement(".marketplace-content-title:nth-child(1)").MustText()
		require.Contains(t, featuredIntegrationsSubHeader, "Featured Integrations")
		storjOtherIntegrationsPage.MustClose()
		page.MustElement(".overview-area__skip-button").MustClick()
		dashboardTitle := page.MustElement(".dashboard-area__header-wrapper__title").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")
	})
}
