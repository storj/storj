package UITests

import (
	"gotest.tools/assert"
	"testing"
)
	var passphrase500chars = "1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop"
	var bucketName = "111111"

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
	page, browser := login_to_account()
	defer browser.MustClose()

	page.MustElementX("(//*[@class=\"navigation-area__item-container\"])[1]").MustClick()
	page.MustElement("div.container:nth-of-type(2)").MustClick()
	page.MustElement("p.generate-container__choosing__right__option:nth-of-type(2)").MustClick()
	page.MustElement("input.headered-input").MustInput(passphrase500chars)
	page.MustElement("div.generate-container__next-button").MustClick()
	page.MustElement(".enter-pass__container__textarea__input").MustInput(passphrase500chars)
	page.MustElement(".enter-pass__container__next-button").MustClick()
	page.MustElement(".buckets-view__title-area__button").MustClick()
	page.MustElement("input.headered-input").MustInput(bucketName)
	page.MustElement("div.container").MustClick().MustWaitInvisible()
	nameFromUI:= page.MustElement("p.bucket-item__name__value").MustText()
	assert.Equal(t, bucketName,nameFromUI)
}
