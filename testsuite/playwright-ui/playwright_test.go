// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	uitest "storj.io/storj/testsuite/playwright-ui/server"
)

func TestRun(t *testing.T) {
	runNpmScript(t, uitest.Run, "test-ci")
}

func TestRunWhiteLabel(t *testing.T) {
	runNpmScript(t, uitest.RunWhiteLabel, "test-ci-whitelabel")
}

func TestDevRunWhiteLabel(t *testing.T) {
	runNpmScript(t, uitest.RunWhiteLabel, "test-dev-whitelabel-ui")
}

func runNpmScript(t *testing.T, runner func(*testing.T, uitest.Test), script string) {
	runner(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet) {
		apiAddr := planet.Satellites[0].API.Console.Listener.Addr().String()
		addrParts := strings.Split(apiAddr, ":")
		require.Len(t, addrParts, 2)

		cmd := exec.Command("npm", "run", script)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PLAYWRIGHT_PORT=%s", addrParts[1]))
		out, err := cmd.CombinedOutput()
		t.Log(string(out))
		require.NoError(t, err)
	})
}
