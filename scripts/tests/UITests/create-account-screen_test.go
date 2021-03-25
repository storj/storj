package UITests

import (
	"fmt"
)

func Example_createAccountScreen () {
	page, browser := login_to_account()
	defer browser.MustClose()
	page.MustElement("div.login-container__register-button").MustClick()

	fmt.Println(page.MustElement("svg.register-container__logo").MustVisible())
	toLogin:= page.MustElement("div.register-container__register-button").MustText()
	fmt.Println(toLogin)
	title:= page.MustElement("h1.register-area__title-container__title").MustText()
	fmt.Println(title)
	fullNameLabel:= page.MustElementX("(//*[@class=\"label-container__label\"])[1]").MustText()
	fmt.Println(fullNameLabel)
	fmt.Println(page.MustElementX("(//*[@class=\"headerless-input\"])[1]").MustVisible())
	fullNamePlaceholder:= page.MustElementX("(//*[@class=\"headerless-input\"])[1]").MustAttribute("placeholder")
	fmt.Println(*fullNamePlaceholder)
	emailLabel:= page.MustElementX("(//*[@class=\"label-container__label\"])[2]").MustText()
	fmt.Println(emailLabel)
	fmt.Println(page.MustElementX("(//*[@class=\"headerless-input\"])[2]").MustVisible())
	emailPlaceholder:= page.MustElementX("(//*[@class=\"headerless-input\"])[2]").MustAttribute("placeholder")
	fmt.Println(*emailPlaceholder)
	passwordLabel:= page.MustElementX("(//*[@class=\"label-container__label\"])[3]").MustText()
	fmt.Println(passwordLabel)
	fmt.Println(page.MustElementX("(//*[@class=\"headerless-input password\"])[1]").MustVisible())
	passwordPlaceholder:= page.MustElementX("(//*[@class=\"headerless-input password\"])[1]").MustAttribute("placeholder")
	fmt.Println(*passwordPlaceholder)
	confirmLabel:= page.MustElementX("(//*[@class=\"label-container__label\"])[4]").MustText()
	fmt.Println(confirmLabel)
	fmt.Println(page.MustElementX("(//*[@class=\"headerless-input password\"])[2]").MustVisible())
	confirmPlaceholder:= page.MustElementX("(//*[@class=\"headerless-input password\"])[2]").MustAttribute("placeholder")
	fmt.Println(*confirmPlaceholder)
	fmt.Println(page.MustElement("span.checkmark").MustVisible())
	termsLabel:= page.MustElement("h2.register-area__submit-container__terms-area__terms-confirmation > label").MustText()
	fmt.Println(termsLabel)
	//Output: true
	// Login
	// Sign Up to Storj
	// Full Name
	// true
	// Enter Full Name
	// Email
	// true
	// Enter Email
	// Password
	// true
	// Enter Password
	// Confirm Password
	// true
	// Confirm Password
	// true
	// I agree to the
}

