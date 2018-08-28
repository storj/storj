package teststore

import (
	"errors"
	"sort"

	"storj.io/storj/storage"
)

var (
	ErrNotExist = errors.New("does not exist")
)

// Client implements in-memory key value store
type Client struct {
	items []keyvalue

	CallCount struct {
		Get         int
		Put         int
		List        int
		ListV2      int
		GetAll      int
		ReverseList int
		Delete      int
		Close       int
		Ping        int
	}
}

func New() *Client { return &Client{} }

type keyvalue struct {
	key   storage.Key
	value storage.Value
}

func (store *Client) indexOf(key storage.Key) (int, bool) {
	i := sort.Search(len(store.items), func(k int) bool {
		return !store.items[k].key.Less(key)
	})

	if i >= len(store.items) {
		return i, false
	}
	return i, store.items[i].key.Equal(key)
}

// Put adds a value to store
func (store *Client) Put(key storage.Key, value storage.Value) error {
	store.CallCount.Put++

	keyIndex, found := store.indexOf(key)
	if found {
		kv := &store.items[keyIndex]
		kv.value = cloneValue(value)
		return nil
	}

	store.items = append(store.items, keyvalue{})
	copy(store.items[keyIndex+1:], store.items[keyIndex:])
	store.items[keyIndex] = keyvalue{
		key:   cloneKey(key),
		value: cloneValue(value),
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

	return cloneValue(store.items[keyIndex].value), nil
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
		values = append(values, cloneValue(store.items[keyIndex].value))
	}
	return values, nil
}

// Delete deletes key and the value
func (store *Client) Delete(key storage.Key) error {
	store.CallCount.Delete++
	keyIndex, found := store.indexOf(key)
	if !found {
		return ErrNotExist
	}

	copy(store.items[keyIndex:], store.items[keyIndex+1:])
	store.items = store.items[:len(store.items)-1]
	return nil
}

// List lists all keys starting from start and upto limit items
func (store *Client) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.CallCount.List++

	firstIndex, _ := store.indexOf(first)
	lastIndex := firstIndex + int(limit)
	if lastIndex > len(store.items) {
		lastIndex = len(store.items)
	}

	keys := make(storage.Keys, lastIndex-firstIndex)
	for i, item := range store.items[firstIndex:lastIndex] {
		keys[i] = cloneKey(item.key)
	}

	return keys, nil
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
		item := store.items[i]
		keys[k] = cloneKey(item.key)
		k++
	}

	return keys, nil
}

// Close closes the store
func (store *Client) Close() error {
	store.CallCount.Close++

	return nil
}

func cloneKey(key storage.Key) storage.Key         { return append(key[:0], key...) }
func cloneValue(value storage.Value) storage.Value { return append(value[:0], value...) }
