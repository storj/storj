package UITests

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
	"time"
)

func Example_CreateProjectElements(){
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.project-selection__toggle-container").MustClick()
		page.MustElement("div.project-dropdown__create-project__button-area").MustClick()

		fmt.Println(page.MustElement("img").MustVisible())
		fmt.Println(*page.MustElement("img").MustAttribute("src"))
		fmt.Println(page.MustElement("h2.create-project-area__title").MustText())
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

