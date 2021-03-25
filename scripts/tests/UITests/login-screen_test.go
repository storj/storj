package UITests

import (
	"fmt"
)

func Example_LoginScreen() {

	page, browser := login_to_account()
	defer browser.MustClose()

	header:= page.MustElement("h1.login-area__title-container__title").MustText()
	fmt.Println(header)
	fmt.Println(page.MustElement("svg.login-container__logo").MustVisible())
	forgotText:= page.MustElement("h3.login-area__navigation-area__nav-link__link").MustText()
	fmt.Println(forgotText)
	forgotLink:= page.MustElement("a.login-area__navigation-area__nav-link").MustAttribute("href")
	fmt.Println(*forgotLink)
	createAccButton:= page.MustElement("div.login-container__register-button").MustText()
	fmt.Println(createAccButton)
	loginButton:= page.MustElement("div.login-area__submit-area__login-button").MustText()
	fmt.Println(loginButton)
	siganture:= page.MustElement("p.login-area__info-area__signature").MustText()
	fmt.Println(siganture)
	termsText:= page.MustElement("a.login-area__info-area__terms").MustText()
	fmt.Println(termsText)
	termsLink:= page.MustElement("a.login-area__info-area__terms").MustAttribute("href")
	fmt.Println(*termsLink)
	supportText:= page.MustElement("a.login-area__info-area__help").MustText()
	fmt.Println(supportText)
	supportLink:= page.MustElement("a.login-area__info-area__help").MustAttribute("href")
	fmt.Println(*supportLink)

	// Output: Login to Storj
	// true
	// Forgot password?
	// /forgot-password
	// Create Account
	// Log In
	// Storj Labs Inc 2020.
	// Terms & Conditions
	// https://tardigrade.io/terms-of-use/
	// Support
	// mailto:support@storj.io


}

