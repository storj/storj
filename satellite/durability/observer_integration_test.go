// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/durability"
	"storj.io/storj/storagenode"
)

func TestDurabilityIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 5, 6, 6),
			StorageNode: func(index int, config *storagenode.Config) {
				if index > 2 {
					config.Operator.Email = "test@storj.io"
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		{
			for i := 0; i < 10; i++ {
				err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket1", fmt.Sprintf("key%d", i), testrand.Bytes(10*memory.KiB))
				require.NoError(t, err)
			}
		}

		{
			// we uploaded to 5 nodes, having 2 node in HU means that we control at least 1 piece, but max 2
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, planet.StorageNodes[0].NodeURL().ID, location.Hungary.String()))
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, planet.StorageNodes[1].NodeURL().ID, location.Hungary.String()))
		}

		result := make(map[string]*durability.HealthStat)
		for _, observer := range planet.Satellites[0].RangedLoop.DurabilityReport.Observer {
			observer.TestChangeReporter(func(n time.Time, class string, value string, stat *durability.HealthStat) {
				result[value] = stat
			})
		}

		// durability reports are executed sequentially one by one with each loop iteration
		// we need as many loop iterations as much observers we have to collect all results
		for range planet.Satellites[0].RangedLoop.DurabilityReport.Observer {
			rangedLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service
			_, err := rangedLoopService.RunOnce(ctx)
			require.NoError(t, err)
		}

		require.Len(t, result, 14)

		// we used all 3 test@storj.io, and 6 pieces. Without test@storj.io, only 3 remained.
		require.NotNil(t, result["test@storj.io"])
		require.Equal(t, result["test@storj.io"].Min(), 3)

	})
}
