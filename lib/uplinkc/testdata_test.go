// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestC(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	libuplink := ctx.CompileShared("uplink", "storj.io/storj/lib/uplinkc")

	currentdir, err := os.Getwd()
	require.NoError(t, err)

	definition := testcontext.Include{Header: filepath.Join(currentdir, "uplink_definitions.h")}

	ctests, err := filepath.Glob(filepath.Join("testdata", "*_test.c"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	defer wg.Wait()
	for _, ctest := range ctests {
		wg.Add(1)

		ctest := ctest
		t.Run(filepath.Base(ctest), func(t *testing.T) {
			defer wg.Done()

			testexe := ctx.CompileC(ctest, libuplink, definition)

			planet, err := testplanet.NewCustom(
				zaptest.NewLogger(t),
				testplanet.Config{
					SatelliteCount:   1,
					StorageNodeCount: 6,
					UplinkCount:      1,
					Reconfigure:      testplanet.DisablePeerCAWhitelist,
				},
			)
			require.NoError(t, err)

			planet.Start(ctx)
			defer ctx.Check(planet.Shutdown)

			cmd := exec.Command(testexe)
			cmd.Env = append(os.Environ(),
				"SATELLITE_0_ADDR="+planet.Satellites[0].Addr(),
				"GATEWAY_0_API_KEY="+planet.Uplinks[0].APIKey[planet.Satellites[0].ID()],
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Error(string(out))
				t.Fatal(err)
			} else {
				t.Log(out)
			}
		})
	}
}
