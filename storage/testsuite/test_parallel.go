// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bytes"
	"math/rand"
	"strconv"
	"testing"

	"storj.io/storj/storage"
)

func testParallel(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("a", "1", false),
		newItem("b", "2", false),
		newItem("c", "3", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(store, items)

	for i := range items {
		item := items[i]
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			// Put
			err := store.Put(item.Key, item.Value)
			if err != nil {
				t.Fatalf("failed to put %q = %v: %v", item.Key, item.Value, err)
			}

			// Get
			value, err := store.Get(item.Key)
			if err != nil {
				t.Fatalf("failed to get %q = %v: %v", item.Key, item.Value, err)
			}
			if !bytes.Equal([]byte(value), []byte(item.Value)) {
				t.Fatalf("invalid value for %q = %v: got %v", item.Key, item.Value, value)
			}

			// GetAll
			values, err := store.GetAll([]storage.Key{item.Key})
			if len(values) != 1 {
				t.Fatalf("failed to GetAll: %v", err)
			}

			if !bytes.Equal([]byte(values[0]), []byte(item.Value)) {
				t.Fatalf("invalid GetAll %q = %v: got %v", item.Key, item.Value, values[i])
			}

			// Update value
			nextValue := storage.Value(string(item.Value) + "X")
			err = store.Put(item.Key, nextValue)
			if err != nil {
				t.Fatalf("failed to update %q = %v: %v", item.Key, nextValue, err)
			}

			value, err = store.Get(item.Key)
			if err != nil {
				t.Fatalf("failed to get %q = %v: %v", item.Key, nextValue, err)
			}
			if !bytes.Equal([]byte(value), []byte(nextValue)) {
				t.Fatalf("invalid updated value for %q = %v: got %v", item.Key, nextValue, value)
			}

			err = store.Delete(item.Key)
			if err != nil {
				t.Fatalf("failed to delete %v: %v", item.Key, err)
			}
		})
	}
}
