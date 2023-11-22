// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/durability"
)

func TestDurabilityIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RS.Min = 3
				config.Metainfo.RS.Repair = 5
				config.Metainfo.RS.Success = 5
				config.Metainfo.RS.Total = 6
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		{
			// upload object
			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)

			_, err = project.CreateBucket(ctx, "bucket1")
			assert.NoError(t, err)

			for i := 0; i < 10; i++ {

				object, err := project.UploadObject(ctx, "bucket1", fmt.Sprintf("key%d", i), nil)
				assert.NoError(t, err)

				_, err = object.Write(make([]byte, 10240))
				assert.NoError(t, err)

				err = object.Commit()
				assert.NoError(t, err)
			}

			require.NoError(t, project.Close())
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

		rangedLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service
		_, err := rangedLoopService.RunOnce(ctx)

		require.Len(t, result, 15)
		// one or two pieces are controlled out of the 5-6 --> 3 or 4 pieces are available without HU nodes
		require.True(t, result["HU"].Min() > 2)
		require.True(t, result["HU"].Min() < 5)
		require.NoError(t, err)
	})
}
