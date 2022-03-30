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

func TestSignup_BusinessUser(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.test"
		password := "qazwsx123"
		companyName := "company"
		positionTitle := "tester"

		page := openPage(browser, signupPageURL)

		// First time User signup
		page.MustElement("[aria-roledescription=professional-label]").MustClick()
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=company-name] input").MustInput(companyName)
		page.MustElement("[aria-roledescription=position] input").MustInput(positionTitle)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElementX("(//*[@class=\"checkmark\"])[2]").MustClick()
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
	})
}

func TestSignup_Content(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		password := "qazwsx123"

		page := openPage(browser, signupPageURL)

		// Satellites dropdown
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()
		usSatLink := page.MustElement("[href=\"https://us1.storj.io/signup\"]").MustText()
		require.Contains(t, usSatLink, "US1")
		euSatLink := page.MustElement("[href=\"https://eu1.storj.io/signup\"]").MustText()
		require.Contains(t, euSatLink, "EU1")
		apSatLink := page.MustElement("[href=\"https://ap1.storj.io/signup\"]").MustText()
		require.Contains(t, apSatLink, "AP1")
		page.MustElement("[aria-roledescription=satellites-dropdown]").MustClick()
		waitVueTick(page)

		// User signup with invalid email
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		invalidEmailAddress := "t@t@t.test"
		page.MustElement("[aria-roledescription=email] input").MustInput(invalidEmailAddress)
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		invalidEmailMessage := page.MustElement("[aria-roledescription=email] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidEmailMessage, "Invalid Email")

		page.MustElement("[aria-roledescription=email] input").MustSelectAllText().MustInput("")

		// User signup with no email or password
		page.MustElement("[aria-roledescription=password] input").MustSelectAllText().MustInput("")
		page.MustElement("[aria-roledescription=retype-password] input").MustSelectAllText().MustInput("")
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		invalidEmailMessage1 := page.MustElement("[aria-roledescription=email] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidEmailMessage1, "Invalid Email")
		invalidPasswordMessage := page.MustElement("[aria-roledescription=password] [aria-roledescription=error-text]").MustText()
		require.Contains(t, invalidPasswordMessage, "Invalid Password")

		validEmailAddresses := []string{
			"тест@тест.test ",
			" अजअज@अज.test",
			" test@email.test ",
		}
		for i, e := range validEmailAddresses {
			page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
			page.MustElement("[aria-roledescription=password] input").MustInput(password)
			page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
			page.MustElement("[aria-roledescription=email] input").MustInput(e)
			if i != 0 {
				page.MustElement(".checkmark").MustClick()
			}
			page.Keyboard.MustPress(input.Enter)
			waitVueTick(page)

			successTitle := page.MustElement("[aria-roledescription=title]").MustText()
			require.Contains(t, successTitle, "You're almost there!")

			page.MustElement("[href=\"/login\"]").MustClick()
			page.MustElement("[href=\"/signup\"]").MustClick()
		}
	})
}

func TestSignup_PersonalUser(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.test"
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
	})
}

func TestSignup_TwiceWithSameEmail(t *testing.T) {
	t.Skip("does not work")
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "signuptwice@test.mail"
		password := "qazwsx123"
		page := browser.MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// First time User signup
		page.MustElement("[placeholder=\"Enter Full Name\"]").MustInput(fullName)
		page.MustElement("[placeholder=\"user@example.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement(".register-success-area__form-container__title").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// Go back to registration page by clicking on login link and then registration link
		page.MustElement("a.register-success-area__login-link").MustClick()
		page.MustElement("a.login-area__content-area__register-link").MustClick()

		// Second time User signup with same email, check for error message "This email is already in use; try another"
		page.MustElement("[placeholder=\"Enter Full Name\"]").MustInput(fullName)
		page.MustElement("[placeholder=\"user@example.com\"]").MustInput(emailAddress)
		page.MustElement("[placeholder=\"Enter Password\"]").MustInput(password)
		page.MustElement("[placeholder=\"Retype Password\"]").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		require.Contains(t, page.MustElement(".notification-wrap__text-area__message").MustText(), "This email is already in use; try another")
	})
}
