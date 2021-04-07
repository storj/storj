package UITests

import (
	"fmt"
	"testing"
)


func Test_SideMenuLinksChecking(t *testing.T)  {
	page, browser := login_to_account()
	defer browser.MustClose()
	firstLink := *(page.MustElement("a.navigation-area__item-container:nth-of-type(1)",).MustAttribute("href"))
	fmt.Println(firstLink)
	// Output: /project-dashboard
	secondLink := *(page.MustElement("a.navigation-area__item-container:nth-of-type(2)",).MustAttribute("href"))
	fmt.Println(secondLink)
	// Output: /api-keys
	thirdLink := *(page.MustElement("a.navigation-area__item-container:nth-of-type(3)",).MustAttribute("href"))
	fmt.Println(thirdLink)
	// Output: /project-dashboard
	// /access-grants
	// /project-members

}
//func login_to_account() (*rod.Page, *rod.Browser) {
//	l := launcher.New().
//		Headless(false).
//		Devtools(false)
////	defer l.Cleanup()
//	url := l.MustLaunch()
//
//	browser := rod.New().
//		Timeout(time.Minute).
//		ControlURL(url).
//		Trace(true).
//		SlowMotion(300 * time.Millisecond).
//		MustConnect()
//
//
//	//// Even you forget to close, rod will close it after main process ends.
//	//  defer browser.MustClose()
//
//	// Timeout will be passed to all chained function calls.
//	// The code will panic out if any chained call is used after the timeout.
//	page := browser.Timeout(25*time.Second).MustPage(startPage)
//
//	// Make sure viewport is always consistent.
//	page.MustSetViewport(screenWidth, screenHeigth, 1, false)
//
//	// We use css selector to get the search input element and input "git"
//	page.MustElement(".headerless-input").MustInput(login)
//	page.MustElement("[type=password]").MustInput(password)
//
//	page.Keyboard.MustPress(input.Enter)
//
//	return page, browser
//}