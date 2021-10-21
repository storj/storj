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

func TestNavigation(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"

		page := openPage(browser, signupPageURL)

		// first time User signs up
		page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
		page.MustElement(".checkmark").MustClick()
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)
		confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

		// first time user logs in
		page.MustElement("[href=\"/login\"]").MustClick()
		waitVueTick(page)
		page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
		page.MustElement("[aria-roledescription=password] input").MustInput(password)
		page.Keyboard.MustPress(input.Enter)
		// waitVueTick(page)

		// skip onboarding process
		page.MustElement("[href=\"/project-dashboard\"]").MustClick()
		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")

		// nav bar test
		// manage projects route
		page.MustElement("[aria-roledescription=project-selection]").MustClick()
		page.MustElementR("p", "Manage Projects").MustClick()
		waitVueTick(page)
		projectsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, projectsTitle, "Projects")

		// create new project route
		page.MustElement("[aria-roledescription=project-selection]").MustClick()
		page.MustElementR("p", "Create new").MustClick()
		waitVueTick(page)
		createProjectTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, createProjectTitle, "Create a Project")
		page.MustNavigateBack()

		// project dashboard route
		page.MustElementR("p", "Dashboard").MustClick()
		waitVueTick(page)
		dashboardTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle1, "Dashboard")

		// project dashboard route
		page.MustElementR("p", "Objects").MustClick()
		waitVueTick(page)
		objectsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, objectsTitle, "Object Browser")

		// access grants route
		page.MustElementR("p", "Access").MustClick()
		waitVueTick(page)
		accessGrantsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, accessGrantsTitle, "Access Grants")

		// project members route
		page.MustElementR("p", "Users").MustClick()
		waitVueTick(page)
		membersTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, membersTitle, "Project Members")

		// resources dropdown
		page.MustElementR("p", "Resources").MustClick()
		docsLinkTitle := page.MustElement("[href=\"https://docs.storj.io/\"] div h2").MustText()
		require.Contains(t, docsLinkTitle, "Docs")
		forumLinkTitle := page.MustElement("[href=\"https://forum.storj.io/\"] div h2").MustText()
		require.Contains(t, forumLinkTitle, "Forum")
		supportLinkTitle := page.MustElement("[href=\"https://supportdcs.storj.io/hc/en-us\"] div h2").MustText()
		require.Contains(t, supportLinkTitle, "Support")

		// quick start dropdown
		// create project route
		page.MustElementR("p", "Quick Start").MustClick()
		page.MustElement("[href=\"/create-project\"]").MustClick()
		waitVueTick(page)
		createProjectTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, createProjectTitle1, "Create a Project")
		page.MustNavigateBack()

		// create access grant route
		page.MustElement("[href=\"/access-grants/create-grant\"]").MustClick()
		waitVueTick(page)
		nameAGTitle := page.MustElement("[aria-roledescription=name-ag-title]").MustText()
		require.Contains(t, nameAGTitle, "Name Your Access Grant")
		page.MustElementX("(//span[text()=\"Cancel\"])").MustClick()

		// objects route
		page.MustElementR("p", "Quick Start").MustClick()
		page.MustElement("[href=\"/objects\"]").MustClick()
		waitVueTick(page)
		objectsTitle1 := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, objectsTitle1, "Object Browser")

		// onboarding cli flow route
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
		page.MustElementX("(//span[text()=\"< Back\"])").MustClick()
		waitVueTick(page)
		page.MustElement("[href=\"/project-dashboard\"]").MustClick()
		waitVueTick(page)

		// account dropdown
		// account settings route
		page.MustElement("[aria-roledescription=account-area]").MustClick()
		page.MustElementR("p", "Account Settings").MustClick()
		waitVueTick(page)
		settingsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, settingsTitle, "Account Settings")

		// billing route
		page.MustElement("[aria-roledescription=account-area]").MustClick()
		page.MustElementR("p", "Billing").MustClick()
		waitVueTick(page)
		pmTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, pmTitle, "Payment Method")

		// upgrade account popup
		page.MustElement("[aria-roledescription=account-area]").MustClick()
		page.MustElementR("p", "Upgrade Plan").MustClick()
		upgradeTitle := page.MustElement("[aria-roledescription=modal-title]").MustText()
		require.Contains(t, upgradeTitle, "Upgrade to Pro Account")
		page.MustElement(".close-cross-container").MustClick()

		// logout route
		page.MustElement("[aria-roledescription=account-area]").MustClick()
		page.MustElementR("p", "Logout").MustClick()
		waitVueTick(page)
		signInTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, signInTitle, "Sign In")
	})
}
