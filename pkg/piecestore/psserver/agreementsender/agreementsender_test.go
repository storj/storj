// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestSendAgreementsToSatellite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.NewCustom(zaptest.NewLogger(t), testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	})
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	before := time.Now()
	for _, node := range planet.StorageNodes {
		node.Agreements.Sender.Loop.Pause()
	}

	// upload a file
	data := make([]byte, 500*memory.KiB)
	_, err = rand.Read(data)
	assert.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path/first", data)
	assert.NoError(t, err)

	// collect all Total's for uploaded pieces to compare when sent to satellite
	putAllocation := make(map[storj.NodeID]int64)
	for _, node := range planet.StorageNodes {
		allocations, err := node.DB.PSDB().GetBandwidthAllocations()
		assert.NoError(t, err)

		allocPerSat := allocations[planet.Satellites[0].ID()]
		if len(allocPerSat) > 0 {
			assert.Equal(t, 1, len(allocPerSat))
			putAllocation[node.ID()] = allocPerSat[0].Agreement.Total
		}
	}

	for _, node := range planet.StorageNodes {
		node.Agreements.Sender.Loop.TriggerWait()
	}

	// check if agreements were deleted from storage node
	for _, node := range planet.StorageNodes {
		allocations, err := node.DB.PSDB().GetBandwidthAllocations()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allocations))
	}

	satAgreements := planet.Satellites[0].DB.BandwidthAgreement()

	satAllocs, err := satAgreements.GetTotals(ctx, before, time.Now())
	assert.NoError(t, err)
	for node, nodeTotal := range putAllocation {
		satTotal := satAllocs[node]
		assert.NotNil(t, satTotal)
		// 0 in array is PUT
		assert.Equal(t, nodeTotal, satTotal[0])
	}

	// sending second file
	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path/second", data)
	assert.NoError(t, err)

	for _, node := range planet.StorageNodes {
		allocations, err := node.DB.PSDB().GetBandwidthAllocations()
		assert.NoError(t, err)

		allocPerSat := allocations[planet.Satellites[0].ID()]
		if len(allocPerSat) > 0 {
			assert.Equal(t, 1, len(allocPerSat))
			// sum with previous upload PUTs
			putAllocation[node.ID()] += allocPerSat[0].Agreement.Total
		}
	}

	for _, node := range planet.StorageNodes {
		node.Agreements.Sender.Loop.TriggerWait()
	}

	// check if agreements were deleted from storage node
	for _, node := range planet.StorageNodes {
		allocations, err := node.DB.PSDB().GetBandwidthAllocations()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allocations))
	}

	satAllocs, err = satAgreements.GetTotals(ctx, before, time.Now())
	assert.NoError(t, err)
	for node, nodeTotal := range putAllocation {
		satTotal := satAllocs[node]
		assert.NotNil(t, satTotal)
		// 0 in array is PUT
		assert.Equal(t, nodeTotal, satTotal[0])
	}
}
