// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/storj/storage"
)

func testPrefix(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("x-a", "a", false),
		newItem("x-b/1", "b/1", false),
		newItem("x-b/2", "b/2", false),
		newItem("x-b/3", "b/3", false),
		newItem("y-c", "c", false),
		newItem("y-c/", "c/", false),
		newItem("y-c//", "c//", false),
		newItem("y-c/1", "c/1", false),
		newItem("y-g", "g", false),
		newItem("y-h", "h", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	t.Run("prefix x dash b slash", func(t *testing.T) {
		check(t, store.IterateAll(storage.Key("x-"), storage.Key("x-b"),
			checkIterator(t, storage.Items{
				newItem("x-b/1", "b/1", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/3", "b/3", false),
			})))
	})

	t.Run("reverse prefix x dash b slash", func(t *testing.T) {
		check(t, store.IterateReverseAll(storage.Key("x-"), storage.Key("x-b/3"),
			checkIterator(t, storage.Items{
				newItem("x-b/3", "b/3", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/1", "b/1", false),
				newItem("x-a", "a", false),
			})))
	})

	t.Run("prefix x dash b slash", func(t *testing.T) {
		check(t, store.Iterate(storage.Key("x-"), storage.Key("x-b"), '/',
			checkIterator(t, storage.Items{
				newItem("x-b/", "", true),
			})))
	})

	t.Run("reverse x dash b slash", func(t *testing.T) {
		check(t, store.IterateReverse(storage.Key("x-"), storage.Key("x-b/2"), '/',
			checkIterator(t, storage.Items{
				newItem("x-b/", "", true),
				newItem("x-a", "a", false),
			})))
	})

	t.Run("prefix y- slash", func(t *testing.T) {
		check(t, store.IterateAll(storage.Key("y-"), nil,
			checkIterator(t, storage.Items{
				newItem("y-c", "c", false),
				newItem("y-c/", "c/", false),
				newItem("y-c//", "c//", false),
				newItem("y-c/1", "c/1", false),
				newItem("y-g", "g", false),
				newItem("y-h", "h", false),
			})))
	})
}
