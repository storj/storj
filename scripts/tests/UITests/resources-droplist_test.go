package UITests

import (
	"gotest.tools/assert"
	"testing"
)

func TestResourcesDroplist(t *testing.T) {
		page, browser := login_to_account()
		defer browser.MustClose()
		page.MustElement("div.resources-selection__toggle-container").MustClick()
		listLength:= len(page.MustElements("a.resources-dropdown__item-container"))
		assert.Equal(t,3,listLength)
		docsText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(1)").MustText()
		assert.Equal(t,"Docs",docsText)
		docsLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(1)").MustAttribute("href")
		assert.Equal(t,"https://docs.storj.io/node",docsLink)
		communityText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(2)").MustText()
		assert.Equal(t,"Community",communityText)
		communityLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(2)").MustAttribute("href")
		assert.Equal(t,"https://storj.io/community/",communityLink)
		supportText:= page.MustElement("a.resources-dropdown__item-container:nth-of-type(3)").MustText()
		assert.Equal(t,"Support",supportText)
		supportLink:= *page.MustElement("a.resources-dropdown__item-container:nth-of-type(3)").MustAttribute("href")
		assert.Equal(t,"mailto:support@storj.io",supportLink)
	}

