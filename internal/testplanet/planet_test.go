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
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	test := func(version storj.IDVersion) {
		planet, err := testplanet.NewWithIdentityVersion(t, &version, 2, 4, 1)
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

		for _, sat := range planet.Satellites {
			satellite := sat.Local().Node
			for _, sn := range planet.StorageNodes {
				node := sn.Local()
				conn, err := sn.Transport.DialNode(ctx, &satellite)
				require.NoError(t, err)
				_, err = pb.NewNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
					Address:  node.GetAddress().GetAddress(),
					Version:  &node.Version,
					Capacity: &node.Capacity,
					Operator: &node.Operator,
				})
				require.NoError(t, err)
			}

		}
		// wait a bit to see whether some failures occur
		time.Sleep(time.Second)
	}

	for _, version := range storj.IDVersions {
		test(version)
	}
}

// test that nodes get put into each satellite's overlay cache
func TestContact(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 5, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite0 := planet.Satellites[0]
		satellite1 := planet.Satellites[1]

		for _, n := range planet.StorageNodes {
			_, err := satellite0.Overlay.Service.Get(ctx, n.ID())
			require.NoError(t, err)
			_, err = satellite1.Overlay.Service.Get(ctx, n.ID())
			require.NoError(t, err)
		}
	})
}

func BenchmarkCreate(b *testing.B) {
	storageNodes := []int{4, 10, 100}
	for _, count := range storageNodes {
		storageNodeCount := count
		b.Run(strconv.Itoa(storageNodeCount), func(b *testing.B) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				planet, err := testplanet.New(nil, 1, storageNodeCount, 1)
				require.NoError(b, err)

				planet.Start(ctx)

				err = planet.Shutdown()
				require.NoError(b, err)
			}
		})
	}
}
