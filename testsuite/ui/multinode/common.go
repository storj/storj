// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"github.com/go-rod/rod"
)

func openPage(browser *rod.Browser, url string) *rod.Page {
	page := browser.MustPage()
	page.MustSetViewport(1350, 600, 1, false)
	page.MustNavigate(url).MustWaitLoad()
	return page
}
