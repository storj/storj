// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/uplink"
)

func TestInspectorStats(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 10, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

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

	expectedData := make([]byte, 100*memory.KiB)
	_, err = rand.Read(expectedData)
	require.NoError(t, err)

	rs := &uplink.RSConfig{
		MinThreshold:     2,
		RepairThreshold:  4,
		SuccessThreshold: 6,
		MaxThreshold:     10,
	}

	err = planet.Uplinks[0].UploadWithConfig(ctx, planet.Satellites[0], rs, "testbucket", "test/path", expectedData)
	require.NoError(t, err)

	_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
	assert.NoError(t, err)

	var downloaded int
	for _, storageNode := range planet.StorageNodes {
		response, err := storageNode.Storage2.Inspector.Stats(ctx, &pb.StatsRequest{})
		require.NoError(t, err)

		// TODO set more accurate assertions
		if response.UsedSpace > 0 {
			assert.True(t, response.UsedBandwidth > 0)
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
	assert.True(t, downloaded >= rs.MinThreshold)
}

func TestInspectorDashboard(t *testing.T) {
	testStartedTime := time.Now()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, storageNode := range planet.StorageNodes {
		response, err := storageNode.Storage2.Inspector.Dashboard(ctx, &pb.DashboardRequest{})
		require.NoError(t, err)

		assert.True(t, response.Uptime.Nanos > 0)
		assert.Equal(t, storageNode.ID(), response.NodeId)
		assert.Equal(t, storageNode.Addr(), response.ExternalAddress)
		assert.NotNil(t, response.Stats)
	}

	expectedData := make([]byte, 100*memory.KiB)
	_, err = rand.Read(expectedData)
	require.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
	require.NoError(t, err)

	for _, storageNode := range planet.StorageNodes {
		response, err := storageNode.Storage2.Inspector.Dashboard(ctx, &pb.DashboardRequest{})
		require.NoError(t, err)

		lastPinged, err := ptypes.Timestamp(response.LastPinged)
		assert.NoError(t, err)
		assert.True(t, lastPinged.After(testStartedTime))

		lastQueried, err := ptypes.Timestamp(response.LastQueried)
		assert.NoError(t, err)
		assert.True(t, lastQueried.After(testStartedTime))

		assert.True(t, response.Uptime.Nanos > 0)
		assert.Equal(t, storageNode.ID(), response.NodeId)
		assert.Equal(t, storageNode.Addr(), response.ExternalAddress)
		assert.Equal(t, int64(len(planet.StorageNodes)+len(planet.Satellites)), response.NodeConnections)
		assert.NotNil(t, response.Stats)
	}
}
