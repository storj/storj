// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/durability"
	"storj.io/storj/shared/location"
	"storj.io/storj/storagenode"
)

func TestDurabilityIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(testplanet.ReconfigureRS(3, 5, 6, 6),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.DurabilityReport.Enabled = true
					config.Durability.Classes = []string{"last_net", "last_ip", "wallet", "email"}
				}),
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

		planet.StorageNodes[0].Contact.Chore.Pause(ctx)
		planet.StorageNodes[1].Contact.Chore.Pause(ctx)

		{
			// we uploaded to 5 nodes, having 2 node in HU means that we control at least 1 piece, but max 2
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, planet.StorageNodes[0].NodeURL().ID, location.Hungary.String()))
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, planet.StorageNodes[1].NodeURL().ID, location.Hungary.String()))
		}

		result := map[int]map[int]durability.Bucket{}
		for i := 0; i < 3; i++ {
			result[i] = map[int]durability.Bucket{}
		}
		for _, observer := range planet.Satellites[0].RangedLoop.DurabilityReport.Observer {
			if observer.Class != "email" {
				continue
			}
			observer.TestChangeReporter(func(n time.Time, class string, missingProvider int, ix int, p storj.PlacementConstraint, stat durability.Bucket, resolver func(id durability.ClassID) string) {
				result[missingProvider][ix] = stat
			})
		}

		// durability reports are executed sequentially one by one with each loop iteration
		// we need as many loop iterations as much observers we have to collect all results
		for range planet.Satellites[0].RangedLoop.DurabilityReport.Observer {
			rangedLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service
			_, err := rangedLoopService.RunOnce(ctx)
			require.NoError(t, err)
		}

		// normal distribution --> we have 3 pieces from each segment (10)
		require.Equal(t, 10, result[0][3].SegmentCount)

		// we used all 3 test@storj.io, and 6 pieces. Without test@storj.io, only 3 remained --> which is RS=3 + 0 pieces
		require.Equal(t, 10, result[1][0].SegmentCount)

	})
}
