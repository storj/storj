// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"strconv"
	"testing"

	"storj.io/storj/storage"
)

// RunTests runs common storage.KeyValueStore tests
func RunTests(t *testing.T, store storage.KeyValueStore) {
	// store = storelogger.NewTest(t, store)

	t.Run("CRUD", func(t *testing.T) { testCRUD(t, store) })
	t.Run("Constraints", func(t *testing.T) { testConstraints(t, store) })
	t.Run("Iterate", func(t *testing.T) { testIterate(t, store) })
	t.Run("IterateAll", func(t *testing.T) { testIterateAll(t, store) })
	t.Run("Prefix", func(t *testing.T) { testPrefix(t, store) })

	t.Run("List", func(t *testing.T) { testList(t, store) })
	t.Run("ListV2", func(t *testing.T) { testListV2(t, store) })

	t.Run("Parallel", func(t *testing.T) { testParallel(t, store) })
}

func testConstraints(t *testing.T, store storage.KeyValueStore) {
	var items storage.Items
	for i := 0; i < storage.LookupLimit+5; i++ {
		items = append(items, storage.ListItem{
			Key:   storage.Key("test-" + strconv.Itoa(i)),
			Value: storage.Value("xyz"),
		})
	}

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatal(err)
		}
	}
	defer cleanupItems(store, items)

	t.Run("Put Empty", func(t *testing.T) {
		var key storage.Key
		var val storage.Value
		defer func() { _ = store.Delete(key) }()

		err := store.Put(key, val)
		if err == nil {
			t.Fatal("putting empty key should fail")
		}
	})

	t.Run("GetAll limit", func(t *testing.T) {
		_, err := store.GetAll(items[:storage.LookupLimit].GetKeys())
		if err != nil {
			t.Fatalf("GetAll LookupLimit should succeed: %v", err)
		}

		_, err = store.GetAll(items[:storage.LookupLimit+1].GetKeys())
		if err == nil && err == storage.ErrLimitExceeded {
			t.Fatalf("GetAll LookupLimit+1 should fail: %v", err)
		}
	})

	t.Run("List limit", func(t *testing.T) {
		keys, err := store.List(nil, storage.LookupLimit)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("List LookupLimit should succeed: %v / got %d", err, len(keys))
		}
		keys, err = store.ReverseList(nil, storage.LookupLimit)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("ReverseList LookupLimit should succeed: %v / got %d", err, len(keys))
		}

		_, err = store.List(nil, storage.LookupLimit+1)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("List LookupLimit+1 shouldn't fail: %v / got %d", err, len(keys))
		}
		_, err = store.ReverseList(nil, storage.LookupLimit+1)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("ReverseList LookupLimit+1 shouldn't fail: %v / got %d", err, len(keys))
		}
	})
}
