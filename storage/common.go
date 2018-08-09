// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

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

// KeyValueStore is an interface describing key/value stores like redis and boltdb
type KeyValueStore interface {
	// Put adds a value to the provided key in the KeyValueStore, returning an error on failure.
	Put(Key, Value) error
	Get(Key) (Value, error)
	GetAll(Keys) (Values, error)
	List(Key, Limit) (Keys, error)
	ReverseList(Key, Limit) (Keys, error)
	Delete(Key) error
	Close() error
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
