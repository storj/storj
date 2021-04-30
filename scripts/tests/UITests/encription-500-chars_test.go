package UITests

import (
	"gotest.tools/assert"
	"testing"
)
	var passphrase500chars = "1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop1234567890!@#$%^&*()?><,./~`:zQWERTYUIOPqwertyuiop"
	var bucketName = "111111"

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
