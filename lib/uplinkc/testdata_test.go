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

				testexe := ctx.CompileC(t, testcontext.CompileCOptions{
					Dest:     testName,
					Sources:  []string{ctest},
					Includes: []testcontext.Include{libuplink, definition},
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

func TestLibstorj(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	libuplink := ctx.CompileShared(t, "uplink", "storj.io/storj/lib/uplinkc")

	currentdir, err := os.Getwd()
	require.NoError(t, err)

	definition := testcontext.Include{
		Header: filepath.Join(currentdir, "uplink_definitions.h"),
	}

	var libstorjIncludes []testcontext.Include
	libstorjHeader := testcontext.Include{
		Header: filepath.Join(currentdir, "..", "libstorj", "src", "storj.h"),
	}

	srcFiles := []string{
		//"bip39.c",
		//"crypto.c",
		//"downloader.c",
		//"http.c",
		//"rs.c",
		"storj.c",
		//"uploader.c",
		//"utils.c",
	}
	for i, base := range srcFiles {
		srcFiles[i] = filepath.Join(currentdir, "..", "libstorj", "src", base)
	}

	testHeaders := []string{
		//"storjtests.h",
		//"mockbridge.json.h",
		//"mockbridgeinfo.json.h",
	}
	for _, headerPath := range testHeaders {
		libstorjIncludes = append(libstorjIncludes, testcontext.Include{
			Header: headerPath,
		})
	}

	testFiles := []string{
		//"mockbridge.c",
		//"mockfarmer.c",
		"tests.c",
	}
	for i, base := range testFiles {
		testFiles[i] = filepath.Join(currentdir, "..", "libstorj", "test", base)
	}

	includes := append([]testcontext.Include{
		libuplink,
		definition,
		testcontext.CLibJSON,
		testcontext.CLibNettle,
		testcontext.CLibUV,
		testcontext.CLibCurl,
		testcontext.CLibMath,
		testcontext.CLibMicroHTTPD,
		libstorjHeader,
	}, libstorjIncludes...)

	testexe := ctx.CompileC(t, testcontext.CompileCOptions{
		Dest:     "libstorj",
		Sources:  append(srcFiles, testFiles...),
		Includes: includes,
		NoWarn:   true,
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
}
