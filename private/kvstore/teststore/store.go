// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststore

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/private/kvstore"
)

var errInternal = errors.New("internal error")
var mon = monkit.Package()

// Client implements in-memory key value store.
type Client struct {
	mu sync.Mutex

	Items      []kvstore.Item
	ForceError int

	CallCount struct {
		Get            int
		Put            int
		Delete         int
		Close          int
		Range          int
		CompareAndSwap int
	}

	version int
}

// New creates a new in-memory key-value store.
func New() *Client { return &Client{} }

// MigrateToLatest pretends to migrate to latest db schema version.
func (store *Client) MigrateToLatest(ctx context.Context) error { return nil }

// indexOf finds index of key or where it could be inserted.
func (store *Client) indexOf(key kvstore.Key) (int, bool) {
	i := sort.Search(len(store.Items), func(k int) bool {
		return !store.Items[k].Key.Less(key)
	})

	if i >= len(store.Items) {
		return i, false
	}
	return i, store.Items[i].Key.Equal(key)
}

func (store *Client) locked() func() {
	store.mu.Lock()
	return store.mu.Unlock
}

func (store *Client) forcedError() bool {
	if store.ForceError > 0 {
		store.ForceError--
		return true
	}
	return false
}

// Put adds a value to store.
func (store *Client) Put(ctx context.Context, key kvstore.Key, value kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.Put++
	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if found {
		kv := &store.Items[keyIndex]
		kv.Value = kvstore.CloneValue(value)
		return nil
	}

	store.put(keyIndex, key, value)
	return nil
}

// Get gets a value to store.
func (store *Client) Get(ctx context.Context, key kvstore.Key) (_ kvstore.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.CallCount.Get++

	if store.forcedError() {
		return nil, errors.New("internal error")
	}

	if key.IsZero() {
		return nil, kvstore.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		return nil, kvstore.ErrKeyNotFound.New("%q", key)
	}

	return kvstore.CloneValue(store.Items[keyIndex].Value), nil
}

// Delete deletes key and the value.
func (store *Client) Delete(ctx context.Context, key kvstore.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.Delete++

	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		return kvstore.ErrKeyNotFound.New("%q", key)
	}

	store.delete(keyIndex)
	return nil
}

// Close closes the store.
func (store *Client) Close() error {
	defer store.locked()()

	store.CallCount.Close++
	if store.forcedError() {
		return errInternal
	}
	return nil
}

// Range iterates over all items in unspecified order.
func (store *Client) Range(ctx context.Context, fn func(context.Context, kvstore.Key, kvstore.Value) error) error {
	store.mu.Lock()
	store.CallCount.Range++
	if store.forcedError() {
		store.mu.Unlock()
		return errors.New("internal error")
	}
	items := append([]kvstore.Item{}, store.Items...)
	store.mu.Unlock()

	for _, item := range items {
		if err := fn(ctx, item.Key, item.Value); err != nil {
			return err
		}
	}
	return nil
}

// CompareAndSwap atomically compares and swaps oldValue with newValue.
func (store *Client) CompareAndSwap(ctx context.Context, key kvstore.Key, oldValue, newValue kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.CompareAndSwap++
	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		if oldValue != nil {
			return kvstore.ErrKeyNotFound.New("%q", key)
		}

		if newValue == nil {
			return nil
		}

		store.put(keyIndex, key, newValue)
		return nil
	}

	kv := &store.Items[keyIndex]
	if !bytes.Equal(kv.Value, oldValue) {
		return kvstore.ErrValueChanged.New("%q", key)
	}

	if newValue == nil {
		store.delete(keyIndex)
		return nil
	}

	kv.Value = kvstore.CloneValue(newValue)

	return nil
}

func (store *Client) put(keyIndex int, key kvstore.Key, value kvstore.Value) {
	store.Items = append(store.Items, kvstore.Item{})
	copy(store.Items[keyIndex+1:], store.Items[keyIndex:])
	store.Items[keyIndex] = kvstore.Item{
		Key:   kvstore.CloneKey(key),
		Value: kvstore.CloneValue(value),
	}
}

func (store *Client) delete(keyIndex int) {
	copy(store.Items[keyIndex:], store.Items[keyIndex+1:])
	store.Items = store.Items[:len(store.Items)-1]
}
