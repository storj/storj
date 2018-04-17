// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore // import "storj.io/storj/pkg/pstore"

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
)

var tmpfile string

func TestStore(t *testing.T) {
	file, err := os.Open(tmpfile)
	if err != nil {
		t.Errorf("Error opening tmp file: %s", err.Error())
		return
	}

	reader := bufio.NewReader(file)
	defer file.Close()

	hash := "0123456789ABCDEFGHIJ"
	Store(hash, reader, os.TempDir())

	folder1 := string(hash[0:2])
	folder2 := string(hash[2:4])
	fileName := string(hash[4:])

	createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)
	defer os.RemoveAll(path.Join(os.TempDir(), folder1))
	_, lStatErr := os.Lstat(createdFilePath)
	if lStatErr != nil {
		t.Errorf("No file was created from Store(): %s", lStatErr.Error())
		return
	}

	createdFile, openCreatedError := os.Open(createdFilePath)
	if openCreatedError != nil {
		t.Errorf("Error: %s opening created file %s", openCreatedError.Error(), createdFilePath)
	}
	defer createdFile.Close()

	buffer := make([]byte, 5)
	createdFile.Seek(0, 0)
	_, _ = createdFile.Read(buffer)

	if string(buffer) != "butts" {
		t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
	}
}

func TestRetrieve(t *testing.T) {
	file, err := os.Open(tmpfile)
	if err != nil {
		t.Errorf("Error opening tmp file: %s", err.Error())
		return
	}

	reader := bufio.NewReader(file)
	defer file.Close()

	hash := "0123456789ABCDEFGHIJ"
	Store(hash, reader, os.TempDir())

	// Create file for retrieving data into
	retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
	retrievalFile, retrievalFileError := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
	if retrievalFileError != nil {
		t.Errorf("Error creating file: %s", retrievalFileError.Error())
		return
	}
	defer retrievalFile.Close()

	writer := bufio.NewWriter(retrievalFile)

	retrieveErr := Retrieve(hash, writer, os.TempDir())

	if retrieveErr != nil {
		t.Errorf("Retrieve Error: %s", retrieveErr.Error())
	}

	buffer := make([]byte, 5)

	retrievalFile.Seek(0, 0)
	_, _ = retrievalFile.Read(buffer)

	fmt.Printf("Retrieved data: %s", string(buffer))

	if string(buffer) != "butts" {
		t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
	}
}

func TestDelete(t *testing.T) {
	file, err := os.Open(tmpfile)
	if err != nil {
		t.Errorf("Error opening tmp file: %s", err.Error())
		return
	}

	reader := bufio.NewReader(file)
	defer file.Close()

	hash := "0123456789ABCDEFGHIJ"
	Store(hash, reader, os.TempDir())

	folder1 := string(hash[0:2])
	folder2 := string(hash[2:4])
	fileName := string(hash[4:])

	_, existErr := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName))
	if existErr != nil {
		t.Errorf("Failed to Store test file")
		return
	}

	Delete(hash, os.TempDir())
	_, deletedExistErr := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName))
	if deletedExistErr == nil {
		t.Errorf("Failed to Delete test file")
		return
	}
}

func TestMain(m *testing.M) {
	content := []byte("butts")
	tmpfilePtr, err := ioutil.TempFile("", "api_test")
	if err != nil {
		log.Fatal(err)
	}

	// defer os.Remove(tmpfile.Name()) // clean up
	tmpfile = tmpfilePtr.Name()

	if _, err := tmpfilePtr.Write(content); err != nil {
		log.Fatal(err)
	}

	if err := tmpfilePtr.Close(); err != nil {
		log.Fatal(err)
	}

	m.Run()

	os.Exit(0)
}
