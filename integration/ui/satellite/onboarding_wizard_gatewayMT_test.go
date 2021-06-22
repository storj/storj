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

func TestOnboardingWizardGatewayMT(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// First time user signup
		page.MustElement(".headerless-input").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
		page.MustElement(".register-area__content-area__login-container__link").MustClick()
		page.MustElement(".headerless-input").MustInput(emailAddress)
		page.MustElement("[type=password]").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		// Testing onboarding workflow gatewayMT
		gatewayMTwizard := page.MustElement(".overview-area__path-section:nth-child(1) span")
		gatewayMTwizard.MustClick()
		createAnAccessGrantTitle := page.MustElement(".onboarding-access__title").MustText()
		require.Contains(t, createAnAccessGrantTitle, "Access Grant")
		page.MustElement(".headered-input").MustInput("grant123")
		page.MustElement(".label").MustClick()
		accessPermissions := page.MustElement(".permissions__title").MustText()
		require.Contains(t, accessPermissions,"Access Permissions")
		page.MustElement(".label").MustClick()
		encryptionPassphraseTitle := page.MustElement(".generate-container__title").MustText()
		require.Contains(t, encryptionPassphraseTitle,"Encryption Passphrase")
		EncryptionPassphraseWarningTitle := page.MustElement(".generate-container__warning__title").MustText()
		require.Contains(t, EncryptionPassphraseWarningTitle, "Save Your Encryption Passphrase")
		page.MustElement(".generate-container__warning__check-area__checkbox").MustClick()
		page.MustElement(".generate-container__next-button").MustClick()
		accessGrantWarning := page.MustElement(".generate-grant__warning").MustText()
		require.Contains(t, accessGrantWarning,"This Information is Only Displayed Once")
		page.MustElement(".generate-grant__done-button").MustClick()
		accessGrants := page.MustElement("a.navigation-area__item-container:nth-of-type(3)")
		accessGrants.MustClick()
		grantContainer := page.MustElement(".grants-item-container").MustText()
		require.Contains(t, grantContainer,"grant123")
	})
}
