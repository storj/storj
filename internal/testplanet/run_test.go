// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"testing"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestRun(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Log("running test")
	})
}
