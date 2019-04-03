// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	// "storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode"
)

func TestStorageNodeServiceRetry(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Retry.BaseWait = 1 * time.Millisecond
				config.Retry.MaxWait = 3 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		err := planet.StopPeer(planet.StorageNodes[0])
		require.NoError(t, err)

		err = planet.StopPeer(planet.Bootstrap)
		require.NoError(t, err)

		err = planet.StorageNodes[0].Kademlia.Service.Close()
		require.NoError(t, err)

		err = planet.StorageNodes[0].Storage2.Sender.Close()
		require.NoError(t, err)

		err = planet.StorageNodes[0].Server.Close()
		require.NoError(t, err)

		err = planet.StopPeer(planet.Satellites[0])
		require.NoError(t, err)

	})
}
