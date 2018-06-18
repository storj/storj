// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"crypto/rand"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/mr-tron/base58/base58"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/ranger"
)

// IDLength -- Minimum ID length
const IDLength = 20

// Errors
var (
	ArgError = errs.Class("argError")
	FSError  = errs.Class("fsError")
)

// PathByID creates datapath from id and dir
func PathByID(id, dir string) (string, error) {
	if len(id) < IDLength {
		return "", ArgError.New("Invalid id length")
	}
	if dir == "" {
		return "", ArgError.New("No path provided")
	}

	folder1 := string(id[0:2])
	folder2 := string(id[2:4])
	fileName := string(id[4:])

	return path.Join(dir, folder1, folder2, fileName), nil
}

// DetermineID creates random id
func DetermineID() string {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return base58.Encode(b)
}

// StoreWriter stores data into piece store in multiple writes
// 	id is the id of the data to be stored
// 	dir is the pstore directory containing all other data stored
// 	returns error if failed and nil if successful
func StoreWriter(id string, dir string) (io.WriteCloser, error) {
	dataPath, err := PathByID(id, dir)
	if err != nil {
		return nil, err
	}

	// Create directory path on file system
	if err = os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
		return nil, err
	}

	// Create File on file system
	return os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0755)
}

// RetrieveReader retrieves data from pstore directory
//	id is the id of the stored data
//	offset	is the offset of the data that you are reading. Useful for multiple connections to split the data transfer
//	length is the amount of data to read. Read all data if -1
//	dir is the pstore directory containing all other data stored
// 	returns error if failed and nil if successful
func RetrieveReader(id string, offset int64, length int64, dir string) (io.ReadCloser, error) {
	dataPath, err := PathByID(id, dir)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(dataPath)
	if err != nil {
		return nil, err
	}

	// If offset is greater than file size return
	if offset >= fileInfo.Size() || offset < 0 {
		return nil, ArgError.New("Invalid offset: %v", offset)
	}

	// If length less than 0 read the entire file
	if length <= -1 {
		length = fileInfo.Size()
	}

	// If trying to read past the end of the file, just read to the end
	if fileInfo.Size() < offset+length {
		length = fileInfo.Size() - offset
	}

	dataFile, err := os.OpenFile(dataPath, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}

	// Created a section reader so that we can concurrently retrieve the same file.
	rr, err := ranger.FileHandleRanger(dataFile)
	if err != nil {
		return nil, err
	}

	return rr.Range(offset, length), nil
}

// Delete deletes data from farmer
//	id is the id of the data to be stored
//	dir is the pstore directory containing all other data stored
//	returns error if failed and nil if successful
func Delete(id string, dir string) error {
	dataPath, err := PathByID(id, dir)
	if err != nil {
		return err
	}

	if _, err = os.Stat(dataPath); os.IsNotExist(err) {
		return nil
	}

	if err = os.Remove(dataPath); err != nil {
		return err
	}

	return nil
}
