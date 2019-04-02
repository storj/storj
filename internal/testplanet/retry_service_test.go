// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"testing"

	"storj.io/storj/bootstrap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestStorageNodeServiceRetry(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Bootstrap: func(index int, config *bootstrap.Config) {
				config.Kademlia.BootstrapAddr = ":9990"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// have all storage nodes start up (without a bootstrap node)
		// confirm that storagenodes can't find anybody
		// after they've all started, start the bootstrap node,
		// then wait for the back off time, then make sure nodes find out about everybody.

	})
}
