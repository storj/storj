package UITests

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"time"
)

var login string = "test1@g.com"
	var password = "123qwe"
	var startPage = "http://127.0.0.1:10002/login"
//	var startPage = "https://satellite.qa.storj.io/login"
	var screenWidth int= 1350
	var screenHeigth int = 600

	// this is for making screenshots - debugging in docker
//func Test_page_screenshot(t *testing.T) {
//	page := rod.New().MustConnect().MustPage(startPage).MustWaitLoad()
//
//	// simple version
//	page.MustScreenshotFullPage("./screenshots/my1.png")
//}

func login_to_account() (*rod.Page, *rod.Browser) {
	l := launcher.New().
		Headless(false).
		Devtools(false)
//	defer l.Cleanup()
	url := l.MustLaunch()

	browser := rod.New().
		Timeout(time.Minute).
		ControlURL(url).
		Trace(true).
		SlowMotion(300 * time.Millisecond).
		MustConnect()


	//// Even you forget to close, rod will close it after main process ends.
	//  defer browser.MustClose()

	// Timeout will be passed to all chained function calls.
	// The code will panic out if any chained call is used after the timeout.
	page := browser.Timeout(25*time.Second).MustPage(startPage)

	// Make sure viewport is always consistent.
	page.MustSetViewport(screenWidth, screenHeigth, 1, false)

	// We use css selector to get the search input element and input "git"
	page.MustElement(".headerless-input").MustInput(login)
	page.MustElement("[type=password]").MustInput(password)

	page.Keyboard.MustPress(input.Enter)

	return page, browser
}




//
//	func Example_LoginScreenTar() {
//
//		l := launcher.New().
//			Headless(true).
//			Devtools(false)
//		defer l.Cleanup()
//		url := l.MustLaunch()
//
//		browser := rod.New().
//			Timeout(time.Minute).
//			ControlURL(url).
//			Trace(true).
//			SlowMotion(300 * time.Millisecond).
//			MustConnect()
//
//		// Even you forget to close, rod will close it after main process ends.
//		defer browser.MustClose()
//
//		// Timeout will be passed to all chained function calls.
//		// The code will panic out if any chained call is used after the timeout.
//		page := browser.Timeout(15 * time.Second).MustPage(startPage)
//
//		// Make sure viewport is always consistent.
//		page.MustSetViewport(screenWidth, screenHeigth, 1, false)
//		fmt.Println(page.MustElement("svg.login-container__logo").MustVisible())
//		header:= page.MustElement("h1.login-area__title-container__title").MustText()
//		fmt.Println(header)
//
//		// Output: true
//		// Login to Tardigrade
//	}

