// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	assert2 "github.com/stretchr/testify/assert"
	"github.com/zeebo/assert"

	"storj.io/common/testcontext"
	"storj.io/storj/integration/ui/uitest"
	"storj.io/storj/private/testplanet"
)

func TestE2eUserCreateLoginAccessInBrowser(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"
		email := "test@dtsff4xxxrvfdwqd0.com"
		nickname := "test2"
		pass := "123qwe"
		accessGrantName := "qwerty"
		page := browser.MustPage(loginPageURL)
		page.MustSetViewport(1350, 600, 1, false)

		// new user creation
		page.MustElement("a.login-area__content-area__forgot-container__link.register-link").MustClick()
		time.Sleep(1 * time.Second)
		page.MustElementX("(//*[@class=\"headerless-input\"])[1]").MustWaitStable().MustWaitEnabled().MustInput(nickname)
		page.MustElementX("(//*[@class=\"headerless-input\"])[2]").MustInput(email)
		page.MustElementX("(//*[@type=\"password\"])[1]").MustInput(pass)
		page.MustElementX("(//*[@type=\"password\"])[2]").MustInput(pass)
		page.MustElement("span.checkmark").MustClick()
		page.MustElement("p.register-area__input-area__container__button").MustClick()

		// checking elements on congrats screen
		header := page.MustElement("h2.register-success-area__form-container__title").MustText()
		assert.Equal(t, header, "You're almost there!")
		textCheck := page.MustElement("p.register-success-area__form-container__sub-title").MustText()
		assert.Equal(t, textCheck, "Check your email to confirm your account and get started.")
		counterText := page.MustElement("p.register-success-area__form-container__text").MustText()
		assert2.Contains(t, counterText, "Didn't receive a verification email?")
		resendButton := page.MustElement("div.register-success-area__form-container__button-container").MustText()
		assert.DeepEqual(t, resendButton, "Resend Email")
		loginButton := page.MustElement("a.register-success-area__form-container__contact__link").MustText()
		assert.DeepEqual(t, loginButton, "Contact our support team")

		// continue to login and onboarding flow
		page.MustElement("a.register-area__input-area__login-container__link").MustClick()
		page.MustElement(".headerless-input").MustInput(email)
		page.MustElement("[type=password]").MustInput(pass)
		page.MustElement("p.login-area__content-area__container__button").MustClick()

		// check welcome to Storj screen elements
		welcomeHeader := page.MustElement("h2.overview-area__header").MustText()
		assert.DeepEqual(t, welcomeHeader, "Welcome to Storj DCS")
		encriptionBanner := page.MustElement("p.overview-area__label.continue-label.server-side-label").MustText()
		assert.Equal(t, "Server-Side Encrypted", encriptionBanner)
		contBrowserHeader := page.MustElement("h3.overview-area__continue__header").MustText()
		assert.Equal(t, "Upload in Browser", contBrowserHeader)
		contBrowserText := page.MustElement("p.overview-area__continue__text").MustText()
		assert.Equal(t, "Start uploading files in the browser and instantly see how your data gets distributed over our global storage network. You can always use other upload methods later.", contBrowserText)
		contBrowserButton := page.MustElementX("(//*[@class=\"label\"])[1]").MustText()
		assert.Equal(t, "Upload in Browser", contBrowserButton)
		contBrowserImage := *page.MustElement(".overview-area__continue__img").MustAttribute("src")
		assert2.Contains(t, contBrowserImage, "/static/dist/img/continue-bg.293860b8.png")
		waysHeader := page.MustElement("h3.overview-area__second-header").MustText()
		assert.Equal(t, "More Ways To Upload", waysHeader)
		gatewayBanner := page.MustElement(".overview-area__label.server-side-label").MustText()
		assert.Equal(t, "Server-Side Encrypted", gatewayBanner)
		gatewayHeader := page.MustElement("h4.overview-area__path-section__title").MustText()
		assert.Equal(t, "GatewayMT", gatewayHeader)
		gatewayText := page.MustElement("p.overview-area__path-section__text").MustText()
		assert.Equal(t, "Backwards S3-Compatible API for uploading data programatically.", gatewayText)
		gatewayButton := page.MustElementX("(//*[@class=\"label\"])[2]").MustText()
		assert.Equal(t, "Continue", gatewayButton)
		uplinkBanner := page.MustElementX("(//*[@class=\"overview-area__label\"])[1]").MustText()
		assert.Equal(t, "End-to-End Encrypted", uplinkBanner)
		uplinkHeader := page.MustElementX("(//*[@class=\"overview-area__path-section__title\"])[2]").MustText()
		assert.Equal(t, "Uplink CLI", uplinkHeader)
		uplinkText := page.MustElementX("(//*[@class=\"overview-area__path-section__text\"])[2]").MustText()
		assert.Equal(t, "Natively installed client for interacting with the Storj Network.", uplinkText)
		uplinkButton := page.MustElementX("(//*[@class=\"label\"])[3]").MustText()
		assert.Equal(t, "Continue", uplinkButton)
		rcloneBanner := page.MustElementX("(//*[@class=\"overview-area__label\"])[2]").MustText()
		assert.Equal(t, "End-to-End Encrypted", rcloneBanner)
		rcloneHeader := page.MustElementX("(//*[@class=\"overview-area__path-section__title\"])[3]").MustText()
		assert.Equal(t, "Sync with Rclone", rcloneHeader)
		rcloneText := page.MustElementX("(//*[@class=\"overview-area__path-section__text\"])[3]").MustText()
		assert.Equal(t, "Map your filesystem to the decentralized cloud.", rcloneText)
		rcloneButton := page.MustElement(".overview-area__path-section__button")
		assert.Equal(t, "Continue", rcloneButton.MustText())
		rcloneLink := *page.MustElement("a.overview-area__path-section__button").MustAttribute("href")
		assert.Equal(t, "https://docs.storj.io/how-tos/sync-files-with-rclone", rcloneLink)

		// continue to next page
		page.MustElement("div.overview-area__path-section__button.container.blue-white").MustClick()

		// create an access grant screen - check elements
		createHeader := page.MustElement("h1.onboarding-access__title").MustText()
		assert.DeepEqual(t, createHeader, "Create an Access Grant")
		createText := page.MustElement("p.onboarding-access__sub-title").MustText()
		assert.DeepEqual(t, createText, "Access Grants are keys that allow access to upload, delete, and view your project’s data.")
		progressbar := page.MustElement("div.progress-bar").MustVisible()
		assert.True(t, progressbar)
		createName := page.MustElement("h1.name-step__title").MustText()
		assert.DeepEqual(t, createName, "Name Your Access Grant")
		nameText := page.MustElement("p.name-step__sub-title").MustText()
		assert.DeepEqual(t, nameText, "Enter a name for your new Access grant to get started.")
		accessTitle := page.MustElement("h3.label-container__main__label").MustText()
		assert.DeepEqual(t, accessTitle, "Access Grant Name")
		accessInput := *page.MustElement("input.headered-input").MustAttribute("placeholder")
		assert.Equal(t, accessInput, "Enter a name here...")
		nextButton := page.MustElement("div.container").MustText()
		assert.DeepEqual(t, nextButton, "Next")

		// set access name and continue
		page.MustElement("input.headered-input").MustInput(accessGrantName)
		page.MustElement("div.container").MustClick()

		// Access permission - check elements
		perTitle := page.MustElement("h1.permissions__title").MustText()
		assert.Equal(t, perTitle, "Access Permissions")
		perText := page.MustElement("p.permissions__sub-title").MustText()
		assert.Equal(t, perText, "Assign permissions to this Access Grant.")
		amountCheckboxes := len(page.MustElementsX("//*[@type=\"checkbox\"]"))
		assert.Equal(t, amountCheckboxes, 4)
		downloadPerm := page.MustElementX("(//*[@class=\"permissions__content__left__item__label\"])[1]").MustText()
		assert.Equal(t, downloadPerm, "Download")
		uploadPerm := page.MustElementX("(//*[@class=\"permissions__content__left__item__label\"])[2]").MustText()
		assert.Equal(t, uploadPerm, "Upload")
		listPerm := page.MustElementX("(//*[@class=\"permissions__content__left__item__label\"])[3]").MustText()
		assert.Equal(t, listPerm, "List")
		deletePerm := page.MustElementX("(//*[@class=\"permissions__content__left__item__label\"])[4]").MustText()
		assert.Equal(t, deletePerm, "Delete")
		durationText := page.MustElement("p.permissions__content__right__duration-select__label").MustText()
		assert.Equal(t, durationText, "Duration")
		durationDrop := page.MustElement("div.duration-selection__toggle-container")
		assert.Equal(t, durationDrop.MustText(), "Forever")

		// check if datepicker appears
		durationDrop.MustClick()
		time.Sleep(1 * time.Second)
		datepicker := page.MustElement("div.duration-picker").MustVisible()
		assert.True(t, datepicker)
		durationDrop.MustClick()
		bucketsText := page.MustElement("p.permissions__content__right__buckets-select__label").MustText()
		assert.Equal(t, bucketsText, "Buckets")
		bucketsDrop := page.MustElement("div.buckets-selection")
		assert.Equal(t, bucketsDrop.MustText(), "All")
		conBro := page.MustElement("div.permissions__button.container")
		assert.Equal(t, conBro.MustText(), "Continue in Browser")
		conCLI := page.MustElement(".permissions__cli-link")
		assert.Equal(t, conCLI.MustText(), "Continue in CLI")

		// continue in Browser
		conBro.MustClick()

		// check success notification
		broNotification := page.MustElement("p.notification-wrap__text-area__message").MustText()
		assert.Equal(t, "Permissions were set successfully", broNotification)
		time.Sleep(2 * time.Second)

		// encryption passphrase screen elements checking
		passCheckbox := page.MustElement("input#pass-checkbox")
		passCheckbox.MustClick()
		encrHeader := page.MustElement("h1.generate-container__title").MustText()
		assert.Equal(t, encrHeader, "Encryption Passphrase")
		encrWarnTitle := page.MustElement(".generate-container__warning__title").MustText()
		assert.Equal(t, encrWarnTitle, "Save Your Encryption Passphrase")
		encrWarnMessage := page.MustElement("p.generate-container__warning__message").MustText()
		assert.Equal(t, encrWarnMessage, "You’ll need this passphrase to access data in the future. This is the only time it will be displayed. Be sure to write it down.")
		encrPassType := page.MustElement("p.generate-container__choosing__label")
		assert.Equal(t, encrPassType.MustText(), "Passphrase")
		generateTab := page.MustElement("p.generate-container__choosing__right__option.left-option")
		assert.Equal(t, generateTab.MustText(), "Generate Phrase")
		createTab := page.MustElementX("(//*[@class=\"generate-container__choosing__right__option\"])")
		assert.Equal(t, createTab.MustText(), "Enter Phrase")

		// checkout to Generate passphrase and check success notification
		passPhrase := page.MustElement("p.generate-container__value-area__mnemonic__value")
		passPhrase.MustVisible()
		passCopy := page.MustElement("div.generate-container__value-area__mnemonic__button.container")
		assert.Equal(t, passCopy.MustText(), "Copy")
		passCheckboxText := page.MustElement("label.generate-container__warning__check-area").MustText()
		assert.DeepEqual(t, passCheckboxText, "Yes, I wrote this down or saved it somewhere.")
		passNext := page.MustElement("div.generate-container__next-button.container")
		assert.Equal(t, passNext.MustText(), "Next")
		time.Sleep(1 * time.Second)
		page.MustElement("div.generate-container__next-button.container").MustWaitVisible().MustWaitEnabled().MustWaitInteractable().MustClick()
		notification := page.MustElement("p.notification-wrap__text-area__message").MustWaitEnabled().MustText()
		assert.Equal(t, "Access Grant was generated successfully", notification)

		// check AG
		agrantTitle := page.MustElement("h1.generate-grant__title").MustText()
		assert.DeepEqual(t, agrantTitle, "Generate Access Grant")
		agrantWarnTitle := page.MustElement("h2.generate-grant__warning__header__label").MustText()
		assert.DeepEqual(t, agrantWarnTitle, "This Information is Only Displayed Once")
		agrantWarnMessage := page.MustElement(".generate-grant__warning__message").MustText()
		assert.Equal(t, agrantWarnMessage, "Save this information in a password manager, or wherever you prefer to store sensitive information.")
		agrantAreaTitle := page.MustElement("h3.generate-grant__grant-area__label").MustText()
		assert.Equal(t, agrantAreaTitle, "Access Grant")
		agrantKey := page.MustElement(".generate-grant__grant-area__container__value").MustVisible()
		assert.True(t, agrantKey)
		agrantCopy := page.MustElement("div.generate-grant__grant-area__container__button.container")
		assert.Equal(t, agrantCopy.MustText(), "Copy")
		downloadButton := page.MustElementX("(//*[@class=\"generate-grant__grant-area__container__button container\"])[2]")
		assert.Equal(t, downloadButton.MustText(), "Download")
		doneButton := page.MustElementX("(//*[@class=\"generate-grant__done-button container\"])[1]")
		assert.Equal(t, doneButton.MustText(), "Done")

		// finish access grant generation and check if AG is in list
		doneButton.MustClick()
		page.MustWindowMaximize().MustElement("#app > div > div > div.dashboard__wrap__main-area > div.navigation-area.regular-navigation > a:nth-child(4)").MustWaitVisible().MustClick()
		createdAGInList := page.MustElement("p.name").MustText()
		assert.Equal(t, createdAGInList, accessGrantName)

	})
}
