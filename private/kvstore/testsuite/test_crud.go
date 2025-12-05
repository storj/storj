// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bytes"
	"math/rand"
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/private/kvstore"
)

func testCRUD(t *testing.T, ctx *testcontext.Context, store kvstore.Store) {
	items := kvstore.Items{
		// newItem("0", "", false), // TODO: broken
		newItem("\x00", "\x00", false),
		newItem("a/b", "\x01\x00", false),
		newItem("a\\b", "\xFF", false),
		newItem("full/path/1", "\x00\xFF\xFF\x00", false),
		newItem("full/path/2", "\x00\xFF\xFF\x01", false),
		newItem("full/path/3", "\x00\xFF\xFF\x02", false),
		newItem("full/path/4", "\x00\xFF\xFF\x03", false),
		newItem("full/path/5", "\x00\xFF\xFF\x04", false),
		newItem("öö", "üü", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(t, ctx, store, items)

	t.Run("Put", func(t *testing.T) {
		for _, item := range items {
			err := store.Put(ctx, item.Key, item.Value)
			if err != nil {
				t.Fatalf("failed to put %q = %v: %v", item.Key, item.Value, err)
			}
		}
	})

	rand.Shuffle(len(items), items.Swap)

	t.Run("Get", func(t *testing.T) {
		for _, item := range items {
			value, err := store.Get(ctx, item.Key)
			if err != nil {
				t.Fatalf("failed to get %q = %v: %v", item.Key, item.Value, err)
			}
			if !bytes.Equal([]byte(value), []byte(item.Value)) {
				t.Fatalf("invalid value for %q = %v: got %v", item.Key, item.Value, value)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		for _, item := range items {
			_, err := store.Get(ctx, item.Key)
			if err != nil {
				t.Fatalf("failed to get %v", item.Key)
			}
		}

		// individual deletes
		for _, item := range items {
			err := store.Delete(ctx, item.Key)
			if err != nil {
				t.Fatalf("failed to delete %v: %v", item.Key, err)
			}
		}

		for _, item := range items {
			value, err := store.Get(ctx, item.Key)
			if err == nil {
				t.Fatalf("got deleted value %q = %v", item.Key, value)
			}
		}
	})
}
