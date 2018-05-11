package main

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func storjPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(usr.HomeDir, ".storj/", "files/"), nil
}

// Save data map
func saveProgress(fileMeta fileMetaData) error {
	storjDir, err := storjPath()
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
	gob.Register(shard{})
	err = encoder.Encode(&fileMeta)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", fileMeta)

	return nil
}

// Load map of data by has
func loadProgress(hash string, fileMeta *fileMetaData) error {
	storjDir, err := storjPath()
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
	gob.Register(shard{})
	err = decoder.Decode(fileMeta)
	if err != nil {
		return err
	}

	return nil
}

// List known hash maps
func listFiles() error {
	dir, err := storjPath()
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

// Get the hash for a section of data
func determineHash(f *os.File, offset int64, length int64) (string, error) {
	h := md5.New()

	fSection := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, fSection); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
