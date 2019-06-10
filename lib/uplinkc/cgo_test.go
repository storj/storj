// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestC(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ctests, err := filepath.Glob(filepath.Join("testdata", "*_test.c"))
	require.NoError(t, err)

	for _, ctest := range ctests {
		ctest := ctest
		t.Run(ctest, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

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

func TestGetIDVersion(t *testing.T) {
	var cErr CCharPtr
	idVersionNumber := storj.LatestIDVersion().Number

	cIDVersion := GetIDVersion(CUint(idVersionNumber), &cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotNil(t, cIDVersion)

	assert.Equal(t, idVersionNumber, storj.IDVersionNumber(cIDVersion.number))
}
