// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
)

func TestClient(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	time.Sleep(2 * time.Second)

	peers := []*testplanet.Node{}
	peers = append(peers, planet.Satellites...)
	peers = append(peers, planet.StorageNodes...)

	{ // Ping
		client, err := planet.StorageNodes[0].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		var group errgroup.Group

		for i := range peers {
			peer := peers[i]
			group.Go(func() error {
				pinged, err := client.Ping(ctx, peer.Info)
				var pingErr error
				if !pinged {
					pingErr = fmt.Errorf("ping to %s should have succeeded", peer.ID())
				}
				return utils.CombineErrors(pingErr, err)
			})
		}

		defer ctx.Check(group.Wait)
	}

	{ // Lookup
		client, err := planet.StorageNodes[1].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		var group errgroup.Group

		for i := range peers {
			peer := peers[i]
			group.Go(func() error {
				for _, target := range peers {
					errTag := fmt.Errorf("lookup peer:%s target:%s\n", peer.ID(), target.ID())

					results, err := client.Lookup(ctx, peer.Info, target.Info)
					if err != nil {
						return utils.CombineErrors(errTag, err)
					}

					if containsResult(results, target.ID()) {
						continue
					}

					// with small network we expect to return everything
					if len(results) != planet.Size() {
						return utils.CombineErrors(errTag, fmt.Errorf("expected %d got %d: %s", planet.Size(), len(results), pb.NodesToIDs(results)))
					}

					return nil
				}
				return nil
			})
		}

		defer ctx.Check(group.Wait)
	}

	{ // Lookup
		client, err := planet.StorageNodes[2].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		targets := []storj.NodeID{
			{},    // empty target
			{255}, // non-empty
		}

		var group errgroup.Group

		for i := range targets {
			target := targets[i]
			for i := range peers {
				peer := peers[i]
				group.Go(func() error {
					errTag := fmt.Errorf("invalid lookup peer:%s target:%s", peer.ID(), target)

					results, err := client.Lookup(ctx, peer.Info, pb.Node{Id: target})
					if err != nil {
						return utils.CombineErrors(errTag, err)
					}

					// with small network we expect to return everything
					if len(results) != planet.Size() {
						return utils.CombineErrors(errTag, fmt.Errorf("expected %d got %d: %s", planet.Size(), len(results), pb.NodesToIDs(results)))
					}

					return nil
				})
			}
		}

		defer ctx.Check(group.Wait)
	}
}

func containsResult(nodes []*pb.Node, target storj.NodeID) bool {
	for _, node := range nodes {
		if node.Id == target {
			return true
		}
	}
	return false
}
