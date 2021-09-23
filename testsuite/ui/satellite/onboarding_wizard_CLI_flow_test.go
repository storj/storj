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

func TestOnboardingWizardCLIFlow(t *testing.T) {
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

		// First time User log in
		page.MustElement("[href=\"/login\"]").MustClick()
		page.MustElement("[aria-roledescription=email]").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)

		// Testing onboarding workflow uplinkCLI method
		// Welcome screen
		page.MustElementX("(//span[text()=\"Continue in cli\"])").MustClick()
		page.MustWaitNavigation()

		// API key generated screen
		apiKeyGeneratedTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, apiKeyGeneratedTitle, "API Key Generated")
		satelliteAddress := page.MustElement("[aria-roledescription=satellite-address]").MustText()
		require.NotEmpty(t, satelliteAddress)
		apiKey := page.MustElement("[aria-roledescription=api-key]").MustText()
		require.NotEmpty(t, apiKey)
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		welcomeTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, welcomeTitle, "Welcome")
		page.MustElementX("(//span[text()=\"Continue in cli\"])").MustClick()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// API key generated screen
		cliInstallTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliInstallTitle, "Install Uplink CLI")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		apiKeyGeneratedTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, apiKeyGeneratedTitle1, "API Key Generated")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// CLI setup screen
		cliSetupTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliSetupTitle, "CLI Setup")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		cliInstallTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliInstallTitle1, "Install Uplink CLI")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// Create bucket screen
		createBucketTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, createBucketTitle, "Create a bucket")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		cliSetupTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliSetupTitle1, "CLI Setup")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// Ready to upload screen
		readyToUploadTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, readyToUploadTitle, "Ready to upload")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		createBucketTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, createBucketTitle1, "Create a bucket")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// List a bucket screen
		listTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, listTitle, "Listing a bucket")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		readyToUploadTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, readyToUploadTitle1, "Ready to upload")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// Download screen
		downloadTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, downloadTitle, "Download")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		listTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, listTitle1, "Listing a bucket")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// Share link screen
		shareLinkTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, shareLinkTitle, "Share a link")
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		page.MustWaitNavigation()
		downloadTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, downloadTitle1, "Download")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()
		page.MustWaitNavigation()

		// Success screen
		successTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, successTitle, "Wonderful")
		page.MustElementX("(//span[text()=\"Finish\"])").MustClick()
		page.MustWaitNavigation()
		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "My First Project Dashboard")
		page.MustNavigateBack()
		successTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, successTitle1, "Wonderful")
		page.MustElementX("(//button[contains(., 'Upgrade')])").MustClick()

		// Upgrade to pro account modal
		addPMModalTitle := page.MustElement("[aria-roledescription=modal-title]").MustText()
		require.Contains(t, addPMModalTitle, "Upgrade to Pro Account")
		page.MustElement(".close-cross-container").MustClick()

		// Dashboard screen
		dashboardTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle1, "My First Project Dashboard")
	})
}
