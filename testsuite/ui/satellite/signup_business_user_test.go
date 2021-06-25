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

func TestBusinessUserCanSignUp(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		companyName := "company"
		positionTitle := "tester"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		// First time User signup
		page.MustElement(".register-area__content-area__toggle__professional").MustClick()
		page.MustElement(".headerless-input").MustInput(fullName)
		page.MustElement("[placeholder=\"example@email.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Acme Corp.\"]").MustInput(companyName)
		page.MustElement("[placeholder=\"Position Title\"]").MustInput(positionTitle)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElementX("(//*[@class=\"checkmark\"])[2]").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
	})
}
