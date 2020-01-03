// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bytes"
	"math/rand"
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func testCRUD(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore) {
	items := storage.Items{
		// newItem("0", "", false), //TODO: broken
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

	t.Run("GetAll", func(t *testing.T) {
		subset := items[:len(items)/2]
		keys := subset.GetKeys()
		values, err := store.GetAll(ctx, keys)
		if err != nil {
			t.Fatalf("failed to GetAll %q: %v", keys, err)
		}
		if len(values) != len(keys) {
			t.Fatalf("failed to GetAll %q: got %q", keys, values)
		}
		for i, item := range subset {
			if !bytes.Equal([]byte(values[i]), []byte(item.Value)) {
				t.Fatalf("invalid GetAll %q = %v: got %v", item.Key, item.Value, values[i])
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		for i, item := range items {
			next := items[(i+1)%len(items)]
			err := store.CompareAndSwap(ctx, item.Key, item.Value, next.Value)
			if err != nil {
				t.Fatalf("failed to update %q: %v -> %v: %v", item.Key, item.Value, next.Value, err)
			}
		}

		for i, item := range items {
			next := items[(i+1)%len(items)]
			value, err := store.Get(ctx, item.Key)
			if err != nil {
				t.Fatalf("failed to get updated %q = %v: %v", item.Key, next.Value, err)
			}
			if !bytes.Equal([]byte(value), []byte(next.Value)) {
				t.Fatalf("invalid updated value for %q = %v: got %v", item.Key, next.Value, value)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
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
