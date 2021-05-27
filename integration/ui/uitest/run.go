// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

// Test defines common services for uitests.
type Test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser)

type zapWriter struct {
	*zap.Logger
}

func (log zapWriter) Write(data []byte) (int, error) {
	log.Logger.Info(string(data))
	return len(data), nil
}

// Run starts a new UI test.
func Run(t *testing.T, test Test) {
	if os.Getenv("STORJ_TEST_SATELLITE_WEB") == "" {
		t.Skip("Enable UI tests by setting STORJ_TEST_SATELLITE_WEB to built npm")
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.StaticDir = os.Getenv("STORJ_TEST_SATELLITE_WEB")
			},
		},
		NonParallel: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		showBrowser := os.Getenv("STORJ_TEST_SHOW_BROWSER") != ""

		logLauncher := zaptest.NewLogger(t).Named("launcher")

		launch := launcher.New().
			Headless(!showBrowser).
			Leakless(false).
			Devtools(false).
			NoSandbox(true).
			Logger(zapWriter{Logger: logLauncher})
		if browserBin := os.Getenv("STORJ_TEST_BROWSER"); browserBin != "" {
			launch = launch.Bin(browserBin)
		}
		defer launch.Cleanup()

		url, err := launch.Launch()
		require.NoError(t, err)

		logBrowser := zaptest.NewLogger(t).Named("rod")

		browser := rod.New().
			Timeout(time.Minute).
			ControlURL(url).
			SlowMotion(300 * time.Millisecond).
			Logger(utils.Log(func(msg ...interface{}) {
				logBrowser.Info(fmt.Sprintln(msg...))
			})).
			Context(ctx)
		defer ctx.Check(browser.Close)

		require.NoError(t, browser.Connect())

		test(t, ctx, planet, browser)
	})
}
