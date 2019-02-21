// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 2, 4, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, satellite := range planet.Satellites {
		t.Log("SATELLITE", satellite.ID(), satellite.Addr())
	}
	for _, storageNode := range planet.StorageNodes {
		t.Log("STORAGE", storageNode.ID(), storageNode.Addr())
	}
	for _, uplink := range planet.Uplinks {
		t.Log("UPLINK", uplink.ID(), uplink.Addr())
	}

	// Example of using pointer db
	_, err = planet.Uplinks[0].DialPointerDB(planet.Satellites[0], "apikey")
	require.NoError(t, err)

	// ping a satellite
	_, err = planet.StorageNodes[0].Kademlia.Service.Ping(ctx, planet.Satellites[0].Local())
	require.NoError(t, err)

	// ping a storage node
	_, err = planet.StorageNodes[0].Kademlia.Service.Ping(ctx, planet.StorageNodes[1].Local())
	require.NoError(t, err)

	err = planet.StopPeer(planet.StorageNodes[1])
	require.NoError(t, err)

	// ping a stopped storage node
	_, err = planet.StorageNodes[0].Kademlia.Service.Ping(ctx, planet.StorageNodes[1].Local())
	require.Error(t, err)

	// wait a bit to see whether some failures occur
	time.Sleep(time.Second)
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				planet, err := testplanet.New(nil, 1, count, 1)
				require.NoError(b, err)

				planet.Start(ctx)

				err = planet.Shutdown()
				require.NoError(b, err)
			}
		})
	}
}
