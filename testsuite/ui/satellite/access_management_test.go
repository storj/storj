// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestCreateAccessGrant(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		accessGrantName := "myTestAccessGrant"

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// continue to dashboard
		page.MustElementR("span", "Continue in web ->").MustClick()
		waitVueTick(page)

		// Access Management Page
		page.MustElement("[href=\"/access-grants\"]").MustClick()
		waitVueTick(page)

		// Make access grant
		page.MustElementR("span", "Create Access Grant").MustClick()
		waitVueTick(page)
		page.MustElement("#access-grant-check").MustClick()
		page.MustElement("[placeholder=\"Input Access Name\"]").MustInput(accessGrantName)
		page.MustElement("#permissions__all-check").MustClick()
		page.MustElementR(".label", "Encrypt My Access").MustClick()
		waitVueTick(page)

		// Encrypt
		page.MustElement("[placeholder=\"Input Your Passphrase\"]").MustInput("my test passphrase")
		page.MustElementR(".label", "Copy to clipboard").MustClick()
		page.MustElement("[type=\"checkbox\"]").MustClick()
		page.MustElementR(".label", "Create my Access").MustClick()

		accessGrant := page.MustElement(".access-grant__modal-container__generated-credentials__text").MustText()
		require.NotEmpty(t, accessGrant)

		page.MustElement(".access-grant__modal-container__header-container__close-cross-container").MustClick()

		// Check that the new access is listed
		page.MustElementR(".name", accessGrantName)
	})
}

func TestCreateS3Credentials(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		s3CredsName := "myTestS3Creds"

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// continue to dashboard
		page.MustElementR("span", "Continue in web ->").MustClick()
		waitVueTick(page)

		// Access Management Page
		page.MustElement("[href=\"/access-grants\"]").MustClick()
		waitVueTick(page)

		// Make s3 creds
		page.MustElementR("span", "Create S3 Credentials").MustClick()
		waitVueTick(page)
		page.MustElement("#s3-check").MustClick()
		page.MustElement("[placeholder=\"Input Access Name\"]").MustInput(s3CredsName)
		page.MustElement("#permissions__all-check").MustClick()
		page.MustElementR(".label", "Encrypt My Access").MustClick()
		waitVueTick(page)

		// Encrypt
		page.MustElement("[placeholder=\"Input Your Passphrase\"]").MustInput("my test passphrase")
		page.MustElementR(".label", "Copy to clipboard").MustClick()
		page.MustElement("[type=\"checkbox\"]").MustClick()
		page.MustElementR(".label", "Create my Access").MustClick()
		waitVueTick(page)

		// Check credentials
		checkCredentials(t, page, "Access Key")
		checkCredentials(t, page, "Secret Key")
		checkCredentials(t, page, "Endpoint")

		page.MustElement(".access-grant__modal-container__header-container__close-cross-container").MustClick()

		// Check that the new access is listed
		page.MustElementR(".name", s3CredsName)
	})
}

func TestCreateCLIKeys(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		myCLIKey := "myCLIKey"

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// continue to dashboard
		page.MustElementR("span", "Continue in web ->").MustClick()
		waitVueTick(page)

		// Access Management Page
		page.MustElement("[href=\"/access-grants\"]").MustClick()
		waitVueTick(page)

		// Make cli creds
		page.MustElementR("span", "Create Keys for CLI").MustClick()
		waitVueTick(page)
		page.MustElement("#api-check").MustClick()
		page.MustElement("[placeholder=\"Input Access Name\"]").MustInput(myCLIKey)
		page.MustElement("#permissions__all-check").MustClick()
		page.MustElement(".access-grant__modal-container__footer-container__encrypt-button").MustClick()
		waitVueTick(page)

		// Check credentials
		checkCredentials(t, page, "Satellite Address")
		checkCredentials(t, page, "API Key")

		page.MustElement(".access-grant__modal-container__header-container__close-cross-container").MustClick()

		// Check that the new access is listed
		page.MustElementR(".name", myCLIKey)
	})
}

func TestCreateAccessRestricted(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		myS3Creds := "myS3Creds"

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// continue to dashboard
		page.MustElementR("span", "Continue in web ->").MustClick()
		waitVueTick(page)

		// Access Management Page
		page.MustElement("[href=\"/access-grants\"]").MustClick()
		waitVueTick(page)

		// Make s3 creds
		page.MustElementR("span", "Create S3 Credentials").MustClick()
		waitVueTick(page)
		page.MustElement("#s3-check").MustClick()
		page.MustElement("[placeholder=\"Input Access Name\"]").MustInput(myS3Creds)
		page.MustElement(".permissions-chevron-up").MustClick()
		page.MustElement("#permissions__Read-check").MustClick()
		page.MustElementR(".label", "Encrypt My Access").MustClick()
		waitVueTick(page)

		// Encrypt
		page.MustElement("[placeholder=\"Input Your Passphrase\"]").MustInput("my test passphrase")
		page.MustElementR(".label", "Copy to clipboard").MustClick()
		page.MustElement("[type=\"checkbox\"]").MustClick()
		page.MustElementR(".label", "Create my Access").MustClick()
		waitVueTick(page)

		// Check access key
		checkCredentials(t, page, "Access Key")
		checkCredentials(t, page, "Secret Key")
		checkCredentials(t, page, "Endpoint")

		page.MustElement(".access-grant__modal-container__header-container__close-cross-container").MustClick()

		// Check that the new access is listed
		page.MustElementR(".name", myS3Creds)
	})
}

func TestDeleteAccess(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// continue to dashboard
		page.MustElementR("span", "Continue in web ->").MustClick()
		waitVueTick(page)

		// Access Management Page
		page.MustElement("[href=\"/access-grants\"]").MustClick()
		waitVueTick(page)

		// Delete default access
		defaultAccess := page.MustElement(".name-container").MustText()
		page.MustElement(".ellipses").MustClick()
		page.MustElement(".popup-menu__popup-delete").MustClick()
		waitVueTick(page)

		page.MustElement("[placeholder=\"Type the name of the access\"]").MustInput(defaultAccess)
		page.MustElementR(".label", "Delete Access").MustClick()
		waitVueTick(page)

		// Check that no access exists
		page.MustElement(".access-grants-items2__empty-state__text")
	})
}

func checkCredentials(t *testing.T, page *rod.Page, label string) {
	credLabelText := page.MustElementR(".access-grant__modal-container__generated-credentials__label__text", label)
	credLabel, err := credLabelText.Parent()
	require.NoError(t, err)
	credField, err := credLabel.Next()
	require.NoError(t, err)
	has, credText, err := credField.Has(".access-grant__modal-container__generated-credentials__text")
	require.True(t, has)
	require.NoError(t, err)
	require.NotEmpty(t, credText.MustText())
}
