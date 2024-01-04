// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestGetSettings(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Admin.Admin.Service

		_, apiErr := service.GetSettings(ctx)
		require.NoError(t, apiErr.Err)
	})
}
