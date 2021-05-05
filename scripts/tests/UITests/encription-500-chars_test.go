package UITests

import (
	"gotest.tools/assert"
	"testing"
	"time"
)

	func Test_warning_server_side_encription (t *testing.T){
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElementX("(//*[@class=\"navigation-area__item-container\"])[1]").MustClick()
		// check texts on warning page
		warningHeader:= page.MustElement("h1.warning-view__container__title").MustText()
		assert.Equal(t, "Object Browser", warningHeader)
		warningSubTitle:= page.MustElement("h2.warning-view__container__message-container__sub-title").MustText()
		assert.Equal(t, "The object browser uses server side encryption.",warningSubTitle)
		warningMessage:= page.MustElement(".warning-view__container__message-container__message").MustText()
		assert.Equal(t, "If you want to use our product with only end-to-end encryption, you may want to skip this feature.", warningMessage)
		returnToDashboard:= page.MustElement(".warning-view__container__buttons-area > div:nth-child(1)")
		continueButton:= page.MustElement(".warning-view__container__buttons-area > div:nth-child(2)")
		assert.Equal(t, "Return to dashboard",returnToDashboard.MustText())
		assert.Equal(t, "Continue",continueButton.MustText())
	}

	func Test_create_500_chars (t *testing.T){
		passphrase500chars:= "1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop"
		bucketName:= "111111"
		//page, browser:=create_AG(passphrase500chars)
	page, browser := login_to_account()
	defer browser.MustClose()

	page.MustElementX("(//*[@class=\"navigation-area__item-container\"])[1]").MustClick()
	page.MustElement("div.container:nth-of-type(2)").MustClick()
	page.MustElement("p.generate-container__choosing__right__option:nth-of-type(2)").MustClick()
	page.MustElement("input.headered-input").MustInput(passphrase500chars)
	page.MustElement("div.generate-container__next-button").MustClick()
	page.MustElement(".enter-pass__container__textarea__input").MustInput(passphrase500chars)
	page.MustElement(".enter-pass__container__next-button").MustClick().MustWaitInvisible()
	time.Sleep(1*time.Second)
	page.MustElement(".buckets-view__title-area__button").MustWaitVisible().MustClick()
	page.MustElement("input.headered-input").MustInput(bucketName)
	page.MustElement("div.container").MustClick().MustWaitInvisible()
	time.Sleep(2*time.Second)
	page.MustElement("p.back").MustClick().MustWaitInvisible()
	nameFromUI:= page.MustElement("p.bucket-item__name__value").MustText()
	assert.Equal(t, bucketName,nameFromUI)
}

	func Test_Bucketname_cyrillic_symbols (t *testing.T){
		bucketName:= "фвап"
		passphrase:="123321"
		page, browser := login_to_account()
		defer browser.MustClose()

		page.MustElementX("(//*[@class=\"navigation-area__item-container\"])[1]").MustClick()
		page.MustElement("div.container:nth-of-type(2)").MustClick()
		page.MustElement("p.generate-container__choosing__right__option:nth-of-type(2)").MustClick()
		page.MustElement("input.headered-input").MustInput(passphrase)
		page.MustElement("div.generate-container__next-button").MustClick()
		page.MustElement(".enter-pass__container__textarea__input").MustInput(passphrase)
		page.MustElement(".enter-pass__container__next-button").MustClick().MustWaitInvisible()
		time.Sleep(1*time.Second)
		page.MustElement(".buckets-view__title-area__button").MustWaitVisible().MustClick()
		page.MustElement("input.headered-input").MustInput(bucketName)
		page.MustElement("div.container").MustClick()
		message:=page.MustElement("p.objects-popup__container__info__msg").MustText()
		assert.Equal(t, "Only lowercase alphanumeric characters are allowed.", message)
		validationError:= page.MustElement("h3.label-container__main__error").MustText()
		assert.Equal(t, "Name must include only lowercase latin characters", validationError)
	}
