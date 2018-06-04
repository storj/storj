// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"os"
	"path"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/filepiece"
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

// StoreWriter stores data into piece store in multiple writes
// 	id is the id of the data to be stored
// 	dir is the pstore directory containing all other data stored
// 	returns error if failed and nil if successful
func StoreWriter(id string, length int64, psFileOffset int64, dir string) (*fpiece.Chunk, error) {
	if psFileOffset < 0 {
		return nil, ArgError.New("Offset is less than 0. Must be greater than or equal to 0")
	}

	dataPath, err := PathByID(id, dir)
	if err != nil {
		return nil, err
	}

	// Create directory path on file system
	if err = os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
		return nil, err
	}

	// Create File on file system
	dataFile, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}

	return fpiece.NewChunk(dataFile, psFileOffset, length)
}

// RetrieveReader retrieves data from pstore directory
//	id is the id of the stored data
//	length is the amount of data to read. Read all data if -1
//	readPosOffset	is the offset of the data that you are reading. Useful for multiple connections to split the data transfer
//	dir is the pstore directory containing all other data stored
// 	returns error if failed and nil if successful
func RetrieveReader(id string, length int64, readPosOffset int64, dir string) (*fpiece.Chunk, error) {
	dataPath, err := PathByID(id, dir)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(dataPath)
	if err != nil {
		return nil, err
	}

	// If offset is greater than file size return
	if readPosOffset >= fileInfo.Size() || readPosOffset < 0 {
		return nil, ArgError.New("Invalid offset: %v", readPosOffset)
	}

	// If length less than 0 read the entire file
	if length <= -1 {
		length = fileInfo.Size()
	}

	// If trying to read past the end of the file, just read to the end
	if fileInfo.Size() < readPosOffset+length {
		length = fileInfo.Size() - readPosOffset
	}

	dataFile, err := os.OpenFile(dataPath, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}

	// Created a section reader so that we can concurrently retrieve the same file.
	return fpiece.NewChunk(dataFile, readPosOffset, length)
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
