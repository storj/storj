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

func TestOnboardingWizardCLISkipPassphrase(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"

		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// First time User signup
		page.MustElement("[placeholder=\"Enter Full Name\"]").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// First time User log in
		page.MustElement("[href=\"/login\"]").MustClick()
		page.MustElement("[type=text]").MustInput(emailAddress)
		page.MustElement("[type=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)

		// Testing onboarding workflow uplinkCLI method
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		encryptYourDataTitle := page.MustElement(".encrypt-container__title").MustText()
		require.Contains(t, encryptYourDataTitle, "Encrypt your data")
		page.MustElementX("(//*[@class=\"label\"])[3]").MustClick()

		apiKeyGeneratedTitle := page.MustElement(".flow-container__title").MustText()
		require.Contains(t, apiKeyGeneratedTitle, "API Key Generated")
		satelliteAddress := page.MustElementX("(//*[@class=\"value-copy__value\"])[1]").MustText()
		require.NotEmpty(t, satelliteAddress)
		apiKey := page.MustElementX("(//*[@class=\"value-copy__value\"])[2]").MustText()
		require.NotEmpty(t, apiKey)
		page.MustElementX("(//*[@class=\"label\"])[4]").MustClick()

		cliSetupTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, cliSetupTitle, "CLI Setup")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		generateAccessGrantTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, generateAccessGrantTitle, "Generate an Access Grant")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		createBucketTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, createBucketTitle, "Create a bucket")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		readyToUploadTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, readyToUploadTitle, "Ready to upload")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		listTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, listTitle, "Listing a bucket")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		downloadTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, downloadTitle, "Download")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		shareLinkTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, shareLinkTitle, "Share a link")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		successTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, successTitle, "Wonderful")
		page.MustElementX("(//*[@class=\"label\"])[2]").MustClick()

		addPMModalTitle := page.MustElement(".pm-area__add-modal__title").MustText()
		require.Contains(t, addPMModalTitle, "Add a Payment Method")
		page.MustElement(".close-cross-container").MustClick()

		dashboardTitle := page.MustElements("h1")[1].MustText()
		require.Contains(t, dashboardTitle, "My First Project Dashboard")
	})
}
