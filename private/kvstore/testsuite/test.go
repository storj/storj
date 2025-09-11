// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"strconv"
	"testing"

	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/private/kvstore"
)

// RunTests runs common kvstore.Store tests.
func RunTests(t *testing.T, store kvstore.Store) {
	t.Run("CRUD", func(t *testing.T) {
		ctx := testcontext.New(t)
		testCRUD(t, ctx, store)
	})
	t.Run("Constraints", func(t *testing.T) {
		ctx := testcontext.New(t)
		testConstraints(t, ctx, store)
	})
	t.Run("Range", func(t *testing.T) {
		ctx := testcontext.New(t)
		testRange(t, ctx, store)
	})
	t.Run("Parallel", func(t *testing.T) {
		ctx := testcontext.New(t)
		testParallel(t, ctx, store)
	})
}

func testConstraints(t *testing.T, ctx *testcontext.Context, store kvstore.Store) {
	var items kvstore.Items
	for i := 0; i < 10; i++ {
		items = append(items, kvstore.Item{
			Key:   kvstore.Key("test-" + strconv.Itoa(i)),
			Value: kvstore.Value("xyz"),
		})
	}

	var group errgroup.Group
	for _, item := range items {
		key := item.Key
		value := item.Value
		group.Go(func() error {
			return store.Put(ctx, key, value)
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	defer cleanupItems(t, ctx, store, items)

	t.Run("Put Empty", func(t *testing.T) {
		var key kvstore.Key
		var val kvstore.Value
		defer func() { _ = store.Delete(ctx, key) }()

		err := store.Put(ctx, key, val)
		if err == nil {
			t.Fatal("putting empty key should fail")
		}
	})
}
