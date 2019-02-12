// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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

	client := planet.StorageNodes[0].Transport

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
			conn, err := client.DialNode(timedCtx, target)
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
		})
		cancel()

		assert.NoError(t, err)
		assert.NotNil(t, conn)

		assert.NoError(t, conn.Close())
	}

	{ // DialAddress with valid address
		timedCtx, cancel := context.WithTimeout(ctx, time.Second)
		conn, err := client.DialAddress(timedCtx, planet.StorageNodes[1].Addr())
		cancel()

		assert.NoError(t, err)
		assert.NotNil(t, conn)

		assert.NoError(t, conn.Close())
	}
}
