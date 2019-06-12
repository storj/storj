// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
)

func TestCTests(t *testing.T) {
	ctests, err := filepath.Glob(filepath.Join(CTestsDir, "*_test.c"))
	require.NoError(t, err)

	t.Run("all", func(t *testing.T) {
		for _, ctest := range ctests {
			ctest := ctest
			t.Run(ctest, func(t *testing.T) {
				t.Parallel()

				ctx := testcontext.New(t)
				defer ctx.Cleanup()

				planet := startTestPlanet(t, ctx)
				defer ctx.Check(planet.Shutdown)

				consoleProject := newProject(t, planet)
				consoleApikey := newAPIKey(t, ctx, planet, consoleProject.ID)
				satelliteAddr := planet.Satellites[0].Addr()

				envVars := []string{
					"SATELLITE_ADDR=" + satelliteAddr,
					"APIKEY=" + consoleApikey,
				}

				runCTest(t, ctx, ctest, envVars...)
			})
		}
	})

	// NB: delete shared object if it exist to satisfy check-clean-directory.go
	_, err = os.Stat(LibuplinkSO)
	if err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	_ = os.Remove(LibuplinkSO)
}

func TestGetIDVersion(t *testing.T) {
	var cErr Cchar
	idVersionNumber := storj.LatestIDVersion().Number

	cIDVersion := GetIDVersion(CUint(idVersionNumber), &cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotNil(t, cIDVersion)

	assert.Equal(t, idVersionNumber, storj.IDVersionNumber(cIDVersion.number))
}
