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
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet) {
		sat := planet.Satellites[0]
		apiAddr := sat.API.Console.Listener.Addr().String()
		addrParts := strings.Split(apiAddr, ":")
		require.Len(t, addrParts, 2)

		cmd := exec.Command("npm", "run", "test-ci")
		portOverrideEnv := fmt.Sprintf(`PLAYWRIGHT_PORT=%s`, addrParts[1])
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, portOverrideEnv)
		out, err := cmd.CombinedOutput()
		t.Log(string(out))
		require.NoError(t, err)
	})
}
