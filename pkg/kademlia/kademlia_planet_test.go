// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode"
)

func TestFetchPeerIdentity(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		peerID, err := planet.StorageNodes[0].Kademlia.Service.FetchPeerIdentity(ctx, sat.ID())
		require.NoError(t, err)
		require.Equal(t, sat.ID(), peerID.ID)
		require.True(t, sat.Identity.Leaf.Equal(peerID.Leaf))
		require.True(t, sat.Identity.CA.Equal(peerID.CA))
	})
}

func TestRequestInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		info, err := planet.Satellites[0].Kademlia.Service.FetchInfo(ctx, node.Local())
		require.NoError(t, err)
		require.Equal(t, node.Local().Type, info.GetType())
		require.Equal(t, node.Local().Metadata.GetEmail(), info.GetOperator().GetEmail())
		require.Equal(t, node.Local().Metadata.GetWallet(), info.GetOperator().GetWallet())
		require.Equal(t, node.Local().Restrictions.GetFreeDisk(), info.GetCapacity().GetFreeDisk())
		require.Equal(t, node.Local().Restrictions.GetFreeBandwidth(), info.GetCapacity().GetFreeBandwidth())
	})
}

func TestPingTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		self := planet.StorageNodes[0]
		routingTable := self.Kademlia.RoutingTable

		tlsOpts, err := tlsopts.NewOptions(self.Identity, tlsopts.Config{})
		require.NoError(t, err)

		self.Transport = transport.NewClientWithTimeout(tlsOpts, 1*time.Millisecond)

		network := &transport.SimulatedNetwork{
			DialLatency:    300 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(self.Transport)
		require.NotNil(t, slowClient)

		node := pb.Node{
			Id: self.ID(),
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
			},
		}

		newService, err := kademlia.NewService(zaptest.NewLogger(t), node, slowClient, routingTable, kademlia.Config{})
		require.NoError(t, err)

		target := pb.Node{
			Id: planet.StorageNodes[2].ID(),
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   planet.StorageNodes[2].Addr(),
			},
		}

		_, err = newService.Ping(ctx, target)
		require.Error(t, err, context.DeadlineExceeded)
		require.True(t, kademlia.NodeErr.Has(err) && transport.Error.Has(err))

	})
}

func TestBootstrapBackoff(t *testing.T) {
	done := make(chan bool)
	go badBootstrapProxy(done)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Kademlia.BootstrapAddr = ":9999"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

	})
}

func badBootstrapProxy(done chan bool) (err error) {
	start := time.Now()

	l, err := net.Listen("tcp", ":9999")
	if err != nil {
		return err
	}

	defer func() {
		done <- true
		l.Close()
	}()

	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			if time.Since(start) < 1*time.Second {
				fmt.Println("dropped conn")
				c.Close()
				return
			}
			c2, err := net.Dial("tcp", "127.0.0.1:9990")
			if err != nil {
				c.Close()
				return
			}
			fmt.Println("successful conn")
			go io.Copy(c, c2)
			io.Copy(c2, c)
		}()
	}
}
