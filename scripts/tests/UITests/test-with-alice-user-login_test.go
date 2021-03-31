package UITests

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"strings"
	"time"
)

func Example_alice_login_to_account() {
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
	page := browser.Timeout(25 * time.Second).MustPage(startPage)

	// Make sure viewport is always consistent.
	page.MustSetViewport(screenWidth, screenHeigth, 1, false)

	// We use css selector to get the search input element and input "git"
	page.MustElement(".headerless-input").MustInput("alice@mail.test")
	page.MustElement("[type=password]").MustInput("123a123")

	page.Keyboard.MustPress(input.Enter)

	//check title
	fmt.Println(strings.Contains(page.MustElement(".dashboard-area__title").MustText(), "Dashboard"))
	// Output: true
	defer browser.MustClose()

}