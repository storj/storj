// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func waitVueTick(page *rod.Page) {
	page.MustEval("VueNextTick()")
}

func openPage(browser *rod.Browser, url string) *rod.Page {
	page := browser.MustPage()
	page.MustSetViewport(1350, 600, 1, false)
	page.MustNavigate(url).MustWaitLoad()
	return page
}

func signUpWithUser(t *testing.T, planet *uitest.EdgePlanet, page *rod.Page) {
	signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
	fullName := "John Doe"
	emailAddress := "test@email.test"
	password := "qazwsx123"

	// navigate to signup page
	page.MustNavigate(signupPageURL)

	// first time User signup
	page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
	page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
	page.MustElement("[aria-roledescription=password] input").MustInput(password)
	page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
	page.MustElement(".checkmark").MustClick()
	page.Keyboard.MustPress(input.Enter)
	confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
	require.Contains(t, confirmAccountEmailMessage, "You're almost there!")
}

func loginWithUser(t *testing.T, planet *uitest.EdgePlanet, page *rod.Page) {
	loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
	emailAddress := "test@email.test"
	password := "qazwsx123"

	// navigate to login page
	page.MustNavigate(loginPageURL)

	// login
	page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
	page.MustElement("[aria-roledescription=password] input").MustInput(password)
	page.Keyboard.MustPress(input.Enter)
}

func generateEmptyFile(t *testing.T, ctx *testcontext.Context, name string, size memory.Size) string {
	path := ctx.File(name)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	require.NoError(t, f.Truncate(size.Int64()))
	return path
}
