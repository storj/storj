// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
)

func TestC(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	libuplink := ctx.CompileShared("uplink", "storj.io/storj/lib/uplinkc")

	ctests, err := filepath.Glob(filepath.Join("testdata", "*_test.c"))
	require.NoError(t, err)

	for _, ctest := range ctests {
		ctest := ctest
		t.Run(filepath.Base(ctest), func(t *testing.T) {
			testexe := ctx.CompileC(ctest, libuplink)
			/*
				planet, err := testplanet.NewCustom(
					zaptest.NewLogger(t),
					testplanet.Config{
						SatelliteCount:   1,
						StorageNodeCount: 8,
						UplinkCount:      0,
						Reconfigure:      testplanet.DisablePeerCAWhitelist,
					},
				)
				require.NoError(t, err)

				planet.Start(ctx)
				defer ctx.Check(planet.Shutdown)
			*/
			out, err := exec.Command(testexe).CombinedOutput()
			if err != nil {
				t.Error(string(out))
				t.Fatal(err)
			} else {
				t.Log(out)
			}

			/*
				consoleProject := newProject(t, planet)
				consoleApikey := newAPIKey(t, ctx, planet, consoleProject.ID)
				satelliteAddr := planet.Satellites[0].Addr()

				envVars := []string{
					"SATELLITE_ADDR=" + satelliteAddr,
					"APIKEY=" + consoleApikey,
				}

				runCTest(t, ctx, ctest, envVars...)
			*/
		})
	}
}

/*
func TestGetIDVersion(t *testing.T) {
	var cErr CCharPtr
	idVersionNumber := storj.LatestIDVersion().Number

	cIDVersion := GetIDVersion(CUint(idVersionNumber), &cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotNil(t, cIDVersion)

	assert.Equal(t, idVersionNumber, storj.IDVersionNumber(cIDVersion.number))
}
*/
