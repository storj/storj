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

var _ storage.IterableStore = &Client{}

// Client implements in-memory key value store
type Client struct {
	Items []KeyValue

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

type KeyValue struct {
	Key   storage.Key
	Value storage.Value
}

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
	store.CallCount.Put++

	keyIndex, found := store.indexOf(key)
	if found {
		kv := &store.Items[keyIndex]
		kv.Value = cloneValue(value)
		return nil
	}

	store.Items = append(store.Items, KeyValue{})
	copy(store.Items[keyIndex+1:], store.Items[keyIndex:])
	store.Items[keyIndex] = KeyValue{
		Key:   cloneKey(key),
		Value: cloneValue(value),
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

	return cloneValue(store.Items[keyIndex].Value), nil
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
		values = append(values, cloneValue(store.Items[keyIndex].Value))
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

	copy(store.Items[keyIndex:], store.Items[keyIndex+1:])
	store.Items = store.Items[:len(store.Items)-1]
	return nil
}

// List lists all keys starting from start and upto limit items
func (store *Client) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.CallCount.List++

	firstIndex, _ := store.indexOf(first)
	lastIndex := firstIndex + int(limit)
	if lastIndex > len(store.Items) {
		lastIndex = len(store.Items)
	}

	keys := make(storage.Keys, lastIndex-firstIndex)
	for i, item := range store.Items[firstIndex:lastIndex] {
		keys[i] = cloneKey(item.Key)
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
		item := store.Items[i]
		keys[k] = cloneKey(item.Key)
		k++
	}

	return keys, nil
}

func (store *Client) Iterate(prefix, after storage.Key, delimiter byte) storage.Iterator {
	if after.Less(prefix) {
		after = prefix
	}

	return &iterator{
		store:     store,
		prefix:    prefix,
		delimiter: delimiter,
		head:      after,
		value:     nil,
		isPrefix:  false,
	}
}

// Close closes the store
func (store *Client) Close() error {
	store.CallCount.Close++

	return nil
}

func cloneKey(key storage.Key) storage.Key         { return append(key[:0], key...) }
func cloneValue(value storage.Value) storage.Value { return append(value[:0], value...) }

var _ storage.Iterator = &iterator{}

type iterator struct {
	store *Client

	prefix    storage.Key
	delimiter byte

	head     storage.Key
	value    storage.Value
	isPrefix bool
}

func (it *iterator) Next() bool {
	index, ok := it.store.indexOf(it.head)
	if ok {
		index++
	}

	// skip all other with the same prefix
	if it.isPrefix {
		for ; index < len(it.store.Items); index++ {
			if !bytes.HasPrefix(it.store.Items[index].Key, it.head) {
				break
			}
		}
	}

	// all done?
	if index >= len(it.store.Items) {
		it.cleanup()
		return false
	}

	// check whether we are still in the correct prefix
	next := &it.store.Items[index]
	if !bytes.HasPrefix(next.Key, it.prefix) {
		return false
	}

	// update head
	it.head = next.Key
	it.isPrefix = false

	// check whether it is a nested item
	for i, b := range it.head[len(it.prefix):] {
		if b == it.delimiter {
			it.isPrefix = true
			it.head = it.head[:i+1]
			break
		}
	}

	if !it.isPrefix {
		it.value = next.Value
	} else {
		it.value = nil
	}

	return true
}

func (it *iterator) cleanup() {
	it.store = nil
	it.head = nil
	it.value = nil
	it.isPrefix = false
}

func (it *iterator) Close() error {
	it.cleanup()
	return nil
}

func (it *iterator) Key() storage.Key     { return it.head }
func (it *iterator) IsPrefix() bool       { return it.isPrefix }
func (it *iterator) Value() storage.Value { return it.value }

func (it *iterator) Err() error { return nil }
