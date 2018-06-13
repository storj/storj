// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

// Key is the type for the keys in a `KeyValueStore`
type Key []byte

// Value is the type for the values in a `ValueValueStore`
type Value []byte

// Keys is the type for a slice of keys in a `KeyValueStore`
type Keys []Key

// KeyValueStore is an interface describing key/value stores like redis and boltdb
type KeyValueStore interface {
	// Put adds a value to the provided key in the KeyValueStore, returning an error on failure.
	Put(Key, Value) error
	Get(Key) (Value, error)
	List() (Keys, error)
	Delete(Key) error
	Close() error
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
