// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststore

import (
	"bytes"
	"errors"
	"sort"
	"sync"

	"storj.io/storj/storage"
)

var errInternal = errors.New("internal error")

// Client implements in-memory key value store
type Client struct {
	mu sync.Mutex

	Items      []storage.ListItem
	ForceError int

	CallCount struct {
		Get         int
		Put         int
		List        int
		GetAll      int
		ReverseList int
		Delete      int
		Close       int
		Iterate     int
	}

	version int
}

// New creates a new in-memory key-value store
func New() *Client { return &Client{} }

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
func (store *Client) Put(key storage.Key, value storage.Value) error {
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

	store.Items = append(store.Items, storage.ListItem{})
	copy(store.Items[keyIndex+1:], store.Items[keyIndex:])
	store.Items[keyIndex] = storage.ListItem{
		Key:   storage.CloneKey(key),
		Value: storage.CloneValue(value),
	}

	return nil
}

// Get gets a value to store
func (store *Client) Get(key storage.Key) (storage.Value, error) {
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
		return nil, storage.ErrKeyNotFound.New(key.String())
	}

	return storage.CloneValue(store.Items[keyIndex].Value), nil
}

// GetAll gets all values from the store
func (store *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	defer store.locked()()

	store.CallCount.GetAll++
	if len(keys) > storage.LookupLimit {
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
func (store *Client) Delete(key storage.Key) error {
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
		return storage.ErrKeyNotFound.New(key.String())
	}

	copy(store.Items[keyIndex:], store.Items[keyIndex+1:])
	store.Items = store.Items[:len(store.Items)-1]
	return nil
}

// List lists all keys starting from start and upto limit items
func (store *Client) List(first storage.Key, limit int) (storage.Keys, error) {
	store.mu.Lock()
	store.CallCount.List++
	if store.forcedError() {
		store.mu.Unlock()
		return nil, errors.New("internal error")
	}
	store.mu.Unlock()
	return storage.ListKeys(store, first, limit)
}

// ReverseList lists all keys in revers order
func (store *Client) ReverseList(first storage.Key, limit int) (storage.Keys, error) {
	store.mu.Lock()
	store.CallCount.ReverseList++
	if store.forcedError() {
		store.mu.Unlock()
		return nil, errors.New("internal error")
	}
	store.mu.Unlock()
	return storage.ReverseListKeys(store, first, limit)
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

// Iterate iterates over items based on opts
func (store *Client) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	defer store.locked()()

	store.CallCount.Iterate++
	if store.forcedError() {
		return errInternal
	}

	var cursor advancer
	if !opts.Reverse {
		cursor = &forward{newCursor(store)}
	} else {
		cursor = &backward{newCursor(store)}
	}

	cursor.PositionToFirst(opts.Prefix, opts.First)
	var lastPrefix storage.Key
	var wasPrefix bool

	return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
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

type backward struct{ cursor }

func (cursor *backward) PositionToFirst(prefix, first storage.Key) {
	if prefix.IsZero() {
		// there's no prefix
		if first.IsZero() {
			// and no first item, so start from the end
			cursor.positionLast()
		} else {
			// theres a first item, so try to position on that or one before that
			cursor.positionBackward(first)
		}
	} else {
		// there's a prefix
		if first.IsZero() || storage.AfterPrefix(prefix).Less(first) {
			// there's no first, or it's after our prefix
			// storage.AfterPrefix("axxx/") is the next item after prefixes
			// so we position to the item before
			cursor.positionBefore(storage.AfterPrefix(prefix))
		} else {
			// otherwise try to position on first or one before that
			cursor.positionBackward(first)
		}
	}
}

func (cursor *backward) SkipPrefix(prefix storage.Key) (*storage.ListItem, bool) {
	cursor.positionBefore(prefix)
	return cursor.prev()
}

func (cursor *backward) Advance() (*storage.ListItem, bool) {
	return cursor.prev()
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
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
	cursor.lastKey = storage.CloneKey(key)
}

// positionLast positions at the last item
func (cursor *cursor) positionLast() {
	store := cursor.store
	cursor.version = store.version
	cursor.nextIndex = len(store.Items) - 1
	cursor.lastKey = storage.NextKey(store.Items[cursor.nextIndex].Key)
}

// positionBefore positions before key
func (cursor *cursor) positionBefore(key storage.Key) {
	store := cursor.store
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
	cursor.nextIndex--
	cursor.lastKey = storage.CloneKey(key) // TODO: probably not the right
}

// positionBackward positions at key or before key
func (cursor *cursor) positionBackward(key storage.Key) {
	store := cursor.store
	cursor.version = store.version
	var ok bool
	cursor.nextIndex, ok = store.indexOf(key)
	if !ok {
		cursor.nextIndex--
	}
	cursor.lastKey = storage.CloneKey(key)
}

func (cursor *cursor) next() (*storage.ListItem, bool) {
	store := cursor.store
	if cursor.done {
		return nil, false
	}

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

func (cursor *cursor) prev() (*storage.ListItem, bool) {
	store := cursor.store
	if cursor.done {
		return nil, false
	}

	if cursor.version != store.version {
		cursor.version = store.version
		var ok bool
		cursor.nextIndex, ok = store.indexOf(cursor.lastKey)
		if !ok {
			cursor.nextIndex--
		}
	}
	if cursor.nextIndex >= len(store.Items) {
		cursor.nextIndex = len(store.Items) - 1
	}
	if cursor.nextIndex < 0 {
		cursor.close()
		return nil, false
	}

	item := &store.Items[cursor.nextIndex]
	cursor.lastKey = item.Key
	cursor.nextIndex--
	return item, true
}
