package inmemstore

import (
	"errors"
	"sort"
	"sync"

	"storj.io/storj/storage"
)

var (
	ErrNotExist = errors.New("does not exist")
)

// Store implements in-memory key value store
type Store struct {
	mu    sync.Mutex
	items []keyvalue
}

func New() *Store { return &Store{} }

type keyvalue struct {
	key   storage.Key
	value storage.Value
}

func (store *Store) locked() func() {
	store.mu.Lock()
	return store.mu.Unlock
}

func (store *Store) indexOf(key storage.Key) (int, bool) {
	i := sort.Search(len(store.items), func(k int) bool {
		return !store.items[k].key.Less(key)
	})

	if i >= len(store.items) {
		return i, false
	}
	return i, store.items[i].key.Equal(key)
}

func cloneKey(key storage.Key) storage.Key         { return append(key[:0], key...) }
func cloneValue(value storage.Value) storage.Value { return append(value[:0], value...) }

// Put adds a value to store
func (store *Store) Put(key storage.Key, value storage.Value) error {
	defer store.locked()()

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
func (store *Store) Get(key storage.Key) (storage.Value, error) {
	defer store.locked()()

	keyIndex, found := store.indexOf(key)
	if !found {
		return nil, ErrNotExist
	}

	return cloneValue(store.items[keyIndex].value), nil
}

// GetAll gets all values from the store
func (store *Store) GetAll(keys storage.Keys) (storage.Values, error) {
	defer store.locked()()

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
func (store *Store) Delete(key storage.Key) error {
	defer store.locked()()
	keyIndex, found := store.indexOf(key)
	if !found {
		return ErrNotExist
	}

	copy(store.items[keyIndex:], store.items[keyIndex+1:])
	store.items = store.items[:len(store.items)-1]
	return nil
}

// List lists all keys starting from start and upto limit items
func (store *Store) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	defer store.locked()()

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
func (store *Store) ListV2(opts storage.ListOptions) (storage.Items, storage.More, error) {
	return nil, false, errors.New("todo")
}

// ReverseList lists all keys in revers order
func (store *Store) ReverseList(first storage.Key, limit storage.Limit) (storage.Keys, error) {
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
func (store *Store) Close() error {
	return nil
}
