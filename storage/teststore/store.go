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

	"storj.io/storj/storage"
)

var errInternal = errors.New("internal error")
var mon = monkit.Package()

// Client implements in-memory key value store
type Client struct {
	lookupLimit int

	mu sync.Mutex

	Items      []storage.ListItem
	ForceError int

	CallCount struct {
		Get            int
		Put            int
		List           int
		GetAll         int
		Delete         int
		Close          int
		Iterate        int
		CompareAndSwap int
	}

	version int
}

// New creates a new in-memory key-value store
func New() *Client { return &Client{lookupLimit: storage.DefaultLookupLimit} }

// MigrateToLatest pretends to migrate to latest db schema version.
func (store *Client) MigrateToLatest(ctx context.Context) error { return nil }

// SetLookupLimit sets the lookup limit.
func (store *Client) SetLookupLimit(v int) { store.lookupLimit = v }

// LookupLimit returns the maximum limit that is allowed.
func (store *Client) LookupLimit() int { return store.lookupLimit }

// indexOf finds index of key or where it could be inserted
func (store *Client) indexOf(key storage.Key) (int, bool) {
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

// Put adds a value to store
func (store *Client) Put(ctx context.Context, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.Put++
	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if found {
		kv := &store.Items[keyIndex]
		kv.Value = storage.CloneValue(value)
		return nil
	}

	store.put(keyIndex, key, value)
	return nil
}

// Get gets a value to store
func (store *Client) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.CallCount.Get++

	if store.forcedError() {
		return nil, errors.New("internal error")
	}

	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		return nil, storage.ErrKeyNotFound.New("%q", key)
	}

	return storage.CloneValue(store.Items[keyIndex].Value), nil
}

// GetAll gets all values from the store
func (store *Client) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.CallCount.GetAll++
	if len(keys) > store.lookupLimit {
		return nil, storage.ErrLimitExceeded
	}

	if store.forcedError() {
		return nil, errors.New("internal error")
	}

	values := storage.Values{}
	for _, key := range keys {
		keyIndex, found := store.indexOf(key)
		if !found {
			values = append(values, nil)
			continue
		}
		values = append(values, storage.CloneValue(store.Items[keyIndex].Value))
	}
	return values, nil
}

// Delete deletes key and the value
func (store *Client) Delete(ctx context.Context, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.Delete++

	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		return storage.ErrKeyNotFound.New("%q", key)
	}

	store.delete(keyIndex)
	return nil
}

// DeleteMultiple deletes keys ignoring missing keys
func (store *Client) DeleteMultiple(ctx context.Context, keys []storage.Key) (_ storage.Items, err error) {
	defer mon.Task()(&ctx, len(keys))(&err)
	defer store.locked()()

	store.version++
	store.CallCount.Delete++

	if store.forcedError() {
		return nil, errInternal
	}

	var items storage.Items
	for _, key := range keys {
		keyIndex, found := store.indexOf(key)
		if !found {
			continue
		}
		e := store.Items[keyIndex]
		items = append(items, storage.ListItem{
			Key:   e.Key,
			Value: e.Value,
		})
		store.delete(keyIndex)
	}

	return items, nil
}

// List lists all keys starting from start and upto limit items
func (store *Client) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	store.mu.Lock()
	store.CallCount.List++
	if store.forcedError() {
		store.mu.Unlock()
		return nil, errors.New("internal error")
	}
	store.mu.Unlock()
	return storage.ListKeys(ctx, store, first, limit)
}

// Close closes the store
func (store *Client) Close() error {
	defer store.locked()()

	store.CallCount.Close++
	if store.forcedError() {
		return errInternal
	}
	return nil
}

// Iterate iterates over items based on opts.
func (store *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	return store.IterateWithoutLookupLimit(ctx, opts, fn)
}

