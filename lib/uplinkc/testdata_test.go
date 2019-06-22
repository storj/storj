// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"os/exec"
	"path"
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
			StorageNodeCount: 6,
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

	libuplink := ctx.CompileShared(t, "uplink", "storj.io/storj/lib/uplinkc")

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

				testexe := ctx.CompileC(t, testName, []string{ctest}, libuplink, definition)

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

func TestLibstorj(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	libuplink := ctx.CompileShared(t, "uplink", "storj.io/storj/lib/uplinkc")

	currentdir, err := os.Getwd()
	require.NoError(t, err)

	definition := testcontext.Include{
		Header: filepath.Join(currentdir, "uplink_definitions.h"),
	}

	libstorjHeaders, err := filepath.Glob(filepath.Join(currentdir, "..", "libstorj", "src", "*.h"))
	require.NoError(t, err)

	testHeaders, err := filepath.Glob(filepath.Join(currentdir, "..", "libstorj", "test", "*.h"))
	require.NoError(t, err)

	var libstorjIncludes []testcontext.Include
	for _, headerPath := range append(libstorjHeaders, testHeaders...) {
		libstorjIncludes = append(libstorjIncludes, testcontext.Include{
			Header: headerPath,
		})
	}

	libstorjSrc, err := filepath.Glob(filepath.Join("..", "libstorj", "src", "*.c"))
	require.NoError(t, err)

	libstorjTestSrc, err := filepath.Glob(filepath.Join("..", "libstorj", "test", "*.c"))
	require.NoError(t, err)

	// TODO: remove "tests.c" from `libstorjTestSrc` slice
	ctests := []string{filepath.Join("..", "libstorj", "test", "tests.c")}
	require.NoError(t, err)

	t.Run("ALL", func(t *testing.T) {
		for _, ctest := range ctests {
			ctest := ctest
			t.Run(filepath.Base(ctest), func(t *testing.T) {
				t.Parallel()

				srcFiles := append(libstorjSrc, libstorjTestSrc...)
				includes := append([]testcontext.Include{
					libuplink,
					definition,
					testcontext.CLibJSON,
					testcontext.CLibNettle,
					testcontext.CLibUV,
					testcontext.CLibCurl,
					testcontext.CLibMath,
					testcontext.CLibMicroHTTPD,
				}, libstorjIncludes...)

				testexe := ctx.CompileC(t, path.Base(ctest), srcFiles, includes...)

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
