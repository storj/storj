package UITests

import (
	"fmt"
)

func Example_forgotPassScreen () {
		page, browser := setup_browser()
		defer browser.MustClose()
		page.MustElement("a.login-area__navigation-area__nav-link").MustClick()

		fmt.Println(page.MustElement("svg.forgot-password-container__logo").MustVisible())
		backToLoginText:= page.MustElement("div.forgot-password-container__login-button").MustText()
		fmt.Println(backToLoginText)
		header:= page.MustElement("h1.forgot-password-area__title-container__title").MustText()
		fmt.Println(header)
		text:= page.MustElement("p.forgot-password-area__info-text").MustText()
		fmt.Println(text)
		//input visibility
		fmt.Println(page.MustElement("input.headerless-input").MustVisible())
		inputPlaceholder:= page.MustElement("input.headerless-input").MustAttribute("placeholder")
		fmt.Println(*inputPlaceholder)
		resetButton:= page.MustElement("div.forgot-password-area__submit-container").MustText()
		fmt.Println(resetButton)

		// Output: true
		// Back to Login
		// Forgot Password
		// Enter your email address below and we'll get you back on track.
		// true
		// Enter Your Email
		// Reset Password
	}


