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
		Iterate     int
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
	if key == nil {
		return storage.ErrEmptyKey
	}

	keyIndex, found := store.indexOf(key)
	if found {
		kv := &store.Items[keyIndex]
		kv.Value = storage.CloneValue(value)
		return nil
	}

	store.Items = append(store.Items, KeyValue{})
	copy(store.Items[keyIndex+1:], store.Items[keyIndex:])
	store.Items[keyIndex] = KeyValue{
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
		keys[i] = storage.CloneKey(item.Key)
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

func (store *Client) Iterate(prefix, after storage.Key, delimiter byte) storage.Iterator {
	store.CallCount.Iterate++
	return &iterator{
		store:     store,
		lastIndex: -1,
		prefix:    prefix,
		delimiter: delimiter,
		head:      after,
		isPrefix:  false,
	}
}

type iterator struct {
	store     *Client
	lastIndex int

	prefix    storage.Key
	delimiter byte

	head     storage.Key
	isPrefix bool
}

func (it *iterator) Next(item *storage.ListItem) bool {
	var index int = -1
	var found bool

	if it.prefix != nil || it.lastIndex < 0 {
		headBeforePrefix := it.head == nil || it.head.Less(it.prefix)
		if headBeforePrefix {
			// position at the location of the prefix
			// or the item that should follow
			index, _ = it.store.indexOf(it.prefix)
		}
	} else if it.lastIndex < len(it.store.Items) {
		// check whether something has changed in between
		last := &it.store.Items[it.lastIndex]
		hasPrefix := it.isPrefix && bytes.HasPrefix(last.Key, it.head)
		isSameKey := !it.isPrefix && last.Key.Equal(it.head)
		if hasPrefix || isSameKey {
			index = it.lastIndex + 1
		}
	}

	// default handling position to item after head
	if index < 0 {
		index, found = it.store.indexOf(it.head)
		if found {
			index++
		}
	}

	// skip all other with the same prefix
	if it.isPrefix {
		for ; index < len(it.store.Items); index++ {
			if !bytes.HasPrefix(it.store.Items[index].Key, it.head) {
				break
			}
		}
	}

	// save last index to avoid binary search on Next
	it.lastIndex = index

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
			// set head to prefix (including the trailing delimiter)
			it.isPrefix = true
			it.head = it.head[:len(it.prefix)+i+1]
			break
		}
	}

	// update the value
	item.Key = append(item.Key[:0], it.head...)
	if !it.isPrefix {
		item.Value = append(item.Value[:0], it.store.Items[index].Value...)
	} else {
		item.Value = nil
	}
	item.IsPrefix = it.isPrefix

	return true
}

func (it *iterator) cleanup() {
	it.store = nil
	it.head = nil
	it.isPrefix = false
}

func (it *iterator) Close() error {
	it.cleanup()
	return nil
}
