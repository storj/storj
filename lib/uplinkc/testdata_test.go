// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestC(t *testing.T) {
	ctx := testcontext.New(t)
	//	defer ctx.Cleanup()

	libuplink := ctx.CompileShared("uplink", "storj.io/storj/lib/uplinkc")

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
			t.Run(filepath.Base(ctest), func(t *testing.T) {
				t.Parallel()

				testexe := ctx.CompileC(ctest, libuplink, definition)

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
