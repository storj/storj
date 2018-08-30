// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"bytes"
	"errors"

	"github.com/zeebo/errs"
)

//ErrKeyNotFound used When something doesn't exist
var ErrKeyNotFound = errs.Class("key not found")

var ErrEmptyKey = errors.New("empty key")

// Key is the type for the keys in a `KeyValueStore`
type Key []byte

// Value is the type for the values in a `ValueValueStore`
type Value []byte

// Keys is the type for a slice of keys in a `KeyValueStore`
type Keys []Key

// Values is the type for a slice of Values in a `KeyValueStore`
type Values []Value

// Limit indicates how many keys to return when calling List
type Limit int

// More indicates if the result was truncated. If false
// then the result []ListItem includes all requested keys.
// If true then the caller must call List again to get more
// results by setting `StartAfter` or `EndBefore` appropriately.
type More bool

// ListOptions are items that are optional for the LIST method
type ListOptions struct {
	Prefix       Key
	StartAfter   Key
	EndBefore    Key
	Recursive    bool
	IncludeValue bool
	Limit        Limit
}

// Items keeps all ListItem
type Items []ListItem

// ListItem returns Key, Value, IsPrefix
type ListItem struct {
	Key      Key
	Value    Value
	IsPrefix bool
}

// KeyValueStore is an interface describing key/value stores like redis and boltdb
type KeyValueStore interface {
	// Put adds a value to store
	Put(Key, Value) error
	// Get gets a value to store
	Get(Key) (Value, error)
	// GetAll gets all values from the store
	GetAll(Keys) (Values, error)
	// Delete deletes key and the value
	Delete(Key) error
	// List lists all keys starting from start and upto limit items
	List(start Key, limit Limit) (Keys, error)
	// ListV2 lists all keys corresponding to ListOptions
	ListV2(opts ListOptions) (Items, More, error)
	// ReverseList lists all keys in revers order
	ReverseList(Key, Limit) (Keys, error)
	// Close closes the store
	Close() error
}

type IterableStore interface {
	KeyValueStore
	// TODO: figure out whether to use after or first?
	// Iterate iterates items skipping nested prefixes
	Iterate(prefix, after Key, delimiter byte, fn func(it Iterator) error) error
	// IterateAll iterates everything
	// IterateAll(prefix, after Key) Iterator
}

type Iterator interface {
	// Next prepares the next list item
	Next(item *ListItem) bool
}

// IsZero returns true if the value struct is it's zero value
func (v *Value) IsZero() (_ bool) {
	return len(*v) == 0
}

// IsZero returns true if the key struct is it's zero value
func (k *Key) IsZero() (_ bool) {
	return len(*k) == 0
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Value type
func (v *Value) MarshalBinary() (_ []byte, _ error) {
	return *v, nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Key type
func (k *Key) MarshalBinary() (_ []byte, _ error) {
	return *k, nil
}

// ByteSlices converts a `Keys` struct to a slice of byte-slices (i.e. `[][]byte`)
func (k *Keys) ByteSlices() [][]byte {
	result := make([][]byte, len(*k))

	for _k, v := range *k {
		result[_k] = []byte(v)
	}

	return result
}

// String implements the Stringer interface
func (k *Key) String() string {
	return string(*k)
}

// GetKeys gets all the Keys in []ListItem and converts them to Keys
func (i *Items) GetKeys() Keys {
	if len(*i) == 0 {
		return nil
	}
	var keys Keys
	for _, item := range *i {
		keys = append(keys, item.Key)
	}
	return keys
}

// Len is the number of elements in the collection.
func (items Items) Len() int { return len(items) }

// Less reports whether the element with
// index i should sort before the element with index j.
func (items Items) Less(i, k int) bool { return items[i].Less(items[k]) }

// Swap swaps the elements with indexes i and j.
func (items Items) Swap(i, k int) { items[i], items[k] = items[k], items[i] }

// Less returns whether a should be sorted before b
func (a ListItem) Less(b ListItem) bool { return a.Key.Less(b.Key) }

// Less returns whether a should be sorted before b
func (a Key) Less(b Key) bool { return bytes.Compare([]byte(a), []byte(b)) < 0 }

// Equal returns whether a and b are equal
func (a Key) Equal(b Key) bool { return bytes.Equal([]byte(a), []byte(b)) }
