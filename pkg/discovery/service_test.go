// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestCache_Refresh(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		for _, storageNode := range planet.StorageNodes {
			node, err := satellite.Overlay.Service.Get(ctx, storageNode.ID())
			if assert.NoError(t, err) {
				assert.Equal(t, storageNode.Addr(), node.Address.Address)
			}
		}
	})
}

func TestCache_Graveyard(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	satellite := planet.Satellites[0]
	offline := planet.StorageNodes[0].ID()

	k := satellite.Kademlia.Service
	k.WaitForBootstrap() // redundant, but leaving here to be clear

	seen := k.Seen()
	assert.NotNil(t, seen)

	err = satellite.Overlay.Service.Delete(ctx, offline)
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)

	node, err := satellite.Overlay.Service.Get(ctx, offline)
	assert.NoError(t, err)
	assert.NotNil(t, node)
	assert.Equal(t, node.Id, offline)
}
