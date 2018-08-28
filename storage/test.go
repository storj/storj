package storage

import (
	"bytes"
	"math/rand"
	"testing"
)

func RunTests(t *testing.T, store KeyValueStore) {
	tests := []struct {
		name string
		test func(*testing.T, KeyValueStore)
	}{
		{"CRUD", testCRUD},
		{"List", testList},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.test(t, store)
		})
	}
}

func newItem(key, value string) ListItem {
	return ListItem{
		Key:   Key(key),
		Value: Value(value),
	}
}

func testCRUD(t *testing.T, store KeyValueStore) {
	items := Items{
		// newItem("0", ""), //TODO: broken
		newItem("\x00", "\x00"),
		newItem("a/b", "\x01\x00"),
		newItem("a\\b", "\xFF"),
		newItem("full/path/1", "\x00\xFF\xFF\x00"),
		newItem("full/path/2", "\x00\xFF\xFF\x01"),
		newItem("full/path/3", "\x00\xFF\xFF\x02"),
		newItem("full/path/4", "\x00\xFF\xFF\x03"),
		newItem("full/path/5", "\x00\xFF\xFF\x04"),
		newItem("öö", "üü"),
	}
	rand.Shuffle(len(items), items.Swap)

	defer cleanupItems(t, store, items)

	// Put
	for _, item := range items {
		err := store.Put(item.Key, item.Value)
		if err != nil {
			t.Fatalf("failed to put %q = %v: %v", item.Key, item.Value, err)
		}
	}

	rand.Shuffle(len(items), items.Swap)

	// Get
	for _, item := range items {
		value, err := store.Get(item.Key)
		if err != nil {
			t.Fatalf("failed to get %q = %v: %v", item.Key, item.Value, err)
		}
		if !bytes.Equal([]byte(value), []byte(item.Value)) {
			t.Fatalf("invalid value for %q = %v: got %v", item.Key, item.Value, value)
		}
	}

	// GetAll
	subset := items[:len(items)/2]
	keys := subset.GetKeys()
	values, err := store.GetAll(keys)
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

	// Update
	for i, item := range items {
		next := items[(i+1)%len(items)]
		err := store.Put(item.Key, next.Value)
		if err != nil {
			t.Fatalf("failed to update %q = %v: %v", item.Key, next.Value, err)
		}
	}

	for i, item := range items {
		next := items[(i+1)%len(items)]
		value, err := store.Get(item.Key)
		if err != nil {
			t.Fatalf("failed to get updated %q = %v: %v", item.Key, next.Value, err)
		}
		if !bytes.Equal([]byte(value), []byte(next.Value)) {
			t.Fatalf("invalid updated value for %q = %v: got %v", item.Key, next.Value, value)
		}
	}

	// Delete
	for _, item := range items {
		err := store.Delete(item.Key)
		if err != nil {
			t.Fatalf("failed to delete %v: %v", item.Key, err)
		}
	}

	for _, item := range items {
		value, err := store.Get(item.Key)
		if err == nil {
			t.Fatalf("got deleted value %q = %v", item.Key, value)
		}
	}
}

func testList(t *testing.T, store KeyValueStore) {
	items := Items{
		newItem("path/0", "\x00\xFF\x00"),
		newItem("path/1", "\x01\xFF\x01"),
		newItem("path/2", "\x02\xFF\x02"),
		newItem("path/3", "\x03\xFF\x03"),
		newItem("path/4", "\x04\xFF\x04"),
		newItem("path/5", "\x05\xFF\x05"),
	}
	rand.Shuffle(len(items), items.Swap)

	defer cleanupItems(t, store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	t.Run("Without Key", func(t *testing.T) {
		keys, err := store.List(Key(""), Limit(3))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("invalid number of keys %v: %v", len(keys), err)
		}
		testKeysSorted(t, keys)
	})

	t.Run("With Key", func(t *testing.T) {
		keys, err := store.List(Key("path/2"), Limit(3))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("invalid number of keys %v: %v", len(keys), err)
		}
		testKeysSorted(t, keys)
	})

	t.Run("Without Key 100", func(t *testing.T) {
		keys, err := store.List(Key(""), Limit(100))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != len(items) {
			t.Fatalf("invalid number of keys %v expected %v: %q", len(keys), len(items), keys)
		}
		testKeysSorted(t, keys)
	})
}

func testKeysSorted(t *testing.T, keys Keys) {
	t.Helper()
	if len(keys) == 0 {
		return
	}

	a := keys[0]
	for _, b := range keys[1:] {
		if b.Less(a) {
			t.Fatal("unsorted order: %v", keys)
		}
	}
}

func cleanupItems(t *testing.T, store KeyValueStore, items Items) {
	t.Helper()
	for _, item := range items {
		store.Delete(item.Key)
	}
}
