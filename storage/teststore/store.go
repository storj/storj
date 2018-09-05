// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package teststore

import (
	"bytes"
	"errors"
	"sort"

	"storj.io/storj/storage"
)

var (
	// ErrNotExist is returned when looked item does not exist
	ErrNotExist = errors.New("does not exist")
)

// Client implements in-memory key value store
type Client struct {
	Items     []storage.ListItem
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

// Put adds a value to store
func (store *Client) Put(key storage.Key, value storage.Value) error {
	store.version++
	store.CallCount.Put++
	if key == nil {
		return storage.ErrEmptyKey
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
	store.CallCount.Get++

	keyIndex, found := store.indexOf(key)
	if !found {
		return nil, ErrNotExist
	}

	return storage.CloneValue(store.Items[keyIndex].Value), nil
}

// GetAll gets all values from the store
func (store *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	store.CallCount.GetAll++

	values := storage.Values{}
	for _, key := range keys {
		keyIndex, found := store.indexOf(key)
		if !found {
			return nil, ErrNotExist
		}
		values = append(values, storage.CloneValue(store.Items[keyIndex].Value))
	}
	return values, nil
}

// Delete deletes key and the value
func (store *Client) Delete(key storage.Key) error {
	store.version++
	store.CallCount.Delete++
	keyIndex, found := store.indexOf(key)
	if !found {
		return ErrNotExist
	}

	copy(store.Items[keyIndex:], store.Items[keyIndex+1:])
	store.Items = store.Items[:len(store.Items)-1]
	return nil
}

// List lists all keys starting from start and upto limit items
func (store *Client) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.CallCount.List++
	return storage.ListKeys(store, first, limit)
}

// ReverseList lists all keys in revers order
func (store *Client) ReverseList(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.CallCount.ReverseList++
	return storage.ReverseListKeys(store, first, limit)
}

// Close closes the store
func (store *Client) Close() error {
	store.CallCount.Close++
	return nil
}

// Iterate iterates over items based on opts
func (store *Client) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	store.CallCount.Iterate++
	if opts.Reverse {
		return store.iterateReverse(opts, fn)
	}
	return store.iterate(opts, fn)
}

func (store *Client) iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	var cur cursor
	if opts.First == nil || opts.First.Less(opts.Prefix) {
		cur.positionForward(store, opts.Prefix)
	} else {
		cur.positionForward(store, opts.First)
	}

	var lastPrefix storage.Key
	var wasPrefix bool

	return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
		next, ok := cur.next(store)
		if !ok {
			return false
		}

		if !opts.Recurse {
			if wasPrefix && bytes.HasPrefix(next.Key, lastPrefix) {
				cur.positionForward(store, storage.AfterPrefix(lastPrefix))
				next, ok = cur.next(store)
				if !ok {
					return false
				}
				wasPrefix = false
			}
		}

		if !bytes.HasPrefix(next.Key, opts.Prefix) {
			cur.close()
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

func (store *Client) iterateReverse(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	var cur cursor

	if opts.Prefix == nil {
		// there's no prefix
		if opts.First == nil {
			// and no first item, so start from the end
			cur.positionLast(store)
		} else {
			// theres a first item, so try to position on that or one before that
			cur.positionBackward(store, opts.First)
		}
	} else {
		// there's a prefix
		if opts.First == nil || storage.AfterPrefix(opts.Prefix).Less(opts.First) {
			// there's no first, or it's after our prefix
			// storage.AfterPrefix("axxx/") is the next item after prefixes
			// so we position to the item before
			cur.positionBefore(store, storage.AfterPrefix(opts.Prefix))
		} else {
			// otherwise try to position on first or one before that
			cur.positionBackward(store, opts.First)
		}
	}

	var lastPrefix storage.Key
	var wasPrefix bool

	return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
		next, ok := cur.prev(store)
		if !ok {
			return false
		}

		if !opts.Recurse {
			if wasPrefix && bytes.HasPrefix(next.Key, lastPrefix) {
				cur.positionBefore(store, lastPrefix)
				next, ok = cur.prev(store)
				if !ok {
					return false
				}
				wasPrefix = false
			}
		}

		if !bytes.HasPrefix(next.Key, opts.Prefix) {
			cur.close()
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

// cursor implements iterating over items with basic repositioning when the items change
type cursor struct {
	done      bool
	nextIndex int
	version   int
	lastKey   storage.Key
}

func (cursor *cursor) close() {
	cursor.done = true
}

// positionForward positions at key or the next item
func (cursor *cursor) positionForward(store *Client, key storage.Key) {
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
	cursor.lastKey = storage.CloneKey(key)
}

// positionLast positions at the last item
func (cursor *cursor) positionLast(store *Client) {
	cursor.version = store.version
	cursor.nextIndex = len(store.Items) - 1
	cursor.lastKey = storage.NextKey(store.Items[cursor.nextIndex].Key)
}

// positionBefore positions before key
func (cursor *cursor) positionBefore(store *Client, key storage.Key) {
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
	cursor.nextIndex--
	cursor.lastKey = storage.CloneKey(key) // TODO: probably not the right
}

// positionBackward positions at key or before key
func (cursor *cursor) positionBackward(store *Client, key storage.Key) {
	cursor.version = store.version
	var ok bool
	cursor.nextIndex, ok = store.indexOf(key)
	if !ok {
		cursor.nextIndex--
	}
	cursor.lastKey = storage.CloneKey(key)
}

func (cursor *cursor) next(store *Client) (*storage.ListItem, bool) {
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

func (cursor *cursor) prev(store *Client) (*storage.ListItem, bool) {
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
