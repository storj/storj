package UITests

import "fmt"

func Example_APIKeysScreen () {
	page, browser := login_to_account()
	defer browser.MustClose()
	page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()

	// screen without keys created
	apiKeysHeader:= page.MustElement("h1.no-api-keys-area__title").MustText()
	fmt.Println(apiKeysHeader)
	apiKeysText:= page.MustElement("p.no-api-keys-area__sub-title").MustText()
	fmt.Println(apiKeysText)
	createKeyButton:= page.MustElement("div.no-api-keys-area__button.container").MustText()
	fmt.Println(createKeyButton)
	uploadSteps:= page.MustElement("div.no-api-keys-area__steps-area__numbers").MustVisible()
	fmt.Println(uploadSteps)
	firstStepText:= page.MustElement("h2.no-api-keys-area__steps-area__items__create-api-key__title").MustText()
	fmt.Println(firstStepText)
	firstStepImage:= page.MustHas("img.no-api-keys-area-image")
	fmt.Println(firstStepImage)
	firstStepImagePath:= page.MustElement("img.no-api-keys-area-image:nth-of-type(1)").MustAttribute("src")
	fmt.Println(*firstStepImagePath)
	secondStepText:= page.MustElement("h2.no-api-keys-area__steps-area__items__setup-uplink__title").MustText()
	fmt.Println(secondStepText)
	secndStepImage:= page.MustHasX("(//*[@class=\"no-api-keys-area-image\"])[2]")
	fmt.Println(secndStepImage)
	secondStepImagePath:= page.MustElementX("(//*[@class=\"no-api-keys-area-image\"])[2]").MustAttribute("src")
	fmt.Println(*secondStepImagePath)
	thirdStepText:= page.MustElement("h2.no-api-keys-area__steps-area__items__store-data__title").MustText()
	fmt.Println(thirdStepText)
	thirdStepImage:= page.MustHasX("(//*[@class=\"no-api-keys-area-image\"])[3]")
	fmt.Println(thirdStepImage)
	thirdStepImagePath:= page.MustElementX("(//*[@class=\"no-api-keys-area-image\"])[3]").MustAttribute("src")
	fmt.Println(*thirdStepImagePath)


	// Output: Create Your First API Key
	// API keys give access to the project to create buckets, upload objects
	// Create API Key
	// true
	// Create & Save API Key
	// true
	// /static/dist/img/apiKey.981d0fef.jpg
	// Setup Uplink CLI
	// true
	// /static/dist/img/uplink.30403d68.jpg
	// Store Data
	// true
	// /static/dist/img/store.eb048f38.jpg
}

