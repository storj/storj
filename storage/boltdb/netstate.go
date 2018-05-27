// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"github.com/boltdb/bolt"
)

// PointerEntry - Path and Pointer are saved as a kv pair to boltdb.
// The following boltdb methods handle the pointer type (defined in
// the protobuf file) after it has been marshalled into bytes.
type PointerEntry struct {
	Path    []byte
	Pointer []byte
}

const (
	pointerBucket = "pointers"
)

// Put saves the Path and Pointer as a kv entry in the "pointers" bucket
func (client *Client) Put(pe PointerEntry) error {
	client.logger.Debug("entering bolt put")
	return client.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(pointerBucket))
		if err != nil {
			return err
		}

		return b.Put(pe.Path, pe.Pointer)
	})
}

// Get retrieves the Pointer value stored at the Path key
func (client *Client) Get(pathKey []byte) ([]byte, error) {
	client.logger.Debug("entering bolt get: " + string(pathKey))
	var pointerBytes []byte
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(pointerBucket))
		v := b.Get(pathKey)
		if v == nil {
			return Error.New("pointer at %#v not found", string(pathKey))
		}
		pointerBytes = v
		return nil
	})

	return pointerBytes, err
}

// List creates a byte array of all path keys in in the "pointers" bucket
func (client *Client) List() ([][]byte, error) {
	client.logger.Debug("entering bolt list")
	var paths [][]byte
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(pointerBucket))

		err := b.ForEach(func(key, value []byte) error {
			paths = append(paths, key)
			return nil
		})
		return err
	})

	return paths, err
}

// Delete deletes a kv pair from the "pointers" bucket, given the Path key
func (client *Client) Delete(pathKey []byte) error {
	client.logger.Debug("entering bolt delete: " + string(pathKey))
	return client.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(pointerBucket)).Delete(pathKey)
	})
}
