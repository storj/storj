package UITests

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
	"time"
)

func Test_membersAdding (t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(4)").MustClick()
		page.MustElement("div.button.container").MustClick()
		fmt.Println(page.MustElement("h2.add-user__info-panel-container__main-label-text").MustText())
		fmt.Println(page.MustElement("p.add-user__form-container__common-label").MustText())
		fmt.Println(*page.MustElement("img").MustAttribute("src"))
		fmt.Println(page.MustElement("div.add-user__close-cross-container").MustVisible())
		fmt.Println(*page.MustElement("input.no-error-input").MustAttribute("placeholder"))
		fmt.Println(page.MustElement("path.delete-input-svg-path").MustVisible())
		fmt.Println(page.MustElement("rect.add-user-row__item__image__rect").MustVisible())
		fmt.Println(page.MustElement("p.add-user-row__item__label").MustText())
		fmt.Println(page.MustElement("div.container.transparent").MustText())
		fmt.Println(page.MustElement("div.container.disabled").MustText())
		fmt.Println(page.MustElement("svg.notification-wrap__image").MustVisible())
		fmt.Println(page.MustElement("p.notification-wrap__text-area__text").MustText())
		fmt.Println(*page.MustElement("p.notification-wrap__text-area__text > a").MustAttribute("href"))

		//Output: Add Team Member
		// Email Address
		// /static/dist/img/addMember.90e0ddbc.jpg
		// true
		// email@example.com
		// true
		// true
		// Add More
		// Cancel
		// Add Team Members
		// true
		// If the team member you want to invite to join the project is still not on this Satellite, please share this link to the signup page and ask them to register here: 127.0.0.1:10002/signup
		// /signup
	}

	func TestMembersAddingFunc(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(4)").MustClick()
		page.MustElement("div.button.container").MustClick()
		listBeforeAdding:= len(page.MustElements("input.no-error-input"))
		page.MustElement("p.add-user-row__item__label").MustClick()
		listAfterAdding:= len(page.MustElements("input.no-error-input"))
		assert.Equal(t,listBeforeAdding,listAfterAdding-1)
		page.MustElement("path.delete-input-svg-path").MustClick()
		listAfterDeleting:= len(page.MustElements("input.no-error-input"))
		assert.Equal(t,listAfterAdding, listAfterDeleting+1)
		page.MustElement("input.no-error-input").MustInput("asd@dfg.com")
		time.Sleep(2*time.Second)
		page.MustElement("div.add-user__form-container__button-container > div.container:nth-child(2)").MustClick()
		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
		assert.Equal(t,"Error during adding project members. validation error: There is no account on this Satellite for the user(s) you have entered. Please add team members with active accounts",notification)
	}

