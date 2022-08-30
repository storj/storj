// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/defaults"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/utils"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

// Our testing suite heavily uses randomly selected ports, which may collide
// with the launcher lock port. We'll disable the lock port entirely for
// the time being.
func init() { defaults.LockPort = 0 }

// Browser starts a browser for testing using environment variables for configuration.
func Browser(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, fn func(*rod.Browser)) {

	logLauncher := planet.Log().Named("launcher")

	/* browserLoaded := browserTimeoutDetector(25 * time.Second)
	defer browserLoaded()
	*/

	launch := launcher.New().
		Headless(true).
		Leakless(true).
		Devtools(false).
		NoSandbox(true).
		UserDataDir(ctx.Dir("browser")).
		Logger(zapWriter{Logger: logLauncher})

	if browserHost := os.Getenv("STORJ_TEST_BROWSER_HOSTPORT"); browserHost != "" {
		host, port, err := net.SplitHostPort(browserHost)
		require.NoError(t, err)
		launch = launch.Set("remote-debugging-address", host).Set(flags.RemoteDebuggingPort, port)
	}

	if browserBin := os.Getenv("STORJ_TEST_BROWSER"); browserBin != "" {
		launch = launch.Bin(browserBin)
	}

	/* defer func() {
		launch.Kill()
		avoidStall(3*time.Second, launch.Cleanup)
	}() */

	url, err := launch.Launch()
	require.NoError(t, err)

	logBrowser := planet.Log().Named("rod")
	logBrowserCDP := logBrowser.Named("cdp")

	client := cdp.New(url).Logger(utils.Log(func(msg ...interface{}) {
		logBrowserCDP.Debug(fmt.Sprintln(msg...))
	}))

	browser := rod.New().
		Timeout(time.Minute).
		Sleeper(MaxDuration(10 * time.Second)).
		Client(client).
		Logger(utils.Log(func(msg ...interface{}) {
			logBrowser.Info(fmt.Sprintln(msg...))
		})).
		Context(ctx).
		WithPanic(func(v interface{}) { require.Fail(t, "check failed", v) })

	if slowBrowser == "slow" {
		browser = browser.SlowMotion(100 * time.Millisecond).Trace(true)
	}

	/* defer ctx.Check(func() error {
		// browser.Close may sometimes return context.Canceled.
		return errs2.IgnoreCanceled(browser.Close())
	})
	*/
	require.NoError(t, browser.Connect())

	// browserLoaded()

	fn(browser)
}

// MaxDuration returns a sleeper constructor with the max duration.
func MaxDuration(max time.Duration) func() utils.Sleeper {
	return func() utils.Sleeper {
		singleSleep := 100 * time.Millisecond
		totalSlept := time.Duration(0)
		return func(ctx context.Context) error {
			if totalSlept > max {
				return errMaxSleepDuration(max)
			}
			if singleSleep > 500*time.Millisecond {
				singleSleep = 500 * time.Millisecond
			}

			totalSlept += singleSleep
			t := time.NewTimer(singleSleep)
			defer t.Stop()
			select {
			case <-t.C:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		}
	}
}

// errMaxSleepDuration is error for exceeding sleep duration.
type errMaxSleepDuration time.Duration

// Error implements error interface.
func (e errMaxSleepDuration) Error() string {
	return fmt.Sprintf("max sleep %v exceeded", time.Duration(e))
}
