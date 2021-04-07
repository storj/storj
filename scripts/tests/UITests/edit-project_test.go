package UITests

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
)

func Test_sideMenuEditProjectDroplist (t *testing.T){
	page, browser := login_to_account()
	defer browser.MustClose()
	text:= page.MustElement("div.edit-project").MustClick().MustElement("div.edit-project__dropdown").MustText()
	fmt.Println(text)
	// Output: Edit Details
}

func Test_editProjectScreen (t *testing.T) {
	page, browser := login_to_account()
	defer browser.MustClose()
	currentProjectNameFromSideMenu := page.MustElement("div.edit-project").MustText()
	page.MustElement("div.edit-project").MustClick().MustElement("div.edit-project__dropdown").MustClick()
	projectDetailsHeader := page.MustElement("h1.project-details__wrapper__container__title").MustText()
	fmt.Println(projectDetailsHeader)
	projectNameHeader := page.MustElement("p.project-details__wrapper__container__label:nth-of-type(1)").MustText()
	fmt.Println(projectNameHeader)
	descriptionHeader := page.MustElement("p.project-details__wrapper__container__label:nth-of-type(2)").MustText()
	fmt.Println(descriptionHeader)
	projectNameFromEditScreen := page.MustElement("p.project-details__wrapper__container__name-area__name").MustText()
	assert.Equal(t, currentProjectNameFromSideMenu, projectNameFromEditScreen)

	descriptionText := page.MustElement("p.project-details__wrapper__container__description-area__description").MustText()
	fmt.Println(descriptionText)
	nameEditButton := page.MustElement("div.container.white:nth-of-type(1)").MustText()
	descriptionEditButton := page.MustElement("#app > div > div > div.dashboard__wrap__main-area > div.dashboard__wrap__main-area__content > div.project-details > div > div > div.project-details__wrapper__container__description-area > div").MustText()
	fmt.Println(nameEditButton, descriptionEditButton)

	// Output: Project Details
	// Name
	// Description
	// No description yet. Please enter some information if any.
	// Edit Edit
}
