// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite"
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	proxy, err := newBadProxy("127.0.0.1:0")
	require.NoError(t, err)

	planet, err := testplanet.NewCustom(zaptest.NewLogger(t), testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Kademlia.BootstrapAddr = proxy.listener.Addr().String()
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Kademlia.BootstrapAddr = proxy.listener.Addr().String()
				config.Kademlia.BootstrapBackoffBase = 1 * time.Second
				config.Kademlia.BootstrapBackoffMax = 10 * time.Second
			},
		},
	})
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	proxy.target = planet.Bootstrap.Addr()

	done := make(chan bool)
	go proxy.start(done)

	planet.Start(ctx)
}

type badProxy struct {
	listener net.Listener
	target   string
}

func newBadProxy(addr string) (*badProxy, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &badProxy{
		listener: l,
	}, nil
}

func (proxy *badProxy) start(done chan bool) (err error) {
	start := time.Now()
	defer func() {
		done <- true
		proxy.listener.Close()
	}()
	for {
		c, err := proxy.listener.Accept()
		if err != nil {
			return err
		}
		go func() {
			if time.Since(start) < 1*time.Second {
				c.Close()
				return
			}
			c2, err := net.Dial("tcp", proxy.target)
			if err != nil {
				c.Close()
				return
			}
			go io.Copy(c, c2)
			io.Copy(c2, c)
		}()
	}
}
