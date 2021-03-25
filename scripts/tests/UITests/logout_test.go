package UITests


import (
	"gotest.tools/assert"
	"testing"

)

func TestLogout(t *testing.T)  {
	page, browser := login_to_account()
	defer browser.MustClose()
	// We use css selector to get the search
	page.MustElement(".account-button__container__avatar").MustClick()
	page.MustElement(".account-dropdown__wrap__item-container").MustClick()

	//check title
	logo := page.MustElement("h1.login-area__title-container__title").MustText()
	assert.Equal(t,"Login to Storj",logo)
}
