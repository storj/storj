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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsBinaryCTA := page.MustElement("[aria-roledescription=windows-binary-link]")
		windowsBinaryURL, err := windowsBinaryCTA.Attribute("href")
		require.NoError(t, err)
		require.Equal(t, "https://github.com/storj/storj/releases/latest/download/uplink_windows_amd64.zip", *windowsBinaryURL)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxAMDBinaryCTA := page.MustElement("[aria-roledescription=linux-amd-binary-link]")
		linuxAMDBinaryURL, err := linuxAMDBinaryCTA.Attribute("href")
		require.NoError(t, err)
		require.Equal(t, "https://github.com/storj/storj/releases/latest/download/uplink_linux_amd64.zip", *linuxAMDBinaryURL)
		linuxARMBinaryCTA := page.MustElement("[aria-roledescription=linux-arm-binary-link]")
		linuxARMBinaryURL, err := linuxARMBinaryCTA.Attribute("href")
		require.NoError(t, err)
		require.Equal(t, "https://github.com/storj/storj/releases/latest/download/uplink_linux_arm.zip", *linuxARMBinaryURL)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macOSBinaryCTA := page.MustElement("[aria-roledescription=macos-binary-link]")
		macOSBinaryURL, err := macOSBinaryCTA.Attribute("href")
		require.NoError(t, err)
		require.Equal(t, "https://github.com/storj/storj/releases/latest/download/uplink_darwin_amd64.zip", *macOSBinaryURL)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsCLISetupCmd := page.MustElement("[aria-roledescription=windows-cli-setup]").MustText()
		require.Equal(t, "./uplink.exe setup", windowsCLISetupCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxCLISetupCmd := page.MustElement("[aria-roledescription=linux-cli-setup]").MustText()
		require.Equal(t, "uplink setup", linuxCLISetupCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosCLISetupCmd := page.MustElement("[aria-roledescription=macos-cli-setup]").MustText()
		require.Equal(t, "uplink setup", macosCLISetupCmd)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsCreateBucketCmd := page.MustElement("[aria-roledescription=windows-create-bucket]").MustText()
		require.Equal(t, "./uplink.exe mb sj://cakes", windowsCreateBucketCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxCreateBucketCmd := page.MustElement("[aria-roledescription=linux-create-bucket]").MustText()
		require.Equal(t, "uplink mb sj://cakes", linuxCreateBucketCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosCreateBucketCmd := page.MustElement("[aria-roledescription=macos-create-bucket]").MustText()
		require.Equal(t, "uplink mb sj://cakes", macosCreateBucketCmd)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsUploadCmd := page.MustElement("[aria-roledescription=windows-upload]").MustText()
		require.Equal(t, "./uplink.exe cp <FILE_PATH> sj://cakes", windowsUploadCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxUploadCmd := page.MustElement("[aria-roledescription=linux-upload]").MustText()
		require.Equal(t, "uplink cp ~/Desktop/cheesecake.jpg sj://cakes", linuxUploadCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosUploadCmd := page.MustElement("[aria-roledescription=macos-upload]").MustText()
		require.Equal(t, "uplink cp ~/Desktop/cheesecake.jpg sj://cakes", macosUploadCmd)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsListCmd := page.MustElement("[aria-roledescription=windows-list]").MustText()
		require.Equal(t, "./uplink.exe ls sj://cakes", windowsListCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxListCmd := page.MustElement("[aria-roledescription=linux-list]").MustText()
		require.Equal(t, "uplink ls sj://cakes", linuxListCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosListCmd := page.MustElement("[aria-roledescription=macos-list]").MustText()
		require.Equal(t, "uplink ls sj://cakes", macosListCmd)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsDownloadCmd := page.MustElement("[aria-roledescription=windows-download]").MustText()
		require.Equal(t, "./uplink.exe cp sj://cakes/cheesecake.jpg <DESTINATION_PATH>/cheesecake.jpg", windowsDownloadCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxDownloadCmd := page.MustElement("[aria-roledescription=linux-download]").MustText()
		require.Equal(t, "uplink cp sj://cakes/cheesecake.jpg ~/Downloads/cheesecake.jpg", linuxDownloadCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosDownloadCmd := page.MustElement("[aria-roledescription=macos-download]").MustText()
		require.Equal(t, "uplink cp sj://cakes/cheesecake.jpg ~/Downloads/cheesecake.jpg", macosDownloadCmd)

		// Back and forth click test
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

		// OS tabs
		page.MustElement("[aria-roledescription=windows]").MustClick()
		windowsShareCmd := page.MustElement("[aria-roledescription=windows-share]").MustText()
		require.Equal(t, "./uplink.exe share --url sj://cakes/cheesecake.jpg", windowsShareCmd)
		page.MustElement("[aria-roledescription=linux]").MustClick()
		linuxShareCmd := page.MustElement("[aria-roledescription=linux-share]").MustText()
		require.Equal(t, "uplink share --url sj://cakes/cheesecake.jpg", linuxShareCmd)
		page.MustElement("[aria-roledescription=macos]").MustClick()
		macosShareCmd := page.MustElement("[aria-roledescription=macos-share]").MustText()
		require.Equal(t, "uplink share --url sj://cakes/cheesecake.jpg", macosShareCmd)

		// Back and forth click test
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
