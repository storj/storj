// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
)

// Progress -- shard state upload/download progress
type Progress int

const (
	// Awaiting -- shard has not begun upload/download
	Awaiting Progress = 0
	// InProgress -- shard has begun upload/download
	InProgress Progress = 1
	// Complete -- shard has completed upload/download
	Complete Progress = 2
	// Failed -- shard has failed upload/download
	Failed Progress = 3
)

// Shard -- struct containing meta about a shard
type Shard struct {
	N         int
	Hash      string
	Offset    int64
	Size      int64
	Locations []string
	Progress  Progress
}

// FileMetaData -- struct containing meta about a file
type FileMetaData struct {
	Size          int64
	Hash          string
	TotalShards   int
	AvgShardSize  int64
	TailShardSize int64
	Shards        []Shard
	Progress      Progress
}

// State -- struct containing all meta for upload/download
type State struct {
	Blacklist []string
	FileMeta  *FileMetaData
	File      *os.File
	FilePath  string
}

// StringInSlice - check if string (a) is in (list)
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// StorjPath -- print the path where relevant meta files are being stored
func StorjPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(usr.HomeDir, ".storj/", "files/"), nil
}

// SaveProgress -- Save data map
func SaveProgress(fileMeta FileMetaData) error {
	storjDir, err := StorjPath()
	if err != nil {
		return err
	}

	err = os.MkdirAll(storjDir, os.ModePerm)
	if err != nil {
		return err
	}

	metadataPath := filepath.Join(storjDir, fileMeta.Hash)

	file, err := os.Create(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	gob.Register(Shard{})
	err = encoder.Encode(&fileMeta)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", fileMeta)

	return nil
}

// LoadProgress -- Load map of data by hash
func LoadProgress(hash string, fileMeta *FileMetaData) error {
	storjDir, err := StorjPath()
	if err != nil {
		return err
	}

	metadataPath := filepath.Join(storjDir, hash)

	file, err := os.Open(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Seek(0, 0)
	decoder := gob.NewDecoder(file)
	gob.Register(Shard{})
	err = decoder.Decode(fileMeta)
	if err != nil {
		return err
	}

	return nil
}

// ListFiles -- List known hash maps
func ListFiles() error {
	dir, err := StorjPath()
	if err != nil {
		return err
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", dir, err)
			return err
		}
		if info.IsDir() {
			return nil
		}

		fmt.Printf("%s\n", filepath.Base(path))
		return nil
	})

	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", dir, err)
	}

	return nil
}

// DetermineHash -- Get the hash for a section of data
func DetermineHash(f *os.File, offset int64, length int64) (string, error) {
	h := md5.New()

	fSection := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, fSection); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
