// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/aleitner/FilePiece"
	"github.com/zeebo/errs"
)

// Errors
var (
	ArgError = errs.Class("argError")
	FSError  = errs.Class("fsError")
)

// creates datapath from hash and dir
func pathByHash(hash, dir string) (string, error) {
	if len(hash) < 20 {
		return "", ArgError.New("Invalid hash length")
	}

	folder1 := string(hash[0:2])
	folder2 := string(hash[2:4])
	fileName := string(hash[4:])

	return path.Join(dir, folder1, folder2, fileName), nil
}

/*
	Store

	Store data into piece store

	hash 		(string)				Hash of the data to be stored
	r 			(io.Reader)	        File/Stream that contains the contents of the data to be stored
	length 	(length)				Size of the data to be stored
	psFileOffset 	(offset)  Offset of the data that you are writing. Useful for multiple connections to split the data transfer
	dir 		(string)				pstore directory containing all other data stored
	returns (error) if 		  failed and nil if successful
*/
func Store(hash string, r io.Reader, length int64, psFileOffset int64, dir string) error {
	if psFileOffset < 0 {
		return ArgError.New("Offset is less than 0. Must be greater than or equal to 0")
	}
	if length < 0 {
		return ArgError.New("Length is less than 0. Must be greater than or equal to 0")
	}
	if dir == "" {
		return ArgError.New("No path provided")
	}

	dataPath, err := pathByHash(hash, dir)
	if err != nil {
		return err
	}

	// Create directory path on file system
	if err = os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
		return err
	}

	// Create File on file system
	dataFile, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}

	dataFileSection := fpiece.NewChunk(dataFile, psFileOffset, length)

	// Close when finished
	defer dataFile.Close()

	buffer := make([]byte, 4096)
  for {
    // Read data from read stream into buffer
    n, err := r.Read(buffer)
    if err == io.EOF {
      break
    }

    // Write the buffer to the stream we opened earlier
    _, err = dataFileSection.Write(buffer[:n])
		if err == io.EOF {
			break
		} else if (err != nil) {
			return err
		}
  }

	return nil
}

/*
	Retrieve

	Retrieve data from pstore directory

	hash 					(string)		   Hash of the stored data
	w 						(io.Writer)	   Stream that recieves the stored data
	length 				(length)		   Amount of data to read. Read all data if -1
	readPosOffset	(offset)	   	 Offset of the data that you are reading. Useful for multiple connections to split the data transfer
	dir 					(string)		   pstore directory containing all other data stored
	returns 			(int64, error) returns err if failed and the number of bytes retrieved if successful
*/
func Retrieve(hash string, w io.Writer, length int64, readPosOffset int64, dir string) (int64, error) {
	if dir == "" {
		return 0, ArgError.New("No path provided")
	}

	dataPath, err := pathByHash(hash, dir)
	if err != nil {
		return 0, err
	}

	fileInfo, err := os.Stat(dataPath)
	if err != nil {
		return 0, err
	}

	// If offset is greater than file size return
	if readPosOffset >= fileInfo.Size() || readPosOffset < 0 {
		return 0, ArgError.New("Invalid offset: %v", readPosOffset)
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
		return 0, err
	}
	// Close when finished
	defer dataFile.Close()

	// Created a section reader so that we can concurrently retrieve the same file.
	dataFileSection := io.NewSectionReader(dataFile, readPosOffset, length)

	var total int64 = 0
	buffer := make([]byte, 4096)
  for {
    // Read data from read stream into buffer
    n, err := dataFileSection.Read(buffer)
    if err == io.EOF {
      return total, io.EOF
    }

    // Write the buffer to the stream we opened earlier
    n, err = w.Write(buffer[:n])

		total += int64(n)

		if err != nil {

			return 0, err
		}
  }
}

/*
	Delete

	Delete data from farmer

	hash (string) Hash of the data to be stored
	dir (string) pstore directory containing all other data stored
	returns (error) if failed and nil if successful
*/
func Delete(hash string, dir string) error {
	if dir == "" {
		return ArgError.New("No path provided")
	}

	dataPath, err := pathByHash(hash, dir)
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
