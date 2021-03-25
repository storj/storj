package UITests

import "fmt"

func Example_droplistChosing (){
	page, browser := login_to_account()
	defer browser.MustClose()
	firstElement:= page.MustElement("div.resources-selection__toggle-container").MustClick().MustElement("a.resources-dropdown__item-container").MustText()
	fmt.Println(firstElement, page.MustInfo().URL)
	// Output: Docs http://127.0.0.1:10002/project-dashboard
}
