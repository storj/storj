// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func RunPlanet(t *testing.T, run func(ctx *testcontext.Context, planet *testplanet.Planet)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.NewCustom(
		zaptest.NewLogger(t, zaptest.Level(zapcore.WarnLevel)),
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 10,
			UplinkCount:      1,
			Reconfigure:      testplanet.DisablePeerCAWhitelist,
		},
	)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// make sure nodes are refreshed in db
	planet.Satellites[0].Discovery.Service.Refresh.TriggerWait()

	run(ctx, planet)
}

func TestC(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	libuplink_include := ctx.CompileShared(t, "uplink", "storj.io/storj/lib/uplinkc")

	currentdir, err := os.Getwd()
	require.NoError(t, err)

	definition := testcontext.Include{
		Header: filepath.Join(currentdir, "uplink_definitions.h"),
	}

	ctests, err := filepath.Glob(filepath.Join("testdata", "*_test.c"))
	require.NoError(t, err)

	t.Run("ALL", func(t *testing.T) {
		for _, ctest := range ctests {
			ctest := ctest
			testName := filepath.Base(ctest)
			t.Run(testName, func(t *testing.T) {
				t.Parallel()

				testexe := ctx.CompileC(t, testcontext.CompileCOptions{
					Dest:    testName,
					Sources: []string{ctest},
					Includes: []testcontext.Include{
						libuplink_include,
						definition,
						testcontext.CLibMath,
					},
				})

				RunPlanet(t, func(ctx *testcontext.Context, planet *testplanet.Planet) {
					cmd := exec.Command(testexe)
					cmd.Dir = filepath.Dir(testexe)
					cmd.Env = append(os.Environ(),
						"SATELLITE_0_ADDR="+planet.Satellites[0].Addr(),
						"GATEWAY_0_API_KEY="+planet.Uplinks[0].APIKey[planet.Satellites[0].ID()],
					)

					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Error(string(out))
						t.Fatal(err)
					} else {
						t.Log(string(out))
					}
				})
			})
		}
	})
}
