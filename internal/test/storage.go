// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"bytes"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// KvStore is an in-memory, crappy key/value store type for testing
type KvStore map[string]storage.Value

// Empty checks if there are any keys in the store
func (k *KvStore) Empty() bool {
	return len(*k) == 0
}

// MockKeyValueStore is a `KeyValueStore` type used for testing (see storj.io/storj/storage/common.go)
type MockKeyValueStore struct {
	Data              KvStore
	GetCalled         int
	PutCalled         int
	ListCalled        int
	ReverseListCalled int
	DeleteCalled      int
	CloseCalled       int
	PingCalled        int

	IterateCalled int
}

var (
	// ErrMissingKey is the error class returned if a key is not in the mock store
	ErrMissingKey = errs.Class("missing")

	// ErrForced is the error class returned when the forced error flag is passed
	// to mock an error
	ErrForced = errs.Class("error forced by using 'error' key in mock")
)

// Get looks up the provided key from the MockKeyValueStore returning either an error or the result.
func (store *MockKeyValueStore) Get(key storage.Key) (storage.Value, error) {
	store.GetCalled++
	if key.String() == "error" {
		return nil, nil
	}
	v, ok := store.Data[key.String()]
	if !ok {
		return storage.Value{}, nil
	}

	return v, nil
}

// Put adds a value to the provided key in the MockKeyValueStore, returning an error on failure.
func (store *MockKeyValueStore) Put(key storage.Key, value storage.Value) error {
	store.PutCalled++
	store.Data[key.String()] = value
	return nil
}

// Delete deletes a key/value pair from the MockKeyValueStore, for a given the key
func (store *MockKeyValueStore) Delete(key storage.Key) error {
	store.DeleteCalled++
	delete(store.Data, key.String())
	return nil
}

// List returns either a list of keys for which the MockKeyValueStore has values or an error.
func (store *MockKeyValueStore) List(first storage.Key, limit int) (storage.Keys, error) {
	store.ListCalled++
	return storage.ListKeys(store, first, limit)
}

// GetAll is a noop to adhere to the interface
func (store *MockKeyValueStore) GetAll(keys storage.Keys) (values storage.Values, err error) {
	result := storage.Values{}
	for _, v := range keys {
		result = append(result, store.Data[v.String()])
	}
	return result, nil
}

func (store *MockKeyValueStore) allPrefixedItems(prefix, first, last storage.Key) storage.Items {
	var all storage.Items

	for key, value := range store.Data {
		if !bytes.HasPrefix([]byte(key), prefix) {
			continue
		}
		if first != nil && storage.Key(key).Less(first) {
			continue
		}
		if last != nil && last.Less(storage.Key(key)) {
			continue
		}

		all = append(all, storage.ListItem{
			Key:      storage.Key(key),
			Value:    value,
			IsPrefix: false,
		})
	}

	sort.Sort(all)
	return all
}

// ReverseList returns either a list of keys for which the MockKeyValueStore has values or an error.
func (store *MockKeyValueStore) ReverseList(first storage.Key, limit int) (storage.Keys, error) {
	return storage.ReverseListKeys(store, first, limit)
}

// Iterate iterates over items based on opts
func (store *MockKeyValueStore) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	store.IterateCalled++
	var items storage.Items
	if !opts.Reverse {
		items = store.allPrefixedItems(opts.Prefix, opts.First, nil)
	} else {
		items = store.allPrefixedItems(opts.Prefix, nil, opts.First)
	}

	if !opts.Recurse {
		items = storage.SortAndCollapse(items, opts.Prefix)
	}
	if opts.Reverse {
		items = storage.ReverseItems(items)
	}

	return fn(&storage.StaticIterator{
		Items: items,
	})
}

// Close closes the client
func (store *MockKeyValueStore) Close() error {
	store.CloseCalled++
	return nil
}

// Ping is called by some redis client code
func (store *MockKeyValueStore) Ping() error {
	store.PingCalled++
	return nil
}

// NewMockKeyValueStore returns a mocked `KeyValueStore` implementation for testing
func NewMockKeyValueStore(d KvStore) *MockKeyValueStore {
	return &MockKeyValueStore{
		Data: d,
	}
}
