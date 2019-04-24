// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
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
	// This should test that the Bootstrap function will retry a connection
	// if it initially fails.
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
				config.Kademlia.BootstrapBackoffBase = 100 * time.Millisecond
				config.Kademlia.BootstrapBackoffMax = 3 * time.Second
			},
		},
	})
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	// We set the bad proxy's "target" to the bootstrap node's addr
	// (which was selected when the new custom planet was set up).
	proxy.target = planet.Bootstrap.Addr()

	droppedConnInterval := 200 * time.Millisecond
	go func() {
		// This starts the unreliable proxy server and sets it to
		// drop connections for the first 500 milliseconds that it's up.
		err := proxy.run(droppedConnInterval)
		require.NoError(t, err)
	}()
	defer func() {
		err := proxy.close()
		// Expect a group of errors such as "use of closed network connection"
		// or "connection reset by peer" or "broken pipe" since we're closing
		// the storage nodes' connections inside proxy.run, and they will
		// attempt to contact each other between the storageNodeConn.Close()
		// and bootstrapNodeConn.Close() calls.
		require.Error(t, err)
	}()

	planet.Start(ctx)
}

type badProxy struct {
	closed   uint32 // using sync/atomic package to do an atomic set/load of closed (as bool flag)
	listener net.Listener
	target   string
	errors   errs.Group
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

func (proxy *badProxy) close() error {
	atomic.StoreUint32(&proxy.closed, 1)
	proxy.errors.Add(proxy.listener.Close())
	return proxy.errors.Err()
}

func (proxy *badProxy) run(droppedConnInterval time.Duration) (err error) {
	start := time.Now()

	defer func() {
		err := proxy.listener.Close()
		if atomic.LoadUint32(&proxy.closed) == 0 {
			proxy.errors.Add(err)
		}
	}()

	for {
		storageNodeConn, err := proxy.listener.Accept()
		if err != nil {
			if atomic.LoadUint32(&proxy.closed) > 0 {
				return nil
			}
			return errs.Wrap(err)
		}
		go func() {
			// Within the droppedConnInterval duration,
			// the proxy will drop conns.
			if time.Since(start) < droppedConnInterval {
				err := storageNodeConn.Close()
				if err != nil {
					proxy.errors.Add(err)
				}
				return
			}
			bootstrapNodeConn, err := net.Dial("tcp", proxy.target)
			if err != nil {
				proxy.errors.Add(err)
				err = storageNodeConn.Close()
				if err != nil {
					proxy.errors.Add(err)
				}
				return
			}
			var closeOnce sync.Once
			closer := func() {
				proxy.errors.Add(storageNodeConn.Close())
				proxy.errors.Add(bootstrapNodeConn.Close())
			}
			// Here the proxy copies the data from
			// Peer A (storage node or satellite)
			// to Peer B (bootstrap node).
			go func() {
				defer closeOnce.Do(closer)
				_, err := io.Copy(storageNodeConn, bootstrapNodeConn)
				if err != nil {
					proxy.errors.Add(err)
				}
			}()
			// Starts copy loops concurrently where
			// the first to exit closes the connections.
			defer closeOnce.Do(closer)
			_, err = io.Copy(bootstrapNodeConn, storageNodeConn)
			if err != nil {
				proxy.errors.Add(err)
			}
		}()
	}
}
