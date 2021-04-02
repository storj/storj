package UITests

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
	"time"
)

func Example_APIKeysCreationFlowElements() {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()

		page.MustElement("div.button.container").MustClick()
		time.Sleep(1* time.Second)
		// checking elements
		fmt.Println(page.MustElement("h2.new-api-key__title").MustText())
		fmt.Println(page.MustElement("div.new-api-key__close-cross-container").MustVisible())
		fmt.Println(*page.MustElement("input.headerless-input").MustAttribute("placeholder"))
		fmt.Println(page.MustElement("span.label").MustText())
		// creation flow
		page.MustElement("input.headerless-input").MustInput("jhghgf")
		page.MustElement("span.label").MustClick()

		fmt.Println(page.MustElement("h2.save-api-popup__title").MustText())
		fmt.Println(page.MustElement("div.save-api-popup__copy-area__key-area").MustVisible())
		fmt.Println(page.MustElement("p.save-api-popup__copy-area__copy-button").MustText())
		fmt.Println(page.MustElement("span.save-api-popup__next-step-area__label").MustText())
		fmt.Println(*page.MustElement("a.save-api-popup__next-step-area__link").MustAttribute("href"))
		fmt.Println(page.MustElement("a.save-api-popup__next-step-area__link").MustText())
		fmt.Println(page.MustElement("div.container").MustText())
		page.MustElement("p.save-api-popup__copy-area__copy-button").MustClick()
		fmt.Println(page.MustElement("p.notification-wrap__text-area__message").MustText())
		page.MustElement("div.container").MustClick()





		//Output: Name Your API Key
		// true
		// Enter API Key Name
		// Next >
		// Save Your Secret API Key! It Will Appear Only Once.
		// true
		// Copy
		// Next Step:
		// https://documentation.tardigrade.io/getting-started/uploading-your-first-object/set-up-uplink-cli
		// Set Up Uplink CLI
		// Done
		// Successfully created new api key


	}


	func TestAPIKeysCreation(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()

		listBeforeAdding := len(page.MustElements("div.apikey-item-container.item-component__item"))
		page.MustElement("div.button.container").MustClick()
		time.Sleep(1 * time.Second)
		// creation flow
		page.MustElement("input.headerless-input").MustInput("khg")
		page.MustElement("span.label").MustClick()
		time.Sleep(1 * time.Second)
		page.MustElement("div.container").MustClick()
		listAfterAdding := len(page.MustElements("div.apikey-item-container.item-component__item"))
		assert.Equal(t, listAfterAdding, listBeforeAdding + 1)

	}

	func Example_APIKeyDeletionElements()  {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
		page.MustElement("div.apikey-item-container.item-component__item").MustClick()

		fmt.Println(page.MustElement("div.button.deletion.container").MustText())
		fmt.Println(page.MustElement("div.button.container.transparent").MustText())
		fmt.Println(page.MustElement("span.header-selected-api-keys__info-text").MustText())
		page.MustElement("div.button.deletion.container").MustClick()
		fmt.Println(page.MustElement("span.header-selected-api-keys__confirmation-label").MustText())
		page.MustElement("div.button.deletion.container").MustClick()
		fmt.Println(page.MustElement("p.notification-wrap__text-area__message").MustText())


		//Output: Delete
		// Cancel
		// 1 API Keys selected
		// Are you sure you want to delete 1 api key ?
		// API keys deleted successfully
	}

	func TestAPIKeysDeletion(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
		listBeforeDeletion:= len(page.MustElements("div.apikey-item-container.item-component__item"))
		page.MustElement("div.apikey-item-container.item-component__item").MustClick()
		page.MustElement("div.button.deletion.container").MustClick()
		page.MustElement("div.button.deletion.container").MustClick()
		time.Sleep(2*time.Second)
		listAfterDeletion:= len(page.MustElements("div.apikey-item-container.item-component__item"))
		assert.Equal(t,listBeforeDeletion,listAfterDeletion+1)
	}
