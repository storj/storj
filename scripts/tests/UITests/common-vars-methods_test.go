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
	var screenWidth int= 1300
	var screenHeigth int = 900

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
		SlowMotion(100 * time.Millisecond).
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

func setup_browser() (*rod.Page, *rod.Browser) {
	l := launcher.New().
		Headless(false).
		Devtools(false)
	//	defer l.Cleanup()
	url := l.MustLaunch()

	browser := rod.New().
		Timeout(time.Minute).
		ControlURL(url).
		Trace(true).
		SlowMotion(100 * time.Millisecond).
		MustConnect()


	//// Even you forget to close, rod will close it after main process ends.
	//  defer browser.MustClose()

	// Timeout will be passed to all chained function calls.
	// The code will panic out if any chained call is used after the timeout.
	page := browser.Timeout(30*time.Second).MustPage(startPage)

	// Make sure viewport is always consistent.
	page.MustSetViewport(screenWidth, screenHeigth, 1, false)

	// We use css selector to get the search input element and input "git"

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


//
//
//
//
//
//
//
//
//
