// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/geoip"
	"storj.io/storj/shared/location"
	"storj.io/storj/storagenode"
)

func TestGeoIPMock(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test does not work with macOS")
	}
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 0,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
					config.Overlay.GeoIP.MockCountries = []string{"US", "GB"}
				},
				StorageNode: func(index int, config *storagenode.Config) {
					config.Server.Address = fmt.Sprintf("127.0.201.%d:0", index+1)
				},
			},
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			// ensure storage nodes checked in with satellite
			for _, node := range planet.StorageNodes {
				node.Contact.Chore.TriggerWait(ctx)
			}

			// expected country codes per node index
			countryCodes := map[int]location.CountryCode{
				0: location.UnitedKingdom,
				1: location.UnitedStates,
				2: location.UnitedKingdom,
			}

			// check the country code for each storage nodes
			for i, node := range planet.StorageNodes {
				dossier, err := planet.Satellites[0].API.Overlay.DB.Get(ctx, node.ID())
				require.NoError(t, err)
				assert.Equal(t, countryCodes[i], dossier.CountryCode)
			}

			// change country in the mock GeoIP service from US to CA
			planet.Satellites[0].Overlay.Service.GeoIP = geoip.NewMockIPToCountry([]string{"CA", "GB"})

			// wait for storage nodes checked in again with satellite
			for _, node := range planet.StorageNodes {
				node.Contact.Chore.TriggerWait(ctx)
			}

			// adjust expected country codes for node 1
			countryCodes[1] = location.Canada

			// check the country code for each storage nodes
			for i, node := range planet.StorageNodes {
				dossier, err := planet.Satellites[0].API.Overlay.DB.Get(ctx, node.ID())
				require.NoError(t, err)
				assert.Equal(t, countryCodes[i], dossier.CountryCode)
			}
		},
	)
}
