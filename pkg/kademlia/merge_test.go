// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/bootstrap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
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

	for _, node := range beta.StorageNodes {
		for _, target := range alpha.StorageNodes {
			// TODO: test whether is in routing table
			_, _ = node, target
		}
	}
}
