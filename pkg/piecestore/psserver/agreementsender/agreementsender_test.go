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

	expectedData := make([]byte, 500*memory.KiB)
	_, err = rand.Read(expectedData)
	assert.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", expectedData)
	assert.NoError(t, err)

	numOfAllocations := 0
	for _, node := range planet.StorageNodes {
		allocations, err := node.DB.PSDB().GetBandwidthAllocations()
		assert.NoError(t, err)
		numOfAllocations += len(allocations)
	}

	for _, node := range planet.StorageNodes {
		node.Agreements.Sender.Loop.Trigger()
	}

	time.Sleep(500 * time.Millisecond)

	satAgreements := planet.Satellites[0].DB.BandwidthAgreement()
	all, err := satAgreements.GetTotals(ctx, before, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, numOfAllocations, len(all))
}
