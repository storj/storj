// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvstore

import (
	"bytes"
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var mon = monkit.Package()

// Delimiter separates nested paths in storage.
const Delimiter = '/'

var (
	// ErrKeyNotFound used when something doesn't exist.
	ErrKeyNotFound = errs.Class("key not found")

	// ErrEmptyKey is returned when an empty key is used in Put or in CompareAndSwap.
	ErrEmptyKey = errs.Class("empty key")

	// ErrValueChanged is returned when the current value of the key does not match the old value in CompareAndSwap.
	ErrValueChanged = errs.Class("value changed")
)

// Key is the type for the keys in a `Store`.
type Key []byte

// Value is the type for the values in a `ValueValueStore`.
type Value []byte

// Keys is the type for a slice of keys in a `Store`.
type Keys []Key

// Values is the type for a slice of Values in a `Store`.
type Values []Value

// Items keeps all Item.
type Items []Item

// Item returns Key, Value, IsPrefix.
type Item struct {
	Key      Key
	Value    Value
	IsPrefix bool
}

// Store describes key/value stores like redis and boltdb.
type Store interface {
	// Put adds a value to store.
	Put(context.Context, Key, Value) error
	// Get gets a value to store.
	Get(context.Context, Key) (Value, error)
	// Delete deletes key and the value.
	Delete(context.Context, Key) error
	// Range iterates over all items in unspecified order.
	// The Key and Value are valid only for the duration of callback.
	Range(ctx context.Context, fn func(context.Context, Key, Value) error) error
	// CompareAndSwap atomically compares and swaps oldValue with newValue.
	CompareAndSwap(ctx context.Context, key Key, oldValue, newValue Value) error
	// Close closes the store.
	Close() error
}

// IsZero returns true if the value struct is a zero value.
func (value Value) IsZero() bool {
	return len(value) == 0
}

// IsZero returns true if the key struct is a zero value.
func (key Key) IsZero() bool {
	return len(key) == 0
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Value type.
func (value Value) MarshalBinary() ([]byte, error) {
	return value, nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Key type.
func (key Key) MarshalBinary() ([]byte, error) {
	return key, nil
}

// ByteSlices converts a `Keys` struct to a slice of byte-slices (i.e. `[][]byte`).
func (keys Keys) ByteSlices() [][]byte {
	result := make([][]byte, len(keys))

	for key, val := range keys {
		result[key] = []byte(val)
	}

	return result
}

// String implements the Stringer interface.
func (key Key) String() string { return string(key) }

// Strings returns everything as strings.
func (keys Keys) Strings() []string {
	strs := make([]string, 0, len(keys))
	for _, key := range keys {
		strs = append(strs, string(key))
	}
	return strs
}

// GetKeys gets all the Keys in []Item and converts them to Keys.
func (items Items) GetKeys() Keys {
	if len(items) == 0 {
		return nil
	}
	var keys Keys
	for _, item := range items {
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

// Less returns whether item should be sorted before b.
func (item Item) Less(b Item) bool { return item.Key.Less(b.Key) }

// Less returns whether key should be sorted before b.
func (key Key) Less(b Key) bool { return bytes.Compare([]byte(key), []byte(b)) < 0 }

// Equal returns whether key and b are equal.
func (key Key) Equal(b Key) bool { return bytes.Equal([]byte(key), []byte(b)) }
