package UITests

import (
	"fmt"
	"testing"
)

func Test_membersScreen (t *testing.T) {
	page, browser := login_to_account()
	defer browser.MustClose()
	page.MustElement("a.navigation-area__item-container:nth-of-type(4)").MustClick()

	membersHeaderText := page.MustElement("h1.team-header-container__title-area__title").MustText()
	fmt.Println(membersHeaderText)
	questionMark:= page.MustHas("svg.team-header-container__title-area__info-button__image")
	fmt.Println(questionMark)
	helper:= page.MustElement("svg.team-header-container__title-area__info-button__image").MustClick().MustElementX("//*[@class=\"info__message-box__text\"]").MustText()
	fmt.Println(helper)
	addmemberButton:= page.MustElement("div.button.container").MustText()
	fmt.Println(addmemberButton)
	searchPlaceholder:= page.MustElement("input.common-search-input").MustAttribute("placeholder")
	searchSizeMin:= page.MustElement("input.common-search-input").MustAttribute("style")
	page.MustElement("input.common-search-input").MustClick().MustInput("ffwefwefhg")
	searchSizeMax:= page.MustElement("input.common-search-input").MustAttribute("style")
	fmt.Println(*searchPlaceholder)
	fmt.Println(*searchSizeMin)
	fmt.Println(*searchSizeMax)
	// Output: Project Members
	// true
	// The only project role currently available is Admin, which gives full access to the project.
	// + Add
	// Search Team Members
	// width: 56px;
	// width: 540px;
}

