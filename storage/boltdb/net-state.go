// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"encoding/json"
	"log"

	"github.com/boltdb/bolt"
)

// File Path and Value are saved to boltdb
type File struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

const (
	fileBucketName = "files"
)

var (
	errCreatingFileBucket = Error.New("error creating file bucket")
	errFileNotFound       = Error.New("error file not found")
	errIterKeys           = Error.New("error unable to iterate through bucket keys")
	errDeletingFile       = Error.New("error unable to delete file key")
)

func (client *Client) Put(file File) error {
	return client.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(fileBucketName))
		if err != nil {
			return errCreatingFileBucket
		}

		fileKey := []byte(file.Path)

		fileBytes, err := json.Marshal(file.Value)
		if err != nil {
			log.Println(err)
		}

		return b.Put(fileKey, fileBytes)
	})
}

func (client *Client) Get(fileKey []byte) (File, error) {
	var fileInfo File
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(fileBucketName))
		v := b.Get(fileKey)
		if v == nil {
			return errFileNotFound
		}
		unmarshalErr := json.Unmarshal(v, &fileInfo.Value)
		return unmarshalErr
	})

	fileInfo.Path = string(fileKey)
	return fileInfo, err
}

func (client *Client) List() ([]string, error) {
	var paths []string
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(fileBucketName))

		err := b.ForEach(func(key, value []byte) error {
			paths = append(paths, string(key))
			return nil
		})
		if err != nil {
			return errIterKeys
		}
		return nil
	})

	return paths, err
}

func (client *Client) Delete(fileKey []byte) error {
	if err := client.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(fileBucketName)).Delete(fileKey)
	}); err != nil {
		return errDeletingFile
	}
	return nil
}
