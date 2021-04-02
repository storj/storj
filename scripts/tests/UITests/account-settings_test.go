package UITests

import (
	"github.com/bmizerany/assert"
	"github.com/go-rod/rod/lib/input"
	"testing"
	"time"
)

func TestAccountSettingsScreenElements(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.settings-selection.settings-selection").MustClick()
		// check list length
		listLength:= len(page.MustElements("div.settings-dropdown__choice"))
		assert.Equal(t, 2,listLength)

		accSettings:= page.MustElement("div.settings-dropdown__choice:nth-of-type(1)")
		billing:= page.MustElement("div.settings-dropdown__choice:nth-of-type(2)")
		assert.Equal(t, "Account Settings", accSettings.MustText())
		assert.Equal(t,"Billing", billing.MustText())

		accSettings.MustClick()

		header:= page.MustElement("h1.profile-container__title").MustText()
		assert.Equal(t, "Account Settings", header)

		editHeader:= page.MustElement("h2.profile-bold-text:nth-of-type(1)").MustText()
		assert.Equal(t,"Edit Profile",editHeader)
		editImage:= page.MustElement("div.profile-container__edit-profile__avatar").MustVisible()
		assert.T(t, editImage)
		editNotification:= page.MustElement("h3.profile-regular-text:nth-of-type(1)").MustText()
		assert.Equal(t,"This information will be visible to all users",editNotification)
		editProfileButton:=page.MustElement("svg.edit-svg:nth-of-type(1)").MustVisible()
		assert.T(t,editProfileButton)

		editpassHeader:= page.MustElementX("(//*[@class=\"profile-bold-text\"])[2]").MustText()
		assert.Equal(t,"Change Password",editpassHeader)
		editpassImage:= page.MustElement("svg.profile-container__secondary-container__img").MustVisible()
		assert.T(t, editpassImage)
		editpassNotification:= page.MustElementX("(//*[@class=\"profile-regular-text\"])[2]").MustText()
		assert.Equal(t,"6 or more characters",editpassNotification)
		editpassButton:=page.MustElementX("(//*[@class=\"edit-svg\"])[2]").MustVisible()
		assert.T(t,editpassButton)

		emailImage:= page.MustElementX("(//*[@class=\"profile-container__secondary-container__img\"])[2]").MustVisible()
		assert.T(t, emailImage)
		emailText:= page.MustElementX("(//*[@class=\"profile-bold-text email\"])").MustText()
		assert.Equal(t,login,emailText)

	}

	func TestAccountSettingsEditAcc(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.settings-selection.settings-selection").MustClick()

		page.MustElement("div.settings-dropdown__choice:nth-of-type(1)").MustClick()
		time.Sleep(3*time.Second)
		page.MustElement("svg.edit-svg").MustClick()

		header:=page.MustElement("h2.edit-profile-popup__form-container__main-label-text").MustText()
		assert.Equal(t,"Edit Profile", header)
		headerImage:=page.MustElement("div.edit-profile-popup__form-container__avatar").MustVisible()
		assert.T(t,headerImage)
		closeButton:= page.MustElement("div.edit-profile-popup__close-cross-container").MustVisible()
		assert.T(t,closeButton)
		fullnametext:= page.MustElementX("(//*[@class=\"label-container__main__label\"])[1]").MustText()
		assert.Equal(t,"Full Name",fullnametext)
		nameInput:= page.MustElementX("//*[@id=\"Full Name\"]").MustAttribute("placeholder")
		assert.Equal(t,"Enter Full Name",*nameInput)
		nicknametext:= page.MustElementX("(//*[@class=\"label-container__main__label\"])[2]").MustText()
		assert.Equal(t,"Nickname",nicknametext)
		nicknameInput:= page.MustElementX("//*[@id=\"Nickname\"]").MustAttribute("placeholder")
		assert.Equal(t,"Enter Nickname",*nicknameInput)
		cancelButton:= page.MustElement("div.container.transparent").MustText()
		assert.Equal(t,"Cancel",cancelButton)
		updateButton:= page.MustElementX("(//*[@class=\"container\"])").MustText()
		assert.Equal(t,"Update",updateButton)
	}
	func TestAccountSettingsEditFunc(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.settings-selection.settings-selection").MustClick()
//		avaFirst:= page.MustElement("h1.account-button__container__avatar__letter").MustText()

		page.MustElement("div.settings-dropdown__choice:nth-of-type(1)").MustClick()
		time.Sleep(3 * time.Second)
		page.MustElement("svg.edit-svg").MustClick()
		page.MustElementX("//*[@id=\"Full Name\"]").MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustInput(" ")
		page.MustElementX("(//*[@class=\"container\"])").MustClick()
		errorMessage:= page.MustElement("h3.label-container__main__error").MustText()
		assert.Equal(t,"Full name expected",errorMessage)

		page.MustElementX("//*[@id=\"Full Name\"]").MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustInput("zzz")
		page.MustElementX("(//*[@class=\"container\"])").MustClick()
		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
		assert.Equal(t,"Account info successfully updated!",notification)
		avaChanged:= page.MustElement("h1.account-button__container__avatar__letter").MustText()
		assert.Equal(t,"Z",avaChanged)

		page.MustElement("svg.edit-svg").MustClick()
		page.MustElementX("//*[@id=\"Nickname\"]").MustInput("яяя")
		page.MustElementX("(//*[@class=\"container\"])").MustClick()
		time.Sleep(2*time.Second)
		avaChanged2:= page.MustElement("h1.account-button__container__avatar__letter").MustText()
		assert.Equal(t,"Я",avaChanged2)

	}
