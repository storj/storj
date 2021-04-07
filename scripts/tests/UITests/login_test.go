package UITests

import (
	"fmt"
	"strings"
	"testing"
)

func Test_login(t *testing.T) {

	page, browser := login_to_account()
	//check title
	fmt.Println(strings.Contains(page.MustElement(".dashboard-area__title").MustText(),"Dashboard"))
	// Output: true
	defer browser.MustClose()
}
