// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
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
}

func testConstraints(t *testing.T, store storage.KeyValueStore) {
	testKey := storage.Key("test")
	if err := store.Put(testKey, storage.Value("xyz")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Delete(testKey); err != nil {
			t.Fatal(err)
		}
	}()

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
		keys := make([]storage.Key, storage.LookupLimit+1)
		for i := range keys {
			keys[i] = testKey
		}

		_, err := store.GetAll(keys[:storage.LookupLimit])
		if err != nil {
			t.Fatalf("GetAll LookupLimit should succeed: %v", err)
		}

		_, err = store.GetAll(keys[:storage.LookupLimit+1])
		if err == nil && err == storage.ErrLimitExceeded {
			t.Fatalf("GetAll LookupLimit+1 should fail: %v", err)
		}
	})

	t.Run("List limit", func(t *testing.T) {
		_, err := store.List(nil, storage.LookupLimit)
		if err != nil {
			t.Fatalf("List LookupLimit should succeed: %v", err)
		}
		_, err = store.ReverseList(nil, storage.LookupLimit)
		if err != nil {
			t.Fatalf("ReverseList LookupLimit should succeed: %v", err)
		}

		_, err = store.List(nil, storage.LookupLimit+1)
		if err == nil && err == storage.ErrLimitExceeded {
			t.Fatalf("List LookupLimit+1 should fail: %v", err)
		}
		_, err = store.ReverseList(nil, storage.LookupLimit+1)
		if err == nil && err == storage.ErrLimitExceeded {
			t.Fatalf("ReverseList LookupLimit+1 should fail: %v", err)
		}
	})
}
