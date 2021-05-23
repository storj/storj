// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitests

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
	"go.uber.org/zap/zaptest"
)

func TestBranding(t *testing.T) {
	log := zaptest.NewLogger(t)
	logFunc := utils.Log(func(msg ...interface{}) {
		log.Info(fmt.Sprintln(msg...))
	})
	loginPageUrl := "http://localhost:10002/login"
	l := launcher.New().
		Headless(true).
		Devtools(false)
	defer l.Cleanup()
	url := l.MustLaunch()

	browser := rod.New().
		Timeout(time.Minute).
		ControlURL(url).
		Trace(true).
		SlowMotion(300 * time.Millisecond).
		Logger(logFunc).
		MustConnect()
	defer browser.MustClose()
	page := browser.Timeout(25 * time.Second).MustPage(loginPageUrl)
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustWaitLoad().MustScreenshot(path.Join(os.Getenv("WORKSPACE"), "data", "branding.png"))
}
