// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestCache_Refresh(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	time.Sleep(2 * time.Second)

	satellite := planet.Satellites[0]
	for _, storageNode := range planet.StorageNodes {
		node, err := satellite.Overlay.Service.Get(ctx, storageNode.ID())
		if assert.NoError(t, err) {
			assert.Equal(t, storageNode.Addr(), node.Address.Address)
		}
	}
}
