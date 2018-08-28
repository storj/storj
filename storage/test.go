package storage

import (
	"bytes"
	"math/rand"
	"testing"
)

func RunTests(t *testing.T, store KeyValueStore) {
	t.Run("CRUD", func(t *testing.T) { testCRUD(t, store) })
	t.Run("List", func(t *testing.T) { testList(t, store) })

	t.Run("Iterator", func(t *testing.T) {
		iterable, ok := store.(IterableStore)
		if !ok {
			t.Skip("not implemented")
		}
		testIterator(t, iterable)
	})
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

	t.Run("Put", func(t *testing.T) {
		for _, item := range items {
			err := store.Put(item.Key, item.Value)
			if err != nil {
				t.Fatalf("failed to put %q = %v: %v", item.Key, item.Value, err)
			}
		}
	})

	rand.Shuffle(len(items), items.Swap)

	t.Run("Get", func(t *testing.T) {
		for _, item := range items {
			value, err := store.Get(item.Key)
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
	})

	t.Run("Update", func(t *testing.T) {
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
	})

	t.Run("Delete", func(t *testing.T) {
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
	})
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

	t.Run("Without Key, Limit 0", func(t *testing.T) {
		t.Skip("unimplemented")
		keys, err := store.List(Key(""), Limit(0))
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(keys) != len(items) {
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

func testIterator(t *testing.T, store IterableStore) {
	items := Items{
		newItem("a", "1"),
		newItem("b/", "2"),
		newItem("b/1", "3"),
		newItem("b/2", "4"),
		newItem("b/3", "5"),
		newItem("c", "6"),
		newItem("c/", "7"),
		newItem("c//", "8"),
		newItem("c/1", "9"),
		newItem("e", "10"),
		newItem("f", "11"),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(t, store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	mkitem := func(key, value string, isPrefix bool) ListItem {
		return ListItem{
			Key:      Key(key),
			Value:    Value(value),
			IsPrefix: isPrefix,
		}
	}

	checkIterator(t, "no limits", store.Iterate(nil, nil, '/'), []ListItem{
		mkitem("a", "1", false),
		mkitem("b/", "2", true),
		mkitem("c", "", false),
		mkitem("c/", "", true),
		mkitem("e", "10", false),
		mkitem("f", "11", false),
	})

	checkIterator(t, "start at c", store.Iterate(nil, Key("c"), '/'), []ListItem{
		mkitem("c", "", false),
		mkitem("c/", "", true),
		mkitem("e", "10", false),
		mkitem("f", "11", false),
	})

	checkIterator(t, "start at c", store.Iterate(Key("c"), nil, '/'), []ListItem{
		mkitem("c", "", false),
		mkitem("c/", "", true),
		mkitem("e", "10", false),
		mkitem("f", "11", false),
	})
}

func newItem(key, value string) ListItem {
	return ListItem{
		Key:   Key(key),
		Value: Value(value),
	}
}

func testKeysSorted(t *testing.T, keys Keys) {
	t.Helper()
	if len(keys) == 0 {
		return
	}

	a := keys[0]
	for _, b := range keys[1:] {
		if b.Less(a) {
			t.Fatalf("unsorted order: %v", keys)
		}
	}
}

func checkIterator(t *testing.T, name string, it Iterator, items []ListItem) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if err := it.Err(); err != nil {
				t.Fatalf("got error:", err)
			}
			if err := it.Close(); err != nil {
				t.Fatalf("failed to close:", err)
			}
		}()

		for i, item := range items {
			if !it.Next() {
				t.Fatalf("%d: finished early", i)
			}

			key, value, isPrefix := it.Key(), it.Value(), it.IsPrefix()
			if !key.Equal(item.Key) || !bytes.Equal(value, item.Value) || isPrefix != item.IsPrefix {
				t.Fatalf("%d: mismatch {%q,%q,%v} expected {{%q,%q,%v}",
					key, value, isPrefix,
					item.Key, item.Value, item.IsPrefix)
			}
		}

		if it.Next() {
			key, value, isPrefix := it.Key(), it.Value(), it.IsPrefix()
			t.Fatalf("%d: too many, got {%q,%q,%v}", len(items), key, value, isPrefix)
		}
	})
}

func cleanupItems(t *testing.T, store KeyValueStore, items Items) {
	t.Helper()
	for _, item := range items {
		store.Delete(item.Key)
	}
}
