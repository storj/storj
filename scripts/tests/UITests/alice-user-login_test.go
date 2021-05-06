package UITests

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func Test_alice_login_to_account(t *testing.T) {
	l := launcher.New().
		Headless(false).
		Devtools(false)
	url := l.MustLaunch()

	browser := rod.New().
		Timeout(time.Minute).
		ControlURL(url).
		Trace(true).
		SlowMotion(300 * time.Millisecond).
		MustConnect()

	page := browser.Timeout(25 * time.Second).MustPage("http://127.0.0.1:10002/login")
	page.MustSetViewport(1350, 600, 1, false)
	page.MustElement(".headerless-input").MustInput("alice@mail.test")
	page.MustElement("[type=password]").MustInput("123a123")
	page.Keyboard.MustPress(input.Enter)
	// check title
	assert.True(t, strings.Contains(page.MustElement(".dashboard-area__header-wrapper__title").MustText(), "Dashboard"))

	defer browser.MustClose()
}
