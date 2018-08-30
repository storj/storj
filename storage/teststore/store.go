package teststore

import (
	"bytes"
	"errors"
	"sort"

	"storj.io/storj/storage"
)

var (
	ErrNotExist = errors.New("does not exist")
)

// Client implements in-memory key value store
type Client struct {
	Items     []storage.ListItem
	CallCount struct {
		Get         int
		Put         int
		List        int
		ListV2      int
		GetAll      int
		ReverseList int
		Delete      int
		Close       int
		Iterate     int
		IterateAll  int
	}

	version int
}

func New() *Client { return &Client{} }

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

// ListV2 lists all keys corresponding to ListOptions
func (store *Client) ListV2(opts storage.ListOptions) (storage.Items, storage.More, error) {
	store.CallCount.ListV2++

	return nil, false, errors.New("todo")
}

// ReverseList lists all keys in revers order
func (store *Client) ReverseList(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.CallCount.ReverseList++

	lastIndex, ok := store.indexOf(first)
	if !ok {
		lastIndex--
	}

	firstIndex := lastIndex - int(limit)
	if firstIndex < 0 {
		firstIndex = 0
	}

	keys := make(storage.Keys, lastIndex-firstIndex)
	k := 0
	for i := lastIndex; i >= firstIndex; i-- {
		item := store.Items[i]
		keys[k] = storage.CloneKey(item.Key)
		k++
	}

	return keys, nil
}

// Close closes the store
func (store *Client) Close() error {
	store.CallCount.Close++
	return nil
}

// Iterate iterates over collapsed items with prefix starting from first or the next key
func (store *Client) Iterate(prefix, first storage.Key, delimiter byte, fn func(storage.Iterator) error) error {
	store.CallCount.Iterate++

	if first.Less(prefix) {
		first = prefix
	}

	var cur cursor
	cur.positionTo(store, first)

	var lastPrefix storage.Key
	var wasPrefix bool

	return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
		next, ok := cur.next(store)
		if !ok {
			return false
		}

		if wasPrefix {
			for bytes.HasPrefix([]byte(next.Key), []byte(lastPrefix)) {
				next, ok = cur.next(store)
				if !ok {
					return false
				}
			}
		}

		if !bytes.HasPrefix(next.Key, prefix) {
			cur.close()
			return false
		}

		if p := bytes.IndexByte([]byte(next.Key[len(prefix):]), delimiter); p >= 0 {
			lastPrefix = append(lastPrefix[:0], next.Key[:len(prefix)+p+1]...)

			item.Key = append(item.Key[:0], storage.Key(lastPrefix)...)
			item.Value = item.Value[:0]
			item.IsPrefix = true

			wasPrefix = true
		} else {
			item.Key = append(item.Key[:0], next.Key...)
			item.Value = append(item.Value[:0], next.Value...)
			item.IsPrefix = false

			wasPrefix = false
		}

		return true
	}))
}

// IterateAll iterates over all items with prefix starting from first or the next key
func (store *Client) IterateAll(prefix, first storage.Key, fn func(it storage.Iterator) error) error {
	store.CallCount.IterateAll++

	if first.Less(prefix) {
		first = prefix
	}
	var cur cursor
	cur.positionTo(store, first)

	return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
		next, ok := cur.next(store)
		if !ok {
			return false
		}
		if !bytes.HasPrefix(next.Key, prefix) {
			cur.close()
			return false
		}

		item.Key = append(item.Key[:0], next.Key...)
		item.Value = append(item.Value[:0], next.Value...)
		item.IsPrefix = false

		return true
	}))
}

type cursor struct {
	done      bool
	nextIndex int
	version   int
	lastKey   storage.Key
}

func (cursor *cursor) close() {
	cursor.done = true
}

func (cursor *cursor) positionTo(store *Client, key storage.Key) {
	cursor.version = store.version
	cursor.nextIndex, _ = store.indexOf(key)
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

	cursor.lastKey = store.Items[cursor.nextIndex].Key
	cursor.nextIndex++
	return &store.Items[cursor.nextIndex-1], true
}
