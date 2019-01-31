// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/bootstrap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

func TestMergePlanets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	alpha, err := testplanet.NewCustom(log.Named("A"), testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 5,
	})
	require.NoError(t, err)

	beta, err := testplanet.NewCustom(log.Named("B"), testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 5,
		Reconfigure: testplanet.Reconfigure{
			Bootstrap: func(planet *testplanet.Planet, index int, config *bootstrap.Config) {
				config.Kademlia.BootstrapAddr = alpha.Bootstrap.Addr()
			},
		},
	})
	require.NoError(t, err)

	defer ctx.Check(alpha.Shutdown)
	defer ctx.Check(beta.Shutdown)

	alpha.Start(ctx)
	beta.Start(ctx)

	// wait until everyone is reachable or fail
	time.Sleep(10 * time.Second)

	satellites := []*satellite.Peer{}
	satellites = append(satellites, alpha.Satellites...)
	satellites = append(satellites, beta.Satellites...)

	storagenodes := []*storagenode.Peer{}
	storagenodes = append(storagenodes, alpha.StorageNodes...)
	storagenodes = append(storagenodes, beta.StorageNodes...)

	for _, satellite := range satellites {
		for _, storagenode := range storagenodes {
			node, err := satellite.Overlay.Service.Get(ctx, storagenode.ID())
			if assert.NoError(t, err) {
				assert.Equal(t, storagenode.Addr(), node.Address.Address)
			}
		}
	}
}
