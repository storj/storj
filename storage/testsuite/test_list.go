package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/storj/storage"
)

func testList(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("path/0", "\x00\xFF\x00", false),
		newItem("path/1", "\x01\xFF\x01", false),
		newItem("path/2", "\x02\xFF\x02", false),
		newItem("path/3", "\x03\xFF\x03", false),
		newItem("path/4", "\x04\xFF\x04", false),
		newItem("path/5", "\x05\xFF\x05", false),
	}
	rand.Shuffle(len(items), items.Swap)

	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	t.Run("Without Key", func(t *testing.T) {
		keys, err := store.List(storage.Key(""), storage.Limit(3))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("invalid number of keys %v: %v", len(keys), err)
		}
		testKeysSorted(t, keys)
	})

	t.Run("Without Key, Limit 0", func(t *testing.T) {
		t.Skip("unimplemented")
		keys, err := store.List(storage.Key(""), storage.Limit(0))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != len(items) {
			t.Fatalf("invalid number of keys %v: %v", len(keys), err)
		}
		testKeysSorted(t, keys)
	})

	t.Run("With Key", func(t *testing.T) {
		keys, err := store.List(storage.Key("path/2"), storage.Limit(3))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("invalid number of keys %v: %v", len(keys), err)
		}
		testKeysSorted(t, keys)
	})

	t.Run("Without Key 100", func(t *testing.T) {
		keys, err := store.List(storage.Key(""), storage.Limit(100))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != len(items) {
			t.Fatalf("invalid number of keys %v expected %v: %q", len(keys), len(items), keys)
		}
		testKeysSorted(t, keys)
	})
}
