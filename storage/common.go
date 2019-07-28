// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"bytes"
	"context"
	"errors"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var mon = monkit.Package()

// Delimiter separates nested paths in storage
const Delimiter = '/'

//ErrKeyNotFound used when something doesn't exist
var ErrKeyNotFound = errs.Class("key not found")

// ErrEmptyKey is returned when an empty key is used in Put or in CompareAndSwap
var ErrEmptyKey = errs.Class("empty key")

// ErrValueChanged is returned when the current value of the key does not match the oldValue in CompareAndSwap
var ErrValueChanged = errs.Class("value changed")

// ErrEmptyQueue is returned when attempting to Dequeue from an empty queue
var ErrEmptyQueue = errs.Class("empty queue")

// ErrLimitExceeded is returned when request limit is exceeded
var ErrLimitExceeded = errors.New("limit exceeded")

// Key is the type for the keys in a `KeyValueStore`
type Key []byte

// Value is the type for the values in a `ValueValueStore`
type Value []byte

// Keys is the type for a slice of keys in a `KeyValueStore`
type Keys []Key

// Values is the type for a slice of Values in a `KeyValueStore`
type Values []Value

// Items keeps all ListItem
type Items []ListItem

// LookupLimit is enforced by storage implementations
const LookupLimit = 1000

// ListItem returns Key, Value, IsPrefix
type ListItem struct {
	Key      Key
	Value    Value
	IsPrefix bool
}

// KeyValueStore describes key/value stores like redis and boltdb
type KeyValueStore interface {
	// Put adds a value to store
	Put(context.Context, Key, Value) error
	// Get gets a value to store
	Get(context.Context, Key) (Value, error)
	// GetAll gets all values from the store
	GetAll(context.Context, Keys) (Values, error)
	// Delete deletes key and the value
	Delete(context.Context, Key) error
	// List lists all keys starting from start and upto limit items
	List(ctx context.Context, start Key, limit int) (Keys, error)
	// Iterate iterates over items based on opts
	Iterate(ctx context.Context, opts IterateOptions, fn func(context.Context, Iterator) error) error
	// CompareAndSwap atomically compares and swaps oldValue with newValue
	CompareAndSwap(ctx context.Context, key Key, oldValue, newValue Value) error
	// Close closes the store
	Close() error
}

// IterateOptions contains options for iterator
type IterateOptions struct {
	// Prefix ensure
	Prefix Key
	// First will be the first item iterator returns or the next item (previous when reverse)
	First Key
	// Recurse, do not collapse items based on Delimiter
	Recurse bool
	// Reverse iterates in reverse order
	Reverse bool
}

// Iterator iterates over a sequence of ListItems
type Iterator interface {
	// Next prepares the next list item.
	// It returns true on success, or false if there is no next result row or an error happened while preparing it.
	Next(ctx context.Context, item *ListItem) bool
}

// IsZero returns true if the value struct is it's zero value
func (value Value) IsZero() bool {
	return len(value) == 0
}

// IsZero returns true if the key struct is it's zero value
func (key Key) IsZero() bool {
	return len(key) == 0
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Value type
func (value Value) MarshalBinary() ([]byte, error) {
	return value, nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface for the Key type
func (key Key) MarshalBinary() ([]byte, error) {
	return key, nil
}

// ByteSlices converts a `Keys` struct to a slice of byte-slices (i.e. `[][]byte`)
func (keys Keys) ByteSlices() [][]byte {
	result := make([][]byte, len(keys))

	for key, val := range keys {
		result[key] = []byte(val)
	}

	return result
}

// String implements the Stringer interface
func (key Key) String() string { return string(key) }

// Strings returns everything as strings
func (keys Keys) Strings() []string {
	strs := make([]string, 0, len(keys))
	for _, key := range keys {
		strs = append(strs, string(key))
	}
	return strs
}

// GetKeys gets all the Keys in []ListItem and converts them to Keys
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

// Less returns whether item should be sorted before b
func (item ListItem) Less(b ListItem) bool { return item.Key.Less(b.Key) }

// Less returns whether key should be sorted before b
func (key Key) Less(b Key) bool { return bytes.Compare([]byte(key), []byte(b)) < 0 }

// Equal returns whether key and b are equal
func (key Key) Equal(b Key) bool { return bytes.Equal([]byte(key), []byte(b)) }
