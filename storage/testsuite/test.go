package testsuite

import (
	"bytes"
	"math/rand"
	"testing"

	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
)

// RunTests runs common storage.KeyValueStore tests
func RunTests(t *testing.T, store storage.KeyValueStore) {
	store = storelogger.NewTest(t, store)

	t.Run("CRUD", func(t *testing.T) { testCRUD(t, store) })
	t.Run("Constraints", func(t *testing.T) { testConstraints(t, store) })
	t.Run("List", func(t *testing.T) { testList(t, store) })
	t.Run("Iterate", func(t *testing.T) { testIterate(t, store) })
	t.Run("IterateAll", func(t *testing.T) { testIterateAll(t, store) })
	t.Run("Prefix", func(t *testing.T) { testPrefix(t, store) })
}

func testCRUD(t *testing.T, store storage.KeyValueStore) {
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

	defer cleanupItems(store, items)

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

func testConstraints(t *testing.T, store storage.KeyValueStore) {
	t.Run("Put Empty", func(t *testing.T) {
		var key storage.Key
		var val storage.Value
		defer store.Delete(key)

		err := store.Put(key, val)
		if err == nil {
			t.Fatal("putting empty key should fail")
		}
	})
}

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

func testIterate(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("a", "a", false),
		newItem("b/1", "b/1", false),
		newItem("b/2", "b/2", false),
		newItem("b/3", "b/3", false),
		newItem("c", "c", false),
		newItem("c/", "c/", false),
		newItem("c//", "c//", false),
		newItem("c/1", "c/1", false),
		newItem("g", "g", false),
		newItem("h", "h", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	t.Run("no limits", func(t *testing.T) {
		store.Iterate(nil, nil, '/',
			checkIterator(t, storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at a", func(t *testing.T) {
		store.Iterate(nil, storage.Key("a"), '/',
			checkIterator(t, storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after a", func(t *testing.T) {
		store.Iterate(nil, storage.NextKey(storage.Key("a")), '/',
			checkIterator(t, storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at b", func(t *testing.T) {
		store.Iterate(nil, storage.Key("b"), '/',
			checkIterator(t, storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after b", func(t *testing.T) {
		store.Iterate(nil, storage.NextKey(storage.Key("b")), '/',
			checkIterator(t, storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at c", func(t *testing.T) {
		store.Iterate(nil, storage.Key("c"), '/',
			checkIterator(t, storage.Items{
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after c", func(t *testing.T) {
		store.Iterate(nil, storage.NextKey(storage.Key("c")), '/',
			checkIterator(t, storage.Items{
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at e", func(t *testing.T) {
		store.Iterate(nil, storage.Key("e"), '/',
			checkIterator(t, storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after e", func(t *testing.T) {
		store.Iterate(nil, storage.NextKey(storage.Key("e")), '/',
			checkIterator(t, storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("prefix b slash", func(t *testing.T) {
		store.Iterate(storage.Key("b/"), nil, '/',
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}))
	})

	t.Run("prefix c slash", func(t *testing.T) {
		store.Iterate(storage.Key("c/"), nil, '/',
			checkIterator(t, storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "", true),
				newItem("c/1", "c/1", false),
			}))
	})

	t.Run("prefix c slash slash", func(t *testing.T) {
		store.Iterate(storage.Key("c//"), nil, '/',
			checkIterator(t, storage.Items{
				newItem("c//", "c//", false),
			}))
	})
}

func testIterateAll(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("a", "a", false),
		newItem("b/1", "b/1", false),
		newItem("b/2", "b/2", false),
		newItem("b/3", "b/3", false),
		newItem("c", "c", false),
		newItem("c/", "c/", false),
		newItem("c//", "c//", false),
		newItem("c/1", "c/1", false),
		newItem("g", "g", false),
		newItem("h", "h", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	t.Run("no limits", func(t *testing.T) {
		store.IterateAll(nil, nil,
			checkIterator(t, storage.Items{
				newItem("a", "a", false),
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at a", func(t *testing.T) {
		store.IterateAll(nil, storage.Key("a"),
			checkIterator(t, storage.Items{
				newItem("a", "a", false),
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after a", func(t *testing.T) {
		store.IterateAll(nil, storage.NextKey(storage.Key("a")),
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at b", func(t *testing.T) {
		store.IterateAll(nil, storage.Key("b"),
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after b", func(t *testing.T) {
		store.IterateAll(nil, storage.NextKey(storage.Key("b")),
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at c", func(t *testing.T) {
		store.IterateAll(nil, storage.Key("c"),
			checkIterator(t, storage.Items{
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("after c", func(t *testing.T) {
		store.IterateAll(nil, storage.NextKey(storage.Key("c")),
			checkIterator(t, storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("at e", func(t *testing.T) {
		store.IterateAll(nil, storage.Key("e"),
			checkIterator(t, storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}))
	})

	t.Run("prefix b slash", func(t *testing.T) {
		store.IterateAll(storage.Key("b/"), nil,
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}))

		store.IterateAll(storage.Key("b/"), storage.Key("a"),
			checkIterator(t, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}))

		store.IterateAll(storage.Key("b/"), storage.Key("b/2"),
			checkIterator(t, storage.Items{
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}))
	})

	t.Run("prefix c slash", func(t *testing.T) {
		store.IterateAll(storage.Key("c/"), nil,
			checkIterator(t, storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
			}))
	})

	t.Run("prefix c slash slash", func(t *testing.T) {
		store.IterateAll(storage.Key("c//"), nil,
			checkIterator(t, storage.Items{
				newItem("c//", "c//", false),
			}))
	})
}

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
		store.IterateAll(storage.Key("x-"), storage.Key("x-b"),
			checkIterator(t, storage.Items{
				newItem("x-b/1", "b/1", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/3", "b/3", false),
			}))
	})

	t.Run("prefix x dash b slash", func(t *testing.T) {
		store.Iterate(storage.Key("x-"), storage.Key("x-b"), '/',
			checkIterator(t, storage.Items{
				newItem("x-b/", "", true),
			}))
	})

	t.Run("prefix y- slash", func(t *testing.T) {
		store.IterateAll(storage.Key("y-"), nil,
			checkIterator(t, storage.Items{
				newItem("y-c", "c", false),
				newItem("y-c/", "c/", false),
				newItem("y-c//", "c//", false),
				newItem("y-c/1", "c/1", false),
				newItem("y-g", "g", false),
				newItem("y-h", "h", false),
			}))
	})
}
