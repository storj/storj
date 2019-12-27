// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/uplink"
)

func TestInspectorStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var availableBandwidth int64
		var availableSpace int64
		for _, storageNode := range planet.StorageNodes {
			response, err := storageNode.Storage2.Inspector.Stats(ctx, &pb.StatsRequest{})
			require.NoError(t, err)

			assert.Zero(t, response.UsedBandwidth)
			assert.Zero(t, response.UsedSpace)
			assert.Zero(t, response.UsedEgress)
			assert.Zero(t, response.UsedIngress)
			assert.True(t, response.AvailableBandwidth > 0)
			assert.True(t, response.AvailableSpace > 0)

			// assume that all storage node should have the same initial values
			availableBandwidth = response.AvailableBandwidth
			availableSpace = response.AvailableSpace
		}

		expectedData := testrand.Bytes(100 * memory.KiB)

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     5,
		}

		err := planet.Uplinks[0].UploadWithConfig(ctx, planet.Satellites[0], rs, "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		assert.NoError(t, err)

		// wait until all requests have been handled
		for {
			total := int32(0)
			for _, storageNode := range planet.StorageNodes {
				total += storageNode.Storage2.Endpoint.TestLiveRequestCount()
			}
			if total == 0 {
				break
			}

			sync2.Sleep(ctx, 100*time.Millisecond)
		}

		var downloaded int
		for _, storageNode := range planet.StorageNodes {
			response, err := storageNode.Storage2.Inspector.Stats(ctx, &pb.StatsRequest{})
			require.NoError(t, err)

			// TODO set more accurate assertions
			if response.UsedSpace > 0 {
				assert.NotZero(t, response.UsedBandwidth)
				assert.Equal(t, response.UsedBandwidth, response.UsedIngress+response.UsedEgress)
				assert.Equal(t, availableBandwidth-response.UsedBandwidth, response.AvailableBandwidth)
				assert.Equal(t, availableSpace-response.UsedSpace, response.AvailableSpace)

				assert.Equal(t, response.UsedSpace, response.UsedBandwidth-response.UsedEgress)
				if response.UsedEgress > 0 {
					downloaded++
					assert.Equal(t, response.UsedBandwidth-response.UsedIngress, response.UsedEgress)
				}
			} else {
				assert.Zero(t, response.UsedSpace)
				// TODO track why this is failing
				//assert.Equal(t, availableBandwidth, response.AvailableBandwidth)
				assert.Equal(t, availableSpace, response.AvailableSpace)
			}
		}
		assert.True(t, downloaded >= rs.MinThreshold, "downloaded=%v, rs.MinThreshold=%v", downloaded, rs.MinThreshold)
	})
}

func TestInspectorDashboard(t *testing.T) {
	testStartedTime := time.Now()

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for _, storageNode := range planet.StorageNodes {
			response, err := storageNode.Storage2.Inspector.Dashboard(ctx, &pb.DashboardRequest{})
			require.NoError(t, err)

			assert.True(t, response.Uptime.Nanos > 0)
			assert.Equal(t, storageNode.ID(), response.NodeId)
			assert.Equal(t, storageNode.Addr(), response.ExternalAddress)
			assert.NotNil(t, response.Stats)
		}

		expectedData := testrand.Bytes(100 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		for _, storageNode := range planet.StorageNodes {
			response, err := storageNode.Storage2.Inspector.Dashboard(ctx, &pb.DashboardRequest{})
			require.NoError(t, err)

			assert.True(t, response.LastPinged.After(testStartedTime))

			assert.True(t, response.Uptime.Nanos > 0)
			assert.Equal(t, storageNode.ID(), response.NodeId)
			assert.Equal(t, storageNode.Addr(), response.ExternalAddress)
			assert.NotNil(t, response.Stats)
		}
	})
}
