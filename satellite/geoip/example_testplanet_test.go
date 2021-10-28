// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip_test

import (
	"testing"

	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGeoIPMock(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 2,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
					config.Overlay.GeoIP.MockCountries = []string{"US", "GB"}
				},
			},
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {},
	)
}