//	func Example_createAccountScreen2 () {
//		l := launcher.New().
//			Headless(false).
//			Devtools(false)
//		defer l.Cleanup()
//		url := l.MustLaunch()
//
//		browser := rod.New().
//			Timeout(time.Minute).
//			ControlURL(url).
//			Trace(true).
//			SlowMotion(300 * time.Millisecond).
//			MustConnect()
//
//		// Even you forget to close, rod will close it after main process ends.
//		defer browser.MustClose()
//
//		// Timeout will be passed to all chained function calls.
//		// The code will panic out if any chained call is used after the timeout.
//		page := browser.Timeout(15 * time.Second).MustPage(startPage)
//		page.MustElement("div.login-container__register-button").MustClick()
//		termsLinkText:= page.MustElement("a.register-area__submit-container__terms-area__link").MustText()
//		fmt.Println(termsLinkText)
//		termsLink:= page.MustElement("a.register-area__submit-container__terms-area__link").MustAttribute("href")
//		fmt.Println(*termsLink)
//		createButton:= page.MustElement("div#createAccountButton").MustText()
//		fmt.Println(createButton)
//
//		// Output: Terms & Conditions
//		// https://tardigrade.io/terms-of-use/
//		// Create Account
//	}
//
//	func Example_forgotPassScreen () {
//		l := launcher.New().
//			Headless(false).
//			Devtools(false)
//		defer l.Cleanup()
//		url := l.MustLaunch()
//
//		browser := rod.New().
//			Timeout(time.Minute).
//			ControlURL(url).
//			Trace(true).
//			SlowMotion(300 * time.Millisecond).
//			MustConnect()
//
//		// Even you forget to close, rod will close it after main process ends.
//		defer browser.MustClose()
//
//		// Timeout will be passed to all chained function calls.
//		// The code will panic out if any chained call is used after the timeout.
//		page := browser.Timeout(15 * time.Second).MustPage(startPage)
//		page.MustElement("a.login-area__navigation-area__nav-link").MustClick()
//
//		fmt.Println(page.MustElement("svg.forgot-password-container__logo").MustVisible())
//		backToLoginText:= page.MustElement("div.forgot-password-container__login-button").MustText()
//		fmt.Println(backToLoginText)
//		header:= page.MustElement("h1.forgot-password-area__title-container__title").MustText()
//		fmt.Println(header)
//		text:= page.MustElement("p.forgot-password-area__info-text").MustText()
//		fmt.Println(text)
//		//input visibility
//		fmt.Println(page.MustElement("input.headerless-input").MustVisible())
//		inputPlaceholder:= page.MustElement("input.headerless-input").MustAttribute("placeholder")
//		fmt.Println(*inputPlaceholder)
//		resetButton:= page.MustElement("div.forgot-password-area__submit-container").MustText()
//		fmt.Println(resetButton)
//
//		// Output: true
//		// Back to Login
//		// Forgot Password
//		// Enter your email address below and we'll get you back on track.
//		// true
//		// Enter Your Email
//		// Reset Password
//	}
//
//
//	func Example_APIKeysCreationFlowElements() {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
//
//		page.MustElement("div.button.container").MustClick()
//		time.Sleep(1* time.Second)
//		// checking elements
//		fmt.Println(page.MustElement("h2.new-api-key__title").MustText())
//		fmt.Println(page.MustElement("div.new-api-key__close-cross-container").MustVisible())
//		fmt.Println(*page.MustElement("input.headerless-input").MustAttribute("placeholder"))
//		fmt.Println(page.MustElement("span.label").MustText())
//		// creation flow
//		page.MustElement("input.headerless-input").MustInput("jhghgf")
//		page.MustElement("span.label").MustClick()
//
//		fmt.Println(page.MustElement("h2.save-api-popup__title").MustText())
//		fmt.Println(page.MustElement("div.save-api-popup__copy-area__key-area").MustVisible())
//		fmt.Println(page.MustElement("p.save-api-popup__copy-area__copy-button").MustText())
//		fmt.Println(page.MustElement("span.save-api-popup__next-step-area__label").MustText())
//		fmt.Println(*page.MustElement("a.save-api-popup__next-step-area__link").MustAttribute("href"))
//		fmt.Println(page.MustElement("a.save-api-popup__next-step-area__link").MustText())
//		fmt.Println(page.MustElement("div.container").MustText())
//		page.MustElement("p.save-api-popup__copy-area__copy-button").MustClick()
//		fmt.Println(page.MustElement("p.notification-wrap__text-area__message").MustText())
//		page.MustElement("div.container").MustClick()
//
//
//
//
//
//		//Output: Name Your API Key
//		// true
//		// Enter API Key Name
//		// Next >
//		// Save Your Secret API Key! It Will Appear Only Once.
//		// true
//		// Copy
//		// Next Step:
//		// https://documentation.tardigrade.io/getting-started/uploading-your-first-object/set-up-uplink-cli
//		// Set Up Uplink CLI
//		// Done
//		// Successfully created new api key
//
//
//	}
//
//	func TestAPIKeysCreation(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
//
//		listBeforeAdding := len(page.MustElements("div.apikey-item-container.item-component__item"))
//		page.MustElement("div.button.container").MustClick()
//		time.Sleep(1 * time.Second)
//		// creation flow
//		page.MustElement("input.headerless-input").MustInput("khg")
//		page.MustElement("span.label").MustClick()
//		time.Sleep(1 * time.Second)
//		page.MustElement("div.container").MustClick()
//		listAfterAdding := len(page.MustElements("div.apikey-item-container.item-component__item"))
//		assert.Equal(t, listAfterAdding, listBeforeAdding + 1)
//
//	}
//
//	func Example_APIKeyDeletionElements()  {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
//		page.MustElement("div.apikey-item-container.item-component__item").MustClick()
//
//		fmt.Println(page.MustElement("div.button.deletion.container").MustText())
//		fmt.Println(page.MustElement("div.button.container.transparent").MustText())
//		fmt.Println(page.MustElement("span.header-selected-api-keys__info-text").MustText())
//		page.MustElement("div.button.deletion.container").MustClick()
//		fmt.Println(page.MustElement("span.header-selected-api-keys__confirmation-label").MustText())
//		page.MustElement("div.button.deletion.container").MustClick()
//		fmt.Println(page.MustElement("p.notification-wrap__text-area__message").MustText())
//
//
//		//Output: Delete
//		// Cancel
//		// 1 API Keys selected
//		// Are you sure you want to delete 1 api key ?
//		// API keys deleted successfully
//	}
//
//	func TestAPIKeysDeletion(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()
//		listBeforeDeletion:= len(page.MustElements("div.apikey-item-container.item-component__item"))
//		page.MustElement("div.apikey-item-container.item-component__item").MustClick()
//		page.MustElement("div.button.deletion.container").MustClick()
//		page.MustElement("div.button.deletion.container").MustClick()
//		time.Sleep(2*time.Second)
//		listAfterDeletion:= len(page.MustElements("div.apikey-item-container.item-component__item"))
//		assert.Equal(t,listBeforeDeletion,listAfterDeletion+1)
//	}
//
//	func Example_membersAdding () {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(3)").MustClick()
//		page.MustElement("div.button.container").MustClick()
//		fmt.Println(page.MustElement("h2.add-user__info-panel-container__main-label-text").MustText())
//		fmt.Println(page.MustElement("p.add-user__form-container__common-label").MustText())
//		fmt.Println(*page.MustElement("img").MustAttribute("src"))
//		fmt.Println(page.MustElement("div.add-user__close-cross-container").MustVisible())
//		fmt.Println(*page.MustElement("input.no-error-input").MustAttribute("placeholder"))
//		fmt.Println(page.MustElement("path.delete-input-svg-path").MustVisible())
//		fmt.Println(page.MustElement("rect.add-user-row__item__image__rect").MustVisible())
//		fmt.Println(page.MustElement("p.add-user-row__item__label").MustText())
//		fmt.Println(page.MustElement("div.container.transparent").MustText())
//		fmt.Println(page.MustElement("div.container.disabled").MustText())
//		fmt.Println(page.MustElement("svg.notification-wrap__image").MustVisible())
//		fmt.Println(page.MustElement("p.notification-wrap__text-area__text").MustText())
//		fmt.Println(*page.MustElement("p.notification-wrap__text-area__text > a").MustAttribute("href"))
//
//		//Output: Add Team Member
//		// Email Address
//		// /static/dist/img/addMember.90e0ddbc.jpg
//		// true
//		// email@example.com
//		// true
//		// true
//		// Add More
//		// Cancel
//		// Add Team Members
//		// true
//		// If the team member you want to invite to join the project is still not on this Satellite, please share this link to the signup page and ask them to register here: 127.0.0.1:10002/signup
//		// /signup
//	}
//
//	func TestMembersAddingFunc(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("a.navigation-area__item-container:nth-of-type(3)").MustClick()
//		page.MustElement("div.button.container").MustClick()
//		listBeforeAdding:= len(page.MustElements("input.no-error-input"))
//		page.MustElement("p.add-user-row__item__label").MustClick()
//		listAfterAdding:= len(page.MustElements("input.no-error-input"))
//		assert.Equal(t,listBeforeAdding,listAfterAdding-1)
//		page.MustElement("path.delete-input-svg-path").MustClick()
//		listAfterDeleting:= len(page.MustElements("input.no-error-input"))
//		assert.Equal(t,listAfterAdding, listAfterDeleting+1)
//		page.MustElement("input.no-error-input").MustInput("asd@dfg.com")
//		time.Sleep(2*time.Second)
//		page.MustElement("div.add-user__form-container__button-container > div.container:nth-child(2)").MustClick()
//		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
//		assert.Equal(t,"Error during adding project members. validation error: There is no account on this Satellite for the user(s) you have entered. Please add team members with active accounts",notification)
//	}
//
//	func Example_CreateProjectElements(){
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.project-selection__toggle-container").MustClick()
//		page.MustElement("div.project-dropdown__create-project__button-area").MustClick()
//
//		fmt.Println(page.MustElement("img").MustVisible())
//		fmt.Println(*page.MustElement("img").MustAttribute("src"))
//		fmt.Println(page.MustElement("h2.create-project-area__title").MustText())
//		fmt.Println(page.MustElement("h3.label-container__main__label").MustText())
//		fmt.Println(page.MustElement("h3.label-container__main__label.add-label").MustText())
//		fmt.Println(page.MustElement("h3.label-container__limit").MustText())
//		fmt.Println(*page.MustElement("input.headered-input").MustAttribute("placeholder"))
//		fmt.Println(page.MustElementX("(//*[@class=\"label-container__main__label\"])[2]").MustText())
//		fmt.Println(page.MustElementX("(//*[@class=\"label-container__main__label add-label\"])[2]").MustText())
//		fmt.Println(page.MustElementX("(//*[@class=\"label-container__limit\"])[2]").MustText())
//		fmt.Println(*page.MustElement("textarea#Description").MustAttribute("placeholder"))
//		fmt.Println(page.MustElement("div.container.transparent").MustText())
//		fmt.Println(page.MustElement("div.container.disabled").MustText())
//
//
//		//Output: true
//		// /static/dist/img/createProject.057ac8a4.png
//		// Create a Project
//		// Project Name
//		// Up To 20 Characters
//		// 0/20
//		// Enter Project Name
//		// Description
//		// Optional
//		// 0/100
//		// Enter Project Description
//		// Cancel
//		// Create Project +
//	}
//
//	func TestCreateProjectFlow(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.project-selection__toggle-container").MustClick()
//		listBeforeProjectadding:= len(page.MustElements("div.project-dropdown__wrap__choice"))
//		page.MustElement("div.project-dropdown__create-project__button-area").MustClick()
//
//		// validation checking
//		page.MustElement("input.headered-input").MustInput("   ")
//		page.MustElement("div.container:nth-of-type(2)").MustClick()
//		errorValidation:= page.MustElement("h3.label-container__main__error").MustText()
//		assert.Equal(t,"Project name can't be empty!",errorValidation)
//		time.Sleep(1*time.Second)
//		// adding valid project notification
//		page.MustElement("input.headered-input").MustInput("1234")
//		page.MustElement("div.container:nth-of-type(2)").MustClick()
//		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
//		assert.Equal(t, "Project created successfully!",notification)
//
//		// checking project list
//		page.MustElement("div.project-selection__toggle-container").MustClick()
//		listAfterProjectadding:= len(page.MustElements("div.project-dropdown__wrap__choice"))
//		assert.Equal(t, listBeforeProjectadding,listAfterProjectadding-1)
//	}
//
//	func TestResourcesDroplist(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.resources-selection__toggle-container").MustClick()
//		listLength:= len(page.MustElements("a.resources-dropdown__item-container"))
//		assert.Equal(t,3,listLength)
//		docsText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(1)").MustText()
//		assert.Equal(t,"Docs",docsText)
//		docsLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(1)").MustAttribute("href")
//		assert.Equal(t,"https://documentation.storj.io",docsLink)
//		communityText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(2)").MustText()
//		assert.Equal(t,"Community",communityText)
//		communityLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(2)").MustAttribute("href")
//		assert.Equal(t,"https://storj.io/community/",communityLink)
//		supportText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(3)").MustText()
//		assert.Equal(t,"Support",supportText)
//		supportLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(3)").MustAttribute("href")
//		assert.Equal(t,"mailto:support@storj.io",supportLink)
//	}
//
//	func TestAccountSettingsScreenElements(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.settings-selection.settings-selection").MustClick()
//		// check list length
//		listLength:= len(page.MustElements("div.settings-dropdown__choice"))
//		assert.Equal(t, 2,listLength)
//
//		accSettings:= page.MustElement("div.settings-dropdown__choice:nth-of-type(1)")
//		billing:= page.MustElement("div.settings-dropdown__choice:nth-of-type(2)")
//		assert.Equal(t, "Account Settings", accSettings.MustText())
//		assert.Equal(t,"Billing", billing.MustText())
//
//		accSettings.MustClick()
//
//		header:= page.MustElement("h1.profile-container__title").MustText()
//		assert.Equal(t, "Account Settings", header)
//
//		editHeader:= page.MustElement("h2.profile-bold-text:nth-of-type(1)").MustText()
//		assert.Equal(t,"Edit Profile",editHeader)
//		editImage:= page.MustElement("div.profile-container__edit-profile__avatar").MustVisible()
//		assert.T(t, editImage)
//		editNotification:= page.MustElement("h3.profile-regular-text:nth-of-type(1)").MustText()
//		assert.Equal(t,"This information will be visible to all users",editNotification)
//		editProfileButton:=page.MustElement("svg.edit-svg:nth-of-type(1)").MustVisible()
//		assert.T(t,editProfileButton)
//
//		editpassHeader:= page.MustElementX("(//*[@class=\"profile-bold-text\"])[2]").MustText()
//		assert.Equal(t,"Change Password",editpassHeader)
//		editpassImage:= page.MustElement("svg.profile-container__secondary-container__img").MustVisible()
//		assert.T(t, editpassImage)
//		editpassNotification:= page.MustElementX("(//*[@class=\"profile-regular-text\"])[2]").MustText()
//		assert.Equal(t,"6 or more characters",editpassNotification)
//		editpassButton:=page.MustElementX("(//*[@class=\"edit-svg\"])[2]").MustVisible()
//		assert.T(t,editpassButton)
//
//		emailImage:= page.MustElementX("(//*[@class=\"profile-container__secondary-container__img\"])[2]").MustVisible()
//		assert.T(t, emailImage)
//		emailText:= page.MustElementX("(//*[@class=\"profile-bold-text email\"])").MustText()
//		assert.Equal(t,login,emailText)
//
//	}
//
//	func TestAccountSettingsEditAcc(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.settings-selection.settings-selection").MustClick()
//
//		page.MustElement("div.settings-dropdown__choice:nth-of-type(1)").MustClick()
//		time.Sleep(3*time.Second)
//		page.MustElement("svg.edit-svg").MustClick()
//
//		header:=page.MustElement("h2.edit-profile-popup__form-container__main-label-text").MustText()
//		assert.Equal(t,"Edit Profile", header)
//		headerImage:=page.MustElement("div.edit-profile-popup__form-container__avatar").MustVisible()
//		assert.T(t,headerImage)
//		closeButton:= page.MustElement("div.edit-profile-popup__close-cross-container").MustVisible()
//		assert.T(t,closeButton)
//		fullnametext:= page.MustElementX("(//*[@class=\"label-container__main__label\"])[1]").MustText()
//		assert.Equal(t,"Full Name",fullnametext)
//		nameInput:= page.MustElementX("//*[@id=\"Full Name\"]").MustAttribute("placeholder")
//		assert.Equal(t,"Enter Full Name",*nameInput)
//		nicknametext:= page.MustElementX("(//*[@class=\"label-container__main__label\"])[2]").MustText()
//		assert.Equal(t,"Nickname",nicknametext)
//		nicknameInput:= page.MustElementX("//*[@id=\"Nickname\"]").MustAttribute("placeholder")
//		assert.Equal(t,"Enter Nickname",*nicknameInput)
//		cancelButton:= page.MustElement("div.container.transparent").MustText()
//		assert.Equal(t,"Cancel",cancelButton)
//		updateButton:= page.MustElementX("(//*[@class=\"container\"])").MustText()
//		assert.Equal(t,"Update",updateButton)
//	}
//	func TestAccountSettingsEditFunc(t *testing.T) {
//		page, browser := login_to_account()
//		defer browser.MustClose()
//		page.MustElement("div.settings-selection.settings-selection").MustClick()
////		avaFirst:= page.MustElement("h1.account-button__container__avatar__letter").MustText()
//
//		page.MustElement("div.settings-dropdown__choice:nth-of-type(1)").MustClick()
//		time.Sleep(3 * time.Second)
//		page.MustElement("svg.edit-svg").MustClick()
//		page.MustElementX("//*[@id=\"Full Name\"]").MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustInput(" ")
//		page.MustElementX("(//*[@class=\"container\"])").MustClick()
//		errorMessage:= page.MustElement("h3.label-container__main__error").MustText()
//		assert.Equal(t,"Full name expected",errorMessage)
//
//		page.MustElementX("//*[@id=\"Full Name\"]").MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustPress(input.Backspace).MustInput("zzz")
//		page.MustElementX("(//*[@class=\"container\"])").MustClick()
//		notification:= page.MustElement("p.notification-wrap__text-area__message").MustText()
//		assert.Equal(t,"Account info successfully updated!",notification)
//		avaChanged:= page.MustElement("h1.account-button__container__avatar__letter").MustText()
//		assert.Equal(t,"Z",avaChanged)
//
//		page.MustElement("svg.edit-svg").MustClick()
//		page.MustElementX("//*[@id=\"Nickname\"]").MustInput("яяя")
//		page.MustElementX("(//*[@class=\"container\"])").MustClick()
//		time.Sleep(2*time.Second)
//		avaChanged2:= page.MustElement("h1.account-button__container__avatar__letter").MustText()
//		assert.Equal(t,"Я",avaChanged2)
//
//	}
//
//
//
//
//
//
//
//
//
