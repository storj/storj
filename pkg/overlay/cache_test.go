// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

func testCache(ctx context.Context, t *testing.T, store storage.KeyValueStore, sdb statdb.DB) {
	valid1ID := storj.NodeID{}
	valid2ID := storj.NodeID{}
	missingID := storj.NodeID{}

	_, _ = rand.Read(valid1ID[:])
	_, _ = rand.Read(valid2ID[:])
	_, _ = rand.Read(missingID[:])

	cache := overlay.Cache{DB: store, StatDB: sdb}

	{ // Put
		err := cache.Put(ctx, valid1ID, pb.Node{Id: valid1ID})
		if err != nil {
			t.Fatal(err)
		}

		err = cache.Put(ctx, valid2ID, pb.Node{Id: valid2ID})
		if err != nil {
			t.Fatal(err)
		}
	}

	{ // Get
		_, err := cache.Get(ctx, storj.NodeID{})
		assert.Error(t, err)
		assert.True(t, err == overlay.ErrEmptyNode)

		valid1, err := cache.Get(ctx, valid1ID)
		if assert.NoError(t, err) {
			assert.Equal(t, valid1.Id, valid1ID)
		}

		valid2, err := cache.Get(ctx, valid2ID)
		if assert.NoError(t, err) {
			assert.Equal(t, valid2.Id, valid2ID)
		}

		invalid2, err := cache.Get(ctx, missingID)
		assert.Error(t, err)
		assert.True(t, err == overlay.ErrNodeNotFound)
		assert.Nil(t, invalid2)

		if storeClient, ok := store.(*teststore.Client); ok {
			storeClient.ForceError++
			_, err := cache.Get(ctx, valid1ID)
			assert.Error(t, err)
		}
	}

	{ // GetAll
		nodes, err := cache.GetAll(ctx, storj.NodeIDList{valid2ID, valid1ID, valid2ID})
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid2ID)
		assert.Equal(t, nodes[1].Id, valid1ID)
		assert.Equal(t, nodes[2].Id, valid2ID)

		nodes, err = cache.GetAll(ctx, storj.NodeIDList{valid1ID, missingID})
		assert.NoError(t, err)
		assert.Equal(t, nodes[0].Id, valid1ID)
		assert.Nil(t, nodes[1])

		nodes, err = cache.GetAll(ctx, make(storj.NodeIDList, 2))
		assert.NoError(t, err)
		assert.Nil(t, nodes[0])
		assert.Nil(t, nodes[1])

		_, err = cache.GetAll(ctx, storj.NodeIDList{})
		assert.True(t, overlay.OverlayError.Has(err))

		if storeClient, ok := store.(*teststore.Client); ok {
			storeClient.ForceError++
			_, err := cache.GetAll(ctx, storj.NodeIDList{valid1ID, valid2ID})
			assert.Error(t, err)
		}
	}
}

func TestCache_Masterdb(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)
	planet.Start(ctx)

	satellitedbtest.Run(t, func(t *testing.T, db *satellitedb.DB) {
		testCache(ctx, t, db.OverlayCache(), db.StatDB())
	})
}
