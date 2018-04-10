package boltdb

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
)

type File struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

var (
	ErrCreatingFileBucket = errors.New("error creating file bucket")
	ErrFileNotFound       = errors.New("error file not found")
	ErrIterKeys           = errors.New("error unable to iterate through bucket keys")
	ErrDeletingFile       = errors.New("error unable to delete file key")
)

func (client *Client) Put(file File) error {
	return client.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("files"))
		if err != nil {
			return ErrCreatingFileBucket
		}

		fileKey := []byte(file.Path)

		fileBytes, err := json.Marshal(file)
		if err != nil {
			log.Println(err)
		}

		return b.Put(fileKey, fileBytes)
	})
}

func (client *Client) Get(fileKey []byte) (File, error) {
	var fileInfo File
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("files"))
		v := b.Get(fileKey)
		if v == nil {
			return ErrFileNotFound
		} else {
			unmarshalErr := json.Unmarshal(v, &fileInfo)
			return unmarshalErr
		}
	})

	return fileInfo, err
}

func (client *Client) List(bucketName []byte) ([]string, error) {
	var paths []string
	err := client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		err := b.ForEach(func(key, value []byte) error {
			paths = append(paths, string(key))
			return nil
		})
		if err != nil {
			return ErrIterKeys
		}
		return nil
	})

	return paths, err
}

func (client *Client) Delete(fileKey []byte) error {
	if err := client.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("files")).Delete(fileKey)
	}); err != nil {
		return ErrDeletingFile
	}
	return nil
}
