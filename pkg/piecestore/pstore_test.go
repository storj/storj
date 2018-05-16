// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

var tmpfile string

func TestStore(t *testing.T) {
	t.Run("it stores data successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)

		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

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
	})

	t.Run("it stores data by offset successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 2, os.TempDir())

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

		buffer := make([]byte, 7)
		createdFile.Seek(0, 0)
		_, _ = createdFile.Read(buffer)

		// \0\0butts
		expected := []byte{0, 0, 98, 117, 116, 116, 115}

		if string(buffer) != string(expected) {
			t.Errorf("Expected data (%v) does not equal Actual data (%v)", expected, buffer)
		}
	})

	t.Run("it stores data by chunk successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, 4, 0, os.TempDir())

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

		buffer := make([]byte, 4)
		createdFile.Seek(0, 0)
		_, _ = createdFile.Read(buffer)

		// butt
		expected := []byte{98, 117, 116, 116}

		if string(buffer) != string(expected) {
			t.Errorf("Expected data %s does not equal Actual data %s", string(expected), string(buffer))
		}
	})

	t.Run("it should return hash err if the hash is too short", func(t *testing.T) {
		assert := assert.New(t)
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "11111111"

		_, err = Store(hash, file, 5, 0, os.TempDir())
		assert.NotNil(err)
		if err != nil {
			assert.Equal(err.Error(), "argError: Invalid hash length", "They should have the same error message")
		}
	})

	// Test passing in negative offset
	t.Run("it should return an error when given negative offset", func(t *testing.T) {
		assert := assert.New(t)
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"

		_, err = Store(hash, file, int64(fi.Size()), -12, os.TempDir())

		assert.NotNil(err)
		if err != nil {
			assert.Equal(err.Error(), "argError: Offset is less than 0. Must be greater than or equal to 0", err.Error())
		}
	})

	// Test passing in a negative length
	t.Run("it should return an error when given length less than 0", func(t *testing.T) {
		assert := assert.New(t)
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		_, err = file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"

		_, err = Store(hash, file, -1, 0, os.TempDir())
		assert.NotNil(err)
		if err != nil {
			assert.Equal(err.Error(), "argError: Length is less than 0. Must be greater than or equal to 0", err.Error())
		}
	})
}

