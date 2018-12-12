// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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

	peers := []*testplanet.Node{}
	peers = append(peers, planet.Satellites...)
	peers = append(peers, planet.StorageNodes...)

	{ // Ping
		client, err := planet.StorageNodes[0].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		for i := range peers {
			peer := peers[i]
			ctx.Go(func() error {
				pinged, err := client.Ping(ctx, peer.Info)
				var pingErr error
				if !pinged {
					pingErr = errors.New("ping should have succeeded")
				}
				return utils.CombineErrors(pingErr, err)
			})
		}
	}

	{ // Lookup
		client, err := planet.StorageNodes[1].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		for i := range peers {
			peer := peers[i]
			ctx.Go(func() error {
				for _, target := range peers {
					results, err := client.Lookup(ctx, peer.Info, target.Info)
					if err != nil {
						return err
					}

					if containsResult(results, target.ID()) {
						continue
					}

					// with small network we expect to return everything
					if len(results) != planet.Size() {
						return fmt.Errorf("expected %d got %d: %s", planet.Size(), len(results), pb.NodesToIDs(results))
					}

					return nil
				}
				return nil
			})
		}
	}

	{ // Lookup
		client, err := planet.StorageNodes[2].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect)

		for i := range peers {
			peer := peers[i]
			ctx.Go(func() error {
				results, err := client.Lookup(ctx, peer.Info, pb.Node{Id: storj.NodeID{}})
				if err != nil {
					return err
				}

				// with small network we expect to return everything
				if len(results) != planet.Size() {
					return fmt.Errorf("expected %d got %d: %s", planet.Size(), len(results), pb.NodesToIDs(results))
				}

				return nil
			})
		}
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
