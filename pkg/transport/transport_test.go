// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package transport_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestDialNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 0, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	client := transport.NewClient(planet.StorageNodes[0].Identity)

	{ // DialNode with invalid targets
		targets := []*pb.Node{
			{
				Id:      storj.NodeID{},
				Address: nil,
				Type:    pb.NodeType_STORAGE,
			},
			{
				Id: storj.NodeID{},
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
				},
				Type: pb.NodeType_STORAGE,
			},
			{
				Id: storj.NodeID{123},
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   "127.0.0.1:100",
				},
				Type: pb.NodeType_STORAGE,
			},
		}

		for _, target := range targets {
			tag := fmt.Sprintf("%+v", target)

			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := client.DialNode(timedCtx, target, grpc.WithBlock())
			cancel()
			assert.Error(t, err, tag)
			assert.Nil(t, conn, tag)
		}
	}

	{ // DialNode with valid target
		timedCtx, cancel := context.WithTimeout(ctx, time.Second)
		conn, err := client.DialNode(timedCtx, &pb.Node{
			Id: planet.StorageNodes[1].ID(),
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   planet.StorageNodes[1].Addr(),
			},
			Type: pb.NodeType_STORAGE,
		}, grpc.WithBlock())
		cancel()

		assert.NoError(t, err)
		assert.NotNil(t, conn)

		assert.NoError(t, conn.Close())
	}

	{ // DialAddress with valid address
		timedCtx, cancel := context.WithTimeout(ctx, time.Second)
		conn, err := client.DialAddress(timedCtx, planet.StorageNodes[1].Addr(), grpc.WithBlock())
		cancel()

		assert.NoError(t, err)
		assert.NotNil(t, conn)

		assert.NoError(t, conn.Close())
	}
}

func testCache(ctx context.Context, t *testing.T, store storage.KeyValueStore, sdb statdb.DB, planet *testplanet.Planet) {
	cache := overlay.NewCache(store, sdb)

	{ // test init with observers
		client := transport.NewClient(planet.StorageNodes[0].Identity, cache)
		assert.NotNil(t, client.Observers())
		assert.NotNil(t, client.Observers()[0])
	}

	{ // test AddObserver
		tester := &Tester{}
		client := transport.NewClient(planet.StorageNodes[0].Identity, cache)
		assert.NotNil(t, client.Observers())
		assert.Equal(t, len(client.Observers()), 1)
		client.AddObserver(tester)
		assert.Equal(t, len(client.Observers()), 2)
	}
}

func TestObservers(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)
	planet.Start(ctx)

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		testCache(ctx, t, db.OverlayCache(), db.StatDB(), planet)
	})
}

type Tester struct{}

func (tester *Tester) ConnSuccess(ctx context.Context, node *pb.Node) {}

func (tester *Tester) ConnFailure(ctx context.Context, node *pb.Node, err error) {}