func TestRetrieve(t *testing.T) {
	t.Run("it retrieves data successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}
		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		// Create file for retrieving data into
		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			t.Errorf("Error creating file: %s", err.Error())
			return
		}
		defer os.RemoveAll(retrievalFilePath)
		defer retrievalFile.Close()

		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), 0, os.TempDir())

		if err != nil {
			if err != io.EOF {
				t.Errorf("Retrieve Error: %s", err.Error())
			}
		}

		buffer := make([]byte, 5)

		retrievalFile.Seek(0, 0)
		_, _ = retrievalFile.Read(buffer)

		fmt.Printf("Retrieved data: %s", string(buffer))

		if string(buffer) != "butts" {
			t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
		}
	})

	t.Run("it retrieves data by offset successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		// Create file for retrieving data into
		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			t.Errorf("Error creating file: %s", err.Error())
			return
		}
		defer os.RemoveAll(retrievalFilePath)
		defer retrievalFile.Close()

		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), 2, os.TempDir())

		if err != nil {
			if err != io.EOF {
				t.Errorf("Retrieve Error: %s", err.Error())
			}
		}

		buffer := make([]byte, 3)

		retrievalFile.Seek(0, 0)
		_, _ = retrievalFile.Read(buffer)

		fmt.Printf("Retrieved data: %s", string(buffer))

		if string(buffer) != "tts" {
			t.Errorf("Expected data (tts) does not equal Actual data (%s)", string(buffer))
		}
	})

	t.Run("it retrieves data by chunk successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		// Create file for retrieving data into
		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			t.Errorf("Error creating file: %s", err.Error())
			return
		}
		defer os.RemoveAll(retrievalFilePath)
		defer retrievalFile.Close()

		_, err = Retrieve(hash, retrievalFile, 3, 0, os.TempDir())

		if err != nil {
			if err != io.EOF {
				t.Errorf("Retrieve Error: %s", err.Error())
			}
		}

		buffer := make([]byte, 3)

		retrievalFile.Seek(0, 0)
		_, _ = retrievalFile.Read(buffer)

		fmt.Printf("Retrieved data: %s", string(buffer))

		if string(buffer) != "but" {
			t.Errorf("Expected data (but) does not equal Actual data (%s)", string(buffer))
		}
	})

	// Test passing in negative offset
	t.Run("it should return an error when retrieving with offset less 0", func(t *testing.T) {
		assert := assert.New(t)
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		// Create file for retrieving data into
		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			t.Errorf("Error creating file: %s", err.Error())
			return
		}
		defer os.RemoveAll(retrievalFilePath)
		defer retrievalFile.Close()

		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), -1, os.TempDir())
		assert.NotNil(err)
		if err != nil {
			assert.Equal("argError: Invalid offset: -1", err.Error(), err.Error())
		}
	})

	// Test passing in negative length
	t.Run("it should return the entire file successfully when retrieving with negative length", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		// Create file for retrieving data into
		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			t.Errorf("Error creating file: %s", err.Error())
			return
		}
		defer os.RemoveAll(retrievalFilePath)
		defer retrievalFile.Close()

		_, err = Retrieve(hash, retrievalFile, -1, 0, os.TempDir())

		if err != nil {
			if err != io.EOF {
				t.Errorf("Retrieve Error: %s", err.Error())
			}
		}

		buffer := make([]byte, 5)

		retrievalFile.Seek(0, 0)
		_, _ = retrievalFile.Read(buffer)

		fmt.Printf("Retrieved data: %s", string(buffer))

		if string(buffer) != "butts" {
			t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
		}
	})

}

func TestDelete(t *testing.T) {
	t.Run("it deletes data successfully", func(t *testing.T) {
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		folder1 := string(hash[0:2])
		folder2 := string(hash[2:4])
		fileName := string(hash[4:])

		if _, err := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName)); err != nil {
			t.Errorf("Failed to Store test file")
			return
		}

		Delete(hash, os.TempDir())
		_, err = os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName))
		if err == nil {
			t.Errorf("Failed to Delete test file")
			return
		}
	})

	// Test passing in a hash that doesn't exist
	t.Run("it returns an error if hash doesn't exist", func(t *testing.T) {
		assert := assert.New(t)
		file, err := os.Open(tmpfile)
		if err != nil {
			t.Errorf("Error opening tmp file: %s", err.Error())
			return
		}

		fi, err := file.Stat()
		if err != nil {
			t.Errorf("Could not stat test file: %s", err.Error())
			return
		}

		defer file.Close()

		hash := "0123456789ABCDEFGHIJ"
		Store(hash, file, int64(fi.Size()), 0, os.TempDir())

		folder1 := string(hash[0:2])
		folder2 := string(hash[2:4])
		fileName := string(hash[4:])

		if _, err := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName)); err != nil {
			t.Errorf("Failed to Store test file")
			return
		}

		falseHash := ""

		err = Delete(falseHash, os.TempDir())
		assert.NotNil(err)
		if err != nil {
			assert.NotEqual(err.Error(), "argError: Hash folder does not exist", "They should be equal")
		}
	})
}

func TestMain(m *testing.M) {
	content := []byte("butts")
	tmpfilePtr, err := ioutil.TempFile("", "api_test")
	if err != nil {
		log.Fatal(err)
	}

	tmpfile = tmpfilePtr.Name()
	defer os.Remove(tmpfile) // clean up

	if _, err := tmpfilePtr.Write(content); err != nil {
		log.Fatal(err)
	}

	if err := tmpfilePtr.Close(); err != nil {
		log.Fatal(err)
	}

	m.Run()
}
