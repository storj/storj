package storage

import (
	"bytes"
	"math/rand"
	"testing"

	"go.uber.org/zap/zaptest"
)

func RunTests(t *testing.T, store KeyValueStore) {
	store = NewLogger(zaptest.NewLogger(t), store)

	t.Run("CRUD", func(t *testing.T) { testCRUD(t, store) })
	t.Run("Constraints", func(t *testing.T) { testConstraints(t, store) })
	t.Run("List", func(t *testing.T) { testList(t, store) })
	t.Run("Iterator", func(t *testing.T) { testIterator(t, store) })
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

func testConstraints(t *testing.T, store KeyValueStore) {
	t.Run("Put Empty", func(t *testing.T) {
		var key Key
		var val Value
		defer store.Delete(key)

		err := store.Put(key, val)
		if err == nil {
			t.Fatal("putting empty key should fail")
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

func testIterator(t *testing.T, store KeyValueStore) {
	items := Items{
		newItem("a", "a"),
		newItem("b/1", "b/1"),
		newItem("b/2", "b/2"),
		newItem("b/3", "b/3"),
		newItem("c", "c"),
		newItem("c/", "c/"),
		newItem("c//", "c//"),
		newItem("c/1", "c/1"),
		newItem("g", "g"),
		newItem("h", "h"),
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

	t.Run("no limits", func(t *testing.T) {
		store.Iterate(nil, nil, '/',
			checkIterator(t, []ListItem{
				mkitem("a", "a", false),
				mkitem("b/", "", true),
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("at a", func(t *testing.T) {
		store.Iterate(nil, Key("a"), '/',
			checkIterator(t, []ListItem{
				mkitem("a", "a", false),
				mkitem("b/", "", true),
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("after a", func(t *testing.T) {
		store.Iterate(nil, NextKey(Key("a")), '/',
			checkIterator(t, []ListItem{
				mkitem("b/", "", true),
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("at b", func(t *testing.T) {
		store.Iterate(nil, Key("b"), '/',
			checkIterator(t, []ListItem{
				mkitem("b/", "", true),
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("after b", func(t *testing.T) {
		store.Iterate(nil, NextKey(Key("b")), '/',
			checkIterator(t, []ListItem{
				mkitem("b/", "", true),
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("at c", func(t *testing.T) {
		store.Iterate(nil, Key("c"), '/',
			checkIterator(t, []ListItem{
				mkitem("c", "c", false),
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("after c", func(t *testing.T) {
		store.Iterate(nil, NextKey(Key("c")), '/',
			checkIterator(t, []ListItem{
				mkitem("c/", "", true),
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("at e", func(t *testing.T) {
		store.Iterate(nil, Key("e"), '/',
			checkIterator(t, []ListItem{
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("after e", func(t *testing.T) {
		store.Iterate(nil, NextKey(Key("e")), '/',
			checkIterator(t, []ListItem{
				mkitem("g", "g", false),
				mkitem("h", "h", false),
			}))
	})

	t.Run("prefix b slash", func(t *testing.T) {
		store.Iterate(Key("b/"), nil, '/',
			checkIterator(t, []ListItem{
				mkitem("b/1", "b/1", false),
				mkitem("b/2", "b/2", false),
				mkitem("b/3", "b/3", false),
			}))
	})

	t.Run("prefix c slash", func(t *testing.T) {
		store.Iterate(Key("c/"), nil, '/',
			checkIterator(t, []ListItem{
				mkitem("c/", "c/", false),
				mkitem("c//", "", true),
				mkitem("c/1", "c/1", false),
			}))
	})

	t.Run("prefix c slash slash", func(t *testing.T) {
		store.Iterate(Key("c//"), nil, '/',
			checkIterator(t, []ListItem{
				mkitem("c//", "c//", false),
			}))
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

func checkIterator(t *testing.T, items []ListItem) func(it Iterator) error {
	t.Helper()
	return func(it Iterator) error {
		t.Helper()

		var got ListItem
		maxErrors := 5
		for i, exp := range items {
			if !it.Next(&got) {
				t.Fatalf("%d: finished early", i)
			}

			if !got.Key.Equal(exp.Key) || !bytes.Equal(got.Value, exp.Value) || got.IsPrefix != exp.IsPrefix {
				t.Errorf("%d: mismatch {%q,%q,%v} expected {{%q,%q,%v}", i,
					got.Key, got.Value, got.IsPrefix,
					exp.Key, exp.Value, exp.IsPrefix)
				maxErrors--
				if maxErrors <= 0 {
					t.Fatal("too many errors")
					return nil
				}
			}
		}

		if it.Next(&got) {
			t.Fatalf("%d: too many, got {%q,%q,%v}", len(items),
				got.Key, got.Value, got.IsPrefix)
		}
		return nil
	}
}

func cleanupItems(t *testing.T, store KeyValueStore, items Items) {
	t.Helper()
	for _, item := range items {
		store.Delete(item.Key)
	}
}
