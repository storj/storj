package UITests

import (
	"fmt"
	"testing"
)

func Test_checkingElementsSideMenu(t *testing.T){
	page, browser := login_to_account()
	defer browser.MustClose()
	first := page.MustElement("a.navigation-area__item-container:nth-of-type(1)").MustText()
	fmt.Println(first)
	second := page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustText()
	fmt.Println(second)
	third := page.MustElement("a.navigation-area__item-container:nth-of-type(3)").MustText()
	fmt.Println(third)
	currentProject := page.MustHas("#app > div > div > div.dashboard__wrap__main-area > div.navigation-area.regular-navigation > div > div")
	fmt.Println(currentProject)
	// Output: Dashboard
	// Access
	// Users
	// true
}

