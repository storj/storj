package UITests

import (
	"fmt"
	"strings"
)

func Example_login() {

	page, browser := login_to_account()
	//check title
	fmt.Println(strings.Contains(page.MustElement(".dashboard-area__title").MustText(),"Dashboard"))
	// Output: true
	defer browser.MustClose()
}
