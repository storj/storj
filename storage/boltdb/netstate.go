// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"github.com/boltdb/bolt"
)

// File Path and Value are saved to boltdb
type File struct {
	Path  string `json:"path"`
	Value []byte `json:"value"`
}

const (
	fileBucketName = "files"
)

// Put saves the file path and value as a kv pair in the "files" bucket
func (client *Client) Put(file File) error {
	client.logger.Debug("entering Client.Put(File)")
	return client.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(fileBucketName))
		if err != nil {
			return err
		}

		fileKey := []byte(file.Path)
		return b.Put(fileKey, file.Value)
	})
}

// Get retrieves the value stored at the file path key
func (client *Client) Get(fileKey []byte) (File, error) {
	client.logger.Debug("entering Client.Get(fileKey)")
	var fileInfo File
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(fileBucketName))
		v := b.Get(fileKey)
		if v == nil {
			return Error.New("file %#v not found", string(fileKey))
		}
		fileInfo.Value = v
		return nil
	})

	fileInfo.Path = string(fileKey)
	return fileInfo, err
}

// List creates a string array of all keys in in the "files" bucket
func (client *Client) List() ([]string, error) {
	client.logger.Debug("entering Client.List()")
	var paths []string
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(fileBucketName))

		err := b.ForEach(func(key, value []byte) error {
			paths = append(paths, string(key))
			return nil
		})
		return err
	})

	return paths, err
}

// Delete deletes a kv pair from the "files" bucket, given the key
func (client *Client) Delete(fileKey []byte) error {
	client.logger.Debug("entering Client.Delete(fileKey)")
	return client.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(fileBucketName)).Delete(fileKey)
	})
}
