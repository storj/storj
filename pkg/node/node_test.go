// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/utils"
)

var (
	ctx     = context.Background()
	helloID = teststorj.NodeIDFromString("hello")
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

	{ // Ping
		client, err := planet.StorageNodes[0].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect())

		var group errgroup.Group

		for i := range planet.StorageNodes {
			sat := planet.StorageNodes[i]
			group.Go(func() error {
				pinged, err := client.Ping(ctx, sat.Info)
				var pingErr error
				if !pinged {
					pingErr = errors.New("ping should have succeeded")
				}
				return utils.CombineErrors(pingErr, err)
			})
		}

		assert.NoError(t, group.Wait())
	}

	{ // Lookup
		client, err := planet.StorageNodes[1].NewNodeClient()
		assert.NoError(t, err)
		defer ctx.Check(client.Disconnect())

		var group errgroup.Group

		for i := range planet.StorageNodes {
			sat := planet.StorageNodes[i]
			group.Go(func() error {
				for _, target := range planet.StorageNodes {
					results, err := client.Lookup(ctx, sat.Info, target.Info)
					if err != nil {
						return err
					}

					if len(results) != planet.NetworkSize() {
						return fmt.Errorf("expected %d got %d", planet.NetworkSize())
					}

					return nil
				}
				return nil
			})
		}

		assert.NoError(t, group.Wait())
	}
}
