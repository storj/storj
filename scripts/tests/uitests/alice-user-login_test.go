// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitests

import (
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/assert"
)

func TestAliceLoginToAccount(t *testing.T) {
	loginPageUrl := "http://127.0.0.1:10002/login"
	aliceEmail := "alice@mail.test"
	alicePassword := "123a123"
	l := launcher.New().
		Headless(false).
		Devtools(false)
	defer l.Cleanup()
	url := l.MustLaunch()

	browser := rod.New().
		Timeout(time.Minute).
		ControlURL(url).
		Trace(true).
		SlowMotion(300 * time.Millisecond).
		MustConnect()
	defer browser.MustClose()
	page := browser.Timeout(25 * time.Second).MustPage(loginPageUrl)
	page.MustSetViewport(1350, 600, 1, false)
	page.MustElement(".headerless-input").MustInput(aliceEmail)
	page.MustElement("[type=password]").MustInput(alicePassword)
	page.Keyboard.MustPress(input.Enter)
	// check title
	assert.True(t, strings.Contains(page.MustElement(".dashboard-area__header-wrapper__title").MustText(), "Dashboard"))
}
