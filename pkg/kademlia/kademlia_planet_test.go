// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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
		info, err := planet.Satellites[0].Kademlia.Service.FetchInfo(ctx, node.Local().Node)
		require.NoError(t, err)
		require.Equal(t, node.Local().Type, info.GetType())
		require.Empty(t, cmp.Diff(node.Local().Operator, *info.GetOperator(), cmp.Comparer(pb.Equal)))
		require.Empty(t, cmp.Diff(node.Local().Capacity, *info.GetCapacity(), cmp.Comparer(pb.Equal)))
		require.Empty(t, cmp.Diff(node.Local().Version, *info.GetVersion(), cmp.Comparer(pb.Equal)))
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

		newService, err := kademlia.NewService(zaptest.NewLogger(t), slowClient, routingTable, kademlia.Config{})
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

func TestBootstrapBackoffReconnect(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// This sets up an unreliable proxy server which will receive conns from
	// storage nodes and the satellite, but doesn't connect them with
	// the bootstrap node (proxy.target) until the droppedConnInterval has passed.
	// The reason for using this bad or unreliable proxy is to accurately test that
	// the Bootstrap function's new backoff functionality will retry a connection
	// if it initially failed.
	proxy, err := newBadProxy(zaptest.NewLogger(t), "127.0.0.1:0")
	require.NoError(t, err)

	planet, err := testplanet.NewCustom(zaptest.NewLogger(t), testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Kademlia.BootstrapAddr = proxy.listener.Addr().String()
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Kademlia.BootstrapAddr = proxy.listener.Addr().String()
				config.Kademlia.BootstrapBackoffBase = 200 * time.Millisecond
				config.Kademlia.BootstrapBackoffMax = 3 * time.Second
			},
		},
	})
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	// We set the bad proxy's "target" to the bootstrap node's addr
	// (which was selected when the new custom planet was set up).
	proxy.target = planet.Bootstrap.Addr()

	done := make(chan bool)

	droppedConnInterval := 500 * time.Millisecond
	go func() {
		// This starts the unreliable proxy server and sets it to
		// drop connections for the first 500 milliseconds that it's up.
		err := proxy.run(done, droppedConnInterval)
		require.NoError(t, err)
	}()
	defer func() {
		err := proxy.close()
		require.NoError(t, err)
	}()

	planet.Start(ctx)
}

type badProxy struct {
	log      *zap.Logger
	listener net.Listener
	target   string
}

func newBadProxy(log *zap.Logger, addr string) (*badProxy, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &badProxy{
		log:      log,
		listener: l,
	}, nil
}

func (proxy *badProxy) close() error {
	return proxy.listener.Close()
}

func (proxy *badProxy) run(done chan bool, droppedConnInterval time.Duration) (err error) {
	start := time.Now()

	defer func() {
		done <- true
		err := proxy.listener.Close()
		proxy.log.Error("bad proxy", zap.Error(err))
	}()

	for {
		c, err := proxy.listener.Accept()
		if err != nil {
			return err
		}
		go func() {
			// Within the droppedConnInterval duration,
			// the proxy will drop conns.
			if time.Since(start) < droppedConnInterval {
				err := c.Close()
				if err != nil {
					proxy.log.Error("bad proxy", zap.Error(err))
				}
				return
			}
			c2, err := net.Dial("tcp", proxy.target)
			if err != nil {
				proxy.log.Error("bad proxy", zap.Error(err))
				err = c.Close()
				if err != nil {
					proxy.log.Error("bad proxy", zap.Error(err))
				}
				return
			}
			// Simulating a successful connection,
			// here the proxy correctly copies the
			// data from Peer A (storage node or satellite)
			// to Peer B (bootstrap node).
			go func() {
				_, err := io.Copy(c, c2)
				if err != nil {
					proxy.log.Error("bad proxy", zap.Error(err))
				}
			}()
			_, err = io.Copy(c2, c)
			if err != nil {
				proxy.log.Error("bad proxy", zap.Error(err))
			}
		}()
	}
}
