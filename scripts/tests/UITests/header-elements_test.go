package UITests

import (
	"fmt"
	"testing"
	"time"
)

func Test_checkingElementsHeader(t *testing.T){
	page, browser := login_to_account()
	browser.SlowMotion(1 * time.Second)
	defer browser.MustClose()

	projects := page.MustElement("div.project-selection__toggle-container").MustText()
	fmt.Println(projects)
	resources := page.MustElement("div.resources-selection__toggle-container").MustText()
	fmt.Println(resources)
	settings := page.MustElement("div.settings-selection__toggle-container").MustText()
	fmt.Println(settings)
	user := page.MustHas("div.account-button__container__avatar")
	fmt.Println(user)
	logo := page.MustHas("div.header-container__left-area__logo-area")
	fmt.Println(logo)
	// Output:
	// Projects
	// Resources
	// Settings
	// true
	// true
}
