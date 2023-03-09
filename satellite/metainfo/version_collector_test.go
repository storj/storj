// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"io"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/environment"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/uplink"
)

func TestUserAgentTransferStats(t *testing.T) {
	// bacause we are testing monkit there is no easy way to separate
	// collected metrics from other tests run in parallel
	t.Skipf("this test can be run only locally and without other tests in parallel")

	monkitRegistry := monkit.NewRegistry()
	environment.Register(monkitRegistry)

	iteration := 0
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		iteration++

		project, err := uplink.Config{
			UserAgent: "uplink-cli", // we need to use known user agent to
		}.OpenProject(ctx, planet.Uplinks[0].Access[planet.Satellites[0].ID()])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		objects := map[string]memory.Size{
			"inline": 1 * memory.KiB,
			"remote": 10 * memory.KiB,
		}

		_, err = project.EnsureBucket(ctx, "testbucket")
		require.NoError(t, err)

		for name, size := range objects {
			upload, err := project.UploadObject(ctx, "testbucket", name, nil)
			require.NoError(t, err)

			_, err = upload.Write(testrand.Bytes(size))
			require.NoError(t, err)

			require.NoError(t, upload.Commit())

			download, err := project.DownloadObject(ctx, "testbucket", name, nil)
			require.NoError(t, err)

			_, err = io.ReadAll(download)
			require.NoError(t, err)
			require.NoError(t, download.Close())
		}

		monkitRegistry.Stats(func(key monkit.SeriesKey, field string, val float64) {
			if key.Measurement == "user_agents_transfer_stats" && key.Tags.Get("user_agent") == "uplink-cli" && field == "total" {
				if key.Tags.Get("type") == "upload" {
					require.Equal(t, iteration*(11*memory.KiB.Int()), int(val))
				} else if key.Tags.Get("type") == "download" {
					require.Equal(t, iteration*(11*memory.KiB.Int()), int(val))
				}
			}
		})
	})
}
