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
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/errs2"
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
	proxy, err := newBadProxy("127.0.0.1:0", 200*time.Millisecond)
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

	// We set the bad proxy's "target" to the bootstrap node's addr
	// (which was selected when the new custom planet was set up).
	proxy.target = planet.Bootstrap.Addr()

	var group errgroup.Group
	group.Go(func() error { return proxy.run(ctx) })
	defer ctx.Check(proxy.close)
	defer ctx.Check(group.Wait)

	planet.Start(ctx)
	time.Sleep(2 * time.Second)
	defer ctx.Check(planet.Shutdown)
}

type badProxy struct {
	target       string
	dropInterval time.Duration
	listener     net.Listener
}

func newBadProxy(addr string, dropInterval time.Duration) (*badProxy, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &badProxy{
		target:       "",
		dropInterval: dropInterval,
		listener:     listener,
	}, nil
}

func (proxy *badProxy) close() error {
	return proxy.listener.Close()
}

func (proxy *badProxy) run(ctx context.Context) error {
	start := time.Now()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		var connections errs2.Group
		defer func() {
			var errlist errs.Group
			errlist.Add(err)
			errlist.Add(connections.Wait()...)
			err = errlist.Err()
		}()

		for {
			conn, err := proxy.listener.Accept()
			if err != nil {
				return errs.Wrap(errs2.IgnoreCanceled(err))
			}

			if time.Since(start) < proxy.dropInterval {
				if err := conn.Close(); err != nil {
					return errs.Wrap(err)
				}
				continue
			}

			connections.Go(func() error {
				defer func() { err = errs.Combine(err, conn.Close()) }()

				targetConn, err := net.Dial("tcp", proxy.target)
				if err != nil {
					return err
				}
				defer func() { err = errs.Combine(err, targetConn.Close()) }()

				var pipe errs2.Group
				pipe.Go(func() error {
					_, err := io.Copy(targetConn, conn)
					return err
				})
				pipe.Go(func() error {
					_, err := io.Copy(conn, targetConn)
					return err
				})

				return errs.Combine(pipe.Wait()...)
			})
		}
	})
	return errs.Wrap(group.Wait())
}