// IterateWithoutLookupLimit calls the callback with an iterator over the keys, but doesn't enforce default limit on opts.
func (store *Client) IterateWithoutLookupLimit(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	store.mu.Lock()
	store.CallCount.Iterate++
	if store.forcedError() {
		store.mu.Unlock()
		return errInternal
	}
	store.mu.Unlock()

	var cursor advancer = &forward{newCursor(store)}

	cursor.PositionToFirst(opts.Prefix, opts.First)
	var lastPrefix storage.Key
	var wasPrefix bool

	return fn(ctx, storage.IteratorFunc(
		func(ctx context.Context, item *storage.ListItem) bool {
			next, ok := cursor.Advance()
			if !ok {
				return false
			}

			if !opts.Recurse {
				if wasPrefix && bytes.HasPrefix(next.Key, lastPrefix) {
					next, ok = cursor.SkipPrefix(lastPrefix)

					if !ok {
						return false
					}
					wasPrefix = false
				}
			}

			if !bytes.HasPrefix(next.Key, opts.Prefix) {
				cursor.close()
				return false
			}

			if !opts.Recurse {
				if p := bytes.IndexByte([]byte(next.Key[len(opts.Prefix):]), storage.Delimiter); p >= 0 {
					lastPrefix = append(lastPrefix[:0], next.Key[:len(opts.Prefix)+p+1]...)

					item.Key = append(item.Key[:0], lastPrefix...)
					item.Value = item.Value[:0]
					item.IsPrefix = true

					wasPrefix = true
					return true
				}
			}

			item.Key = append(item.Key[:0], next.Key...)
			item.Value = append(item.Value[:0], next.Value...)
			item.IsPrefix = false

			return true
		}))
}

type advancer interface {
	close()
	PositionToFirst(prefix, first storage.Key)
	SkipPrefix(prefix storage.Key) (*storage.ListItem, bool)
	Advance() (*storage.ListItem, bool)
}

type forward struct{ cursor }

func (cursor *forward) PositionToFirst(prefix, first storage.Key) {
	if first.IsZero() || first.Less(prefix) {
		cursor.positionForward(prefix)
	} else {
		cursor.positionForward(first)
	}
}

func (cursor *forward) SkipPrefix(prefix storage.Key) (*storage.ListItem, bool) {
	cursor.positionForward(storage.AfterPrefix(prefix))
	return cursor.next()
}

func (cursor *forward) Advance() (*storage.ListItem, bool) {
	return cursor.next()
}

// cursor implements iterating over items with basic repositioning when the items change
type cursor struct {
	store     *Client
	done      bool
	nextIndex int
	version   int
	lastKey   storage.Key
}

func newCursor(store *Client) cursor { return cursor{store: store} }

func (cursor *cursor) close() {
	cursor.store = nil
	cursor.done = true
}

// positionForward positions at key or the next item
func (cursor *cursor) positionForward(key storage.Key) {
	store := cursor.store
	store.mu.Lock()
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
	store.mu.Unlock()
	cursor.lastKey = storage.CloneKey(key)
}

func (cursor *cursor) next() (*storage.ListItem, bool) {
	store := cursor.store
	if cursor.done {
		return nil, false
	}
	defer store.locked()()

	if cursor.version != store.version {
		cursor.version = store.version
		var ok bool
		cursor.nextIndex, ok = store.indexOf(cursor.lastKey)
		if ok {
			cursor.nextIndex++
		}
	}

	if cursor.nextIndex >= len(store.Items) {
		cursor.close()
		return nil, false
	}

	item := &store.Items[cursor.nextIndex]
	cursor.lastKey = item.Key
	cursor.nextIndex++
	return item, true
}

// CompareAndSwap atomically compares and swaps oldValue with newValue
func (store *Client) CompareAndSwap(ctx context.Context, key storage.Key, oldValue, newValue storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer store.locked()()

	store.version++
	store.CallCount.CompareAndSwap++
	if store.forcedError() {
		return errInternal
	}

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	keyIndex, found := store.indexOf(key)
	if !found {
		if oldValue != nil {
			return storage.ErrKeyNotFound.New("%q", key)
		}

		if newValue == nil {
			return nil
		}

		store.put(keyIndex, key, newValue)
		return nil
	}

	kv := &store.Items[keyIndex]
	if !bytes.Equal(kv.Value, oldValue) {
		return storage.ErrValueChanged.New("%q", key)
	}

	if newValue == nil {
		store.delete(keyIndex)
		return nil
	}

	kv.Value = storage.CloneValue(newValue)

	return nil
}

func (store *Client) put(keyIndex int, key storage.Key, value storage.Value) {
	store.Items = append(store.Items, storage.ListItem{})
	copy(store.Items[keyIndex+1:], store.Items[keyIndex:])
	store.Items[keyIndex] = storage.ListItem{
		Key:   storage.CloneKey(key),
		Value: storage.CloneValue(value),
	}
}

func (store *Client) delete(keyIndex int) {
	copy(store.Items[keyIndex:], store.Items[keyIndex+1:])
	store.Items = store.Items[:len(store.Items)-1]
}
