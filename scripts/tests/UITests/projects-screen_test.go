package UITests

import (
	"fmt"
	assert2 "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"strings"
	"testing"
	"time"
)

func Test_CreateProjectElements(t *testing.T){
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.project-selection__toggle-container").MustClick()
		page.MustElement("div.project-dropdown__create-project__button-area").MustClick()
		page.MustElementX("(//*[@class=\"container\"])[1]").MustClick()

		fmt.Println(page.MustElement("img").MustVisible())
		fmt.Println(*page.MustElement("img").MustAttribute("src"))
		fmt.Println(page.MustElement("h2.create-project-area__container__title").MustText())
		fmt.Println(page.MustElement("h3.label-container__main__label").MustText())
		fmt.Println(page.MustElement("h3.label-container__main__label.add-label").MustText())
		fmt.Println(page.MustElement("h3.label-container__limit").MustText())
		fmt.Println(*page.MustElement("input.headered-input").MustAttribute("placeholder"))
		fmt.Println(page.MustElementX("(//*[@class=\"label-container__main__label\"])[2]").MustText())
		fmt.Println(page.MustElementX("(//*[@class=\"label-container__main__label add-label\"])[2]").MustText())
		fmt.Println(page.MustElementX("(//*[@class=\"label-container__limit\"])[2]").MustText())
		fmt.Println(*page.MustElement("textarea#Description").MustAttribute("placeholder"))
		fmt.Println(page.MustElement("div.container.transparent").MustText())
		fmt.Println(page.MustElement("div.container.disabled").MustText())


		//Output: true
		// /static/dist/img/createProject.057ac8a4.png
		// Create a Project
		// Project Name
		// Up To 20 Characters
		// 0/20
		// Enter Project Name
		// Description
		// Optional
		// 0/100
		// Enter Project Description
		// Cancel
		// Create Project +
	}

	func TestCreateProjectFlow(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.project-selection__toggle-container").MustClick()
		listBeforeProjectadding:= len(page.MustElements("div.project-dropdown__wrap__choice"))
		page.MustElement("div.project-dropdown__create-project__button-area").MustClick()
		page.MustElementX("(//*[@class=\"container\"])[1]").MustClick()

		// validation checking
		page.MustElement("input.headered-input").MustInput("   ")
		page.MustElement("div.container:nth-of-type(2)").MustClick()
		errorValidation:= page.MustElement("h3.label-container__main__error").MustText()
		assert.Equal(t,"Project name can't be empty!",errorValidation)
		time.Sleep(1*time.Second)
		// adding valid project notification
		page.MustElement("input.headered-input").MustInput("1234")
		page.MustElement("div.container:nth-of-type(2)").MustClick()
		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
		assert.Equal(t, "Project created successfully!",notification)

		// checking project list
		page.MustElement("div.project-selection__toggle-container").MustClick()
		listAfterProjectadding:= len(page.MustElements("div.project-dropdown__wrap__choice"))
		assert.Equal(t, listBeforeProjectadding,listAfterProjectadding-1)
	}

func Test_projectsScreen (t *testing.T) {
	page, browser := login_to_account()
	defer browser.MustClose()

	// go to projects screen
	page.MustElement("div.project-selection__toggle-container").MustClick()
	page.MustElement("div.project-dropdown__create-project").MustClick()
	// checking notification
	notificationBegin := page.MustElement("b.info-bar__info-area__first-value").MustText()
	assert.Assert(t, strings.Contains(notificationBegin, "You have used"))
	notificationMiddle := page.MustElement("span.info-bar__info-area__first-description").MustText()
	assert2.Equal(t, "of your", notificationMiddle)
	notificationEnd := page.MustElement("span.info-bar__info-area__second-description").MustText()
	assert.Equal(t, "available projects.", notificationEnd)
	notificationLink := page.MustElement("a.info-bar__link.blue").MustAttribute("href")
	assert.DeepEqual(t, *(notificationLink), "https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212")
	projectsTitle:= page.MustElement("h2.projects-list__title-area__title").MustText()
	assert2.Equal(t,"Projects", projectsTitle)
	createButton:= page.MustElementX("(//*[@class=\"container\"])[1]").MustText()
	assert2.Equal(t,"Create Project +",createButton)
	nameColumnTitle:= page.MustElement("p.sort-header-container__name-item__title").MustText()
	assert2.Equal(t,"NAME",nameColumnTitle)
	usersColumnTitle:= page.MustElement("p.sort-header-container__users-item__title").MustText()
	assert2.Equal(t,"# USERS",usersColumnTitle)
	dateColumnTitle:= page.MustElement("p.sort-header-container__date-item__title").MustText()
	assert2.Equal(t,"DATE ADDED",dateColumnTitle)
	createdProjectName:= page.MustElement("p.container__item.name").MustText()
	assert2.Equal(t,"1234",createdProjectName)
	createdProjectUsers:= page.MustElement("p.container__item.member-count").MustText()
	assert2.Equal(t,"1",createdProjectUsers)
	createdProjectDate:= page.MustElement("p.container__item.date").MustText()
	assert.Assert(t,strings.Contains(createdProjectDate,", 2021"))
}


