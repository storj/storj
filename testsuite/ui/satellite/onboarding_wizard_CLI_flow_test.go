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
		page.MustElementX("(//span[text()=\"CONTINUE IN CLI\"])").MustClick()
		apiKeyGeneratedTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, apiKeyGeneratedTitle, "API Key Generated")
		satelliteAddress := page.MustElement("[aria-roledescription=satellite-address]").MustText()
		require.NotEmpty(t, satelliteAddress)
		apiKey := page.MustElement("[aria-roledescription=api-key]").MustText()
		require.NotEmpty(t, apiKey)
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		cliInstallTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliInstallTitle, "Install Uplink CLI")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		cliSetupTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, cliSetupTitle, "CLI Setup")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		generateAccessGrantTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, generateAccessGrantTitle, "Generate an Access Grant")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		createBucketTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, createBucketTitle, "Create a bucket")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		readyToUploadTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, readyToUploadTitle, "Ready to upload")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		listTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, listTitle, "Listing a bucket")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		downloadTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, downloadTitle, "Download")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		shareLinkTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, shareLinkTitle, "Share a link")
		page.MustElementX("(//span[text()=\"Next >\"])").MustClick()

		successTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, successTitle, "Wonderful")
		page.MustElementX("(//button[contains(., 'Upgrade')])").MustClick()

		addPMModalTitle := page.MustElement("[aria-roledescription=modal-title]").MustText()
		require.Contains(t, addPMModalTitle, "Upgrade to Pro Account")
		page.MustElement(".close-cross-container").MustClick()

		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "My First Project Dashboard")
	})
}
