// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademliaclient_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/kademlia/kademliaclient"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

func TestDialer(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 3)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	expectedKademliaEntries := len(planet.Satellites) + len(planet.StorageNodes)

	// TODO: also use satellites
	peers := planet.StorageNodes

	{ // PingNode: storage node pings all other storage nodes
		self := planet.StorageNodes[0]

		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), self.Transport)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		for _, peer := range peers {
			peer := peer
			group.Go(func() error {
				pinged, err := dialer.PingNode(ctx, peer.Local().Node)
				var pingErr error
				if !pinged {
					pingErr = fmt.Errorf("ping to %s should have succeeded", peer.ID())
				}
				return errs.Combine(pingErr, err)
			})
		}
	}

	{ // FetchPeerIdentity: storage node fetches identity of the satellite
		self := planet.StorageNodes[0]

		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), self.Transport)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		group.Go(func() error {
			ident, err := dialer.FetchPeerIdentity(ctx, planet.Satellites[0].Local().Node)
			if err != nil {
				return fmt.Errorf("failed to fetch peer identity")
			}
			if ident.ID != planet.Satellites[0].Local().Id {
				return fmt.Errorf("fetched wrong identity")
			}

			ident, err = dialer.FetchPeerIdentityUnverified(ctx, planet.Satellites[0].Addr())
			if err != nil {
				return fmt.Errorf("failed to fetch peer identity from address")
			}
			if ident.ID != planet.Satellites[0].Local().Id {
				return fmt.Errorf("fetched wrong identity from address")
			}

			return nil
		})
	}

	{ // Lookup: storage node query every node for everyone elese
		self := planet.StorageNodes[1]
		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), self.Transport)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		for _, peer := range peers {
			peer := peer
			group.Go(func() error {
				for _, target := range peers {
					errTag := fmt.Errorf("lookup peer:%s target:%s", peer.ID(), target.ID())

					selfnode := self.Local().Node
					results, err := dialer.Lookup(ctx, &selfnode, peer.Local().Node, target.Local().Node.Id, self.Kademlia.RoutingTable.K())
					if err != nil {
						return errs.Combine(errTag, err)
					}

					if containsResult(results, target.ID()) {
						continue
					}

					// with small network we expect to return everything
					if len(results) != expectedKademliaEntries {
						return errs.Combine(errTag, fmt.Errorf("expected %d got %d: %s", expectedKademliaEntries, len(results), pb.NodesToIDs(results)))
					}
					return nil
				}
				return nil
			})
		}
	}

	{ // Lookup: storage node queries every node for missing storj.NodeID{} and storj.NodeID{255}
		self := planet.StorageNodes[2]
		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), self.Transport)
		defer ctx.Check(dialer.Close)

		targets := []storj.NodeID{
			{},    // empty target
			{255}, // non-empty
		}

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		for _, target := range targets {
			target := target
			for _, peer := range peers {
				peer := peer
				group.Go(func() error {
					errTag := fmt.Errorf("invalid lookup peer:%s target:%s", peer.ID(), target)

					selfnode := self.Local().Node
					results, err := dialer.Lookup(ctx, &selfnode, peer.Local().Node, target, self.Kademlia.RoutingTable.K())
					if err != nil {
						return errs.Combine(errTag, err)
					}

					// with small network we expect to return everything
					if len(results) != expectedKademliaEntries {
						return errs.Combine(errTag, fmt.Errorf("expected %d got %d: %s", expectedKademliaEntries, len(results), pb.NodesToIDs(results)))
					}
					return nil
				})
			}
		}
	}
}

func TestSlowDialerHasTimeout(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// TODO: also use satellites
	peers := planet.StorageNodes

	func() { // PingNode
		self := planet.StorageNodes[0]

		tlsOpts, err := tlsopts.NewOptions(self.Identity, tlsopts.Config{})
		require.NoError(t, err)

		self.Transport = transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
			Dial: 20 * time.Millisecond,
		})

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(self.Transport)
		require.NotNil(t, slowClient)

		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), slowClient)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		for _, peer := range peers {
			peer := peer
			group.Go(func() error {
				_, err := dialer.PingNode(ctx, peer.Local().Node)
				if !transport.Error.Has(err) || errs.Unwrap(err) != context.DeadlineExceeded {
					return errs.New("invalid error: %v", err)
				}
				return nil
			})
		}
	}()

	func() { // FetchPeerIdentity
		self := planet.StorageNodes[1]

		tlsOpts, err := tlsopts.NewOptions(self.Identity, tlsopts.Config{})
		require.NoError(t, err)

		self.Transport = transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
			Dial: 20 * time.Millisecond,
		})

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(self.Transport)
		require.NotNil(t, slowClient)

		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), slowClient)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		group.Go(func() error {
			_, err := dialer.FetchPeerIdentity(ctx, planet.Satellites[0].Local().Node)
			if !transport.Error.Has(err) || errs.Unwrap(err) != context.DeadlineExceeded {
				return errs.New("invalid error: %v", err)
			}
			_, err = dialer.FetchPeerIdentityUnverified(ctx, planet.Satellites[0].Addr())
			if !transport.Error.Has(err) || errs.Unwrap(err) != context.DeadlineExceeded {
				return errs.New("invalid error: %v", err)
			}
			return nil
		})
	}()

	func() { // Lookup
		self := planet.StorageNodes[2]

		tlsOpts, err := tlsopts.NewOptions(self.Identity, tlsopts.Config{})
		require.NoError(t, err)

		self.Transport = transport.NewClientWithTimeouts(tlsOpts, transport.Timeouts{
			Dial: 20 * time.Millisecond,
		})

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(self.Transport)
		require.NotNil(t, slowClient)

		dialer := kademliaclient.NewDialer(zaptest.NewLogger(t), slowClient)
		defer ctx.Check(dialer.Close)

		var group errgroup.Group
		defer ctx.Check(group.Wait)

		for _, peer := range peers {
			peer := peer
			group.Go(func() error {
				for _, target := range peers {
					selfnode := self.Local().Node
					_, err := dialer.Lookup(ctx, &selfnode, peer.Local().Node, target.Local().Node.Id, self.Kademlia.RoutingTable.K())
					if !transport.Error.Has(err) || errs.Unwrap(err) != context.DeadlineExceeded {
						return errs.New("invalid error: %v (peer:%s target:%s)", err, peer.ID(), target.ID())
					}
					return nil
				}
				return nil
			})
		}
	}()
}

func containsResult(nodes []*pb.Node, target storj.NodeID) bool {
	for _, node := range nodes {
		if node.Id == target {
			return true
		}
	}
	return false
}
