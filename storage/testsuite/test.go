// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

// RunTests runs common storage.KeyValueStore tests
func RunTests(t *testing.T, store storage.KeyValueStore) {
	// store = storelogger.NewTest(t, store)
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("CRUD", func(t *testing.T) { testCRUD(t, ctx, store) })
	t.Run("Constraints", func(t *testing.T) { testConstraints(t, ctx, store) })
	t.Run("Iterate", func(t *testing.T) { testIterate(t, ctx, store) })
	t.Run("IterateAll", func(t *testing.T) { testIterateAll(t, ctx, store) })
	t.Run("Prefix", func(t *testing.T) { testPrefix(t, ctx, store) })

	t.Run("List", func(t *testing.T) { testList(t, ctx, store) })
	t.Run("ListV2", func(t *testing.T) { testListV2(t, ctx, store) })

	t.Run("Parallel", func(t *testing.T) { testParallel(t, ctx, store) })
}

func testConstraints(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore) {
	var items storage.Items
	for i := 0; i < storage.LookupLimit+5; i++ {
		items = append(items, storage.ListItem{
			Key:   storage.Key("test-" + strconv.Itoa(i)),
			Value: storage.Value("xyz"),
		})
	}

	var group errgroup.Group
	for _, item := range items {
		key := item.Key
		value := item.Value
		group.Go(func() error {
			return store.Put(ctx, key, value)
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	defer cleanupItems(t, ctx, store, items)

	t.Run("Put Empty", func(t *testing.T) {
		var key storage.Key
		var val storage.Value
		defer func() { _ = store.Delete(ctx, key) }()

		err := store.Put(ctx, key, val)
		if err == nil {
			t.Fatal("putting empty key should fail")
		}
	})

	t.Run("GetAll limit", func(t *testing.T) {
		_, err := store.GetAll(ctx, items[:storage.LookupLimit].GetKeys())
		if err != nil {
			t.Fatalf("GetAll LookupLimit should succeed: %v", err)
		}

		_, err = store.GetAll(ctx, items[:storage.LookupLimit+1].GetKeys())
		if err == nil && err == storage.ErrLimitExceeded {
			t.Fatalf("GetAll LookupLimit+1 should fail: %v", err)
		}
	})

	t.Run("List limit", func(t *testing.T) {
		keys, err := store.List(ctx, nil, storage.LookupLimit)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("List LookupLimit should succeed: %v / got %d", err, len(keys))
		}
		_, err = store.List(ctx, nil, storage.LookupLimit+1)
		if err != nil || len(keys) != storage.LookupLimit {
			t.Fatalf("List LookupLimit+1 shouldn't fail: %v / got %d", err, len(keys))
		}
	})

	t.Run("CompareAndSwap Empty Key", func(t *testing.T) {
		var key storage.Key
		var val storage.Value

		err := store.CompareAndSwap(ctx, key, val, val)
		require.Error(t, err, "putting empty key should fail")
	})

	t.Run("CompareAndSwap Empty Old Value", func(t *testing.T) {
		key := storage.Key("test-key")
		val := storage.Value("test-value")
		defer func() { _ = store.Delete(ctx, key) }()

		err := store.CompareAndSwap(ctx, key, nil, val)
		require.NoError(t, err, "failed to update %q: %v -> %v: %+v", key, nil, val, err)

		value, err := store.Get(ctx, key)
		require.NoError(t, err, "failed to get %q = %v: %+v", key, val, err)
		require.Equal(t, value, val, "invalid value for %q = %v: got %v", key, val, value)
	})

	t.Run("CompareAndSwap Empty New Value", func(t *testing.T) {
		key := storage.Key("test-key")
		val := storage.Value("test-value")
		defer func() { _ = store.Delete(ctx, key) }()

		err := store.Put(ctx, key, val)
		require.NoError(t, err, "failed to put %q = %v: %+v", key, val, err)

		err = store.CompareAndSwap(ctx, key, val, nil)
		require.NoError(t, err, "failed to update %q: %v -> %v: %+v", key, val, nil, err)

		value, err := store.Get(ctx, key)
		require.Error(t, err, "got deleted value %q = %v", key, value)
	})

	t.Run("CompareAndSwap Empty Both Empty Values", func(t *testing.T) {
		key := storage.Key("test-key")

		err := store.CompareAndSwap(ctx, key, nil, nil)
		require.NoError(t, err, "failed to update %q: %v -> %v: %+v", key, nil, nil, err)

		value, err := store.Get(ctx, key)
		require.Error(t, err, "got unexpected value %q = %v", key, value)
	})

	t.Run("CompareAndSwap Missing Key", func(t *testing.T) {
		for i, tt := range []struct {
			old, new storage.Value
		}{
			{storage.Value("old-value"), nil},
			{storage.Value("old-value"), storage.Value("new-value")},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)
			key := storage.Key("test-key")

			err := store.CompareAndSwap(ctx, key, tt.old, tt.new)
			assert.True(t, storage.ErrKeyNotFound.Has(err), "%s: unexpected error: %+v", errTag, err)
		}
	})

	t.Run("CompareAndSwap Value Changed", func(t *testing.T) {
		for i, tt := range []struct {
			old, new storage.Value
		}{
			{nil, nil},
			{nil, storage.Value("new-value")},
			{storage.Value("old-value"), nil},
			{storage.Value("old-value"), storage.Value("new-value")},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)
			key := storage.Key("test-key")
			val := storage.Value("test-value")
			defer func() { _ = store.Delete(ctx, key) }()

			err := store.Put(ctx, key, val)
			require.NoError(t, err, errTag)

			err = store.CompareAndSwap(ctx, key, tt.old, tt.new)
			assert.True(t, storage.ErrValueChanged.Has(err), "%s: unexpected error: %+v", errTag, err)
		}
	})

	t.Run("CompareAndSwap Concurrent", func(t *testing.T) {
		const count = 100

		key := storage.Key("test-key")
		defer func() { _ = store.Delete(ctx, key) }()

		// Add concurrently all numbers from 1 to `count` in a set under test-key
		var group errgroup.Group
		for i := 0; i < count; i++ {
			i := i
			group.Go(func() error {
				for {
					set := make(map[int]bool)

					oldValue, err := store.Get(ctx, key)
					if !storage.ErrKeyNotFound.Has(err) {
						if err != nil {
							return err
						}

						set, err = decodeSet(oldValue)
						if err != nil {
							return err
						}
					}

					set[i] = true
					newValue, err := encodeSet(set)
					if err != nil {
						return err
					}

					err = store.CompareAndSwap(ctx, key, oldValue, storage.Value(newValue))
					if storage.ErrValueChanged.Has(err) {
						// Another goroutine was faster. Make a new attempt.
						continue
					}

					return err
				}
			})
		}
		err := group.Wait()
		require.NoError(t, err)

		// Check that all numbers were added in the set
		value, err := store.Get(ctx, key)
		require.NoError(t, err)

		set, err := decodeSet(value)
		require.NoError(t, err)

		for i := 0; i < count; i++ {
			assert.Contains(t, set, i)
		}
	})
}

func encodeSet(set map[int]bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(set)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeSet(b []byte) (map[int]bool, error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)

	var set map[int]bool
	err := dec.Decode(&set)
	if err != nil {
		return nil, err
	}

	return set, nil
}
