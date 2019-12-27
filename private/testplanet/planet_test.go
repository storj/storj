// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestBasic(t *testing.T) {
	for _, version := range storj.IDVersions {
		version := version
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 2, StorageNodeCount: 4, UplinkCount: 1,
			IdentityVersion: &version,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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
					conn, err := sn.Dialer.DialNode(ctx, &satellite)
					require.NoError(t, err)
					defer ctx.Check(conn.Close)
					_, err = pb.NewDRPCNodeClient(conn.Raw()).CheckIn(ctx, &pb.CheckInRequest{
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
		})
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
