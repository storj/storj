// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	// "fmt"
	// "io"
	// "io/ioutil"
	// "log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
  tests := []struct{
		it string
		hash string
		size int64
		offset int64
		content []byte
		expectedContent []byte
    err string
  } {
	    {
				it: "should successfully store data",
				hash: "0123456789ABCDEFGHIJ",
				size: 5,
				offset: 0,
				content: []byte("butts"),
				expectedContent: []byte("butts"),
	      err: "",
	    },
			{
				it: "should successfully store data by offset",
				hash: "0123456789ABCDEFGHIJ",
				size: 5,
				offset: 5,
				content: []byte("butts"),
				expectedContent: []byte("butts"),
				err: "",
			},
			{
				it: "should successfully store data by chunk",
				hash: "0123456789ABCDEFGHIJ",
				size: 2,
				offset: 3,
				content: []byte("butts"),
				expectedContent: []byte("bu"),
				err: "",
			},
			{
				it: "should return an error when given an invalid hash",
				hash: "012",
				size: 5,
				offset: 0,
				content: []byte("butts"),
				expectedContent: []byte("butts"),
				err: "argError: Invalid hash length",
			},
			{
				it: "should return an error when given negative offset",
				hash: "0123456789ABCDEFGHIJ",
				size: 5,
				offset: -1,
				content: []byte("butts"),
				expectedContent: []byte(""),
				err: "argError: Offset is less than 0. Must be greater than or equal to 0",
			},
			{
				it: "should return an error when given negative length",
				hash: "0123456789ABCDEFGHIJ",
				size: -1,
				offset: 0,
				content: []byte("butts"),
				expectedContent: []byte(""),
				err: "Invalid Length",
			},
	 }

  for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)
			storeFile, err := StoreWriter(tt.hash, tt.size, tt.offset, os.TempDir())
			if tt.err != "" {
				if assert.NotNil(err) {
					assert.Equal(err.Error(), tt.err)
				}
				return
			}

			// Write chunk received to disk
			_, err = storeFile.Write(tt.content)
			assert.Nil(err)

			storeFile.Close()

			folder1 := string(tt.hash[0:2])
			folder2 := string(tt.hash[2:4])
			fileName := string(tt.hash[4:])

			createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)

			createdFile, err := os.Open(createdFilePath)
			if err != nil {
				t.Errorf("Error: %s opening created file %s", err.Error(), createdFilePath)
				return
			}

			buffer := make([]byte, tt.size)
			createdFile.Seek(tt.offset, 0)
			_, _ = createdFile.Read(buffer)

			createdFile.Close()
			os.RemoveAll(path.Join(os.TempDir(), folder1))

			if string(buffer) != string(tt.expectedContent) {
				t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
				return
			}
		})
  }
}

func TestRetrieve(t *testing.T) {

}
// 	t.Run("it retrieves data successfully", func(t *testing.T) {
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		// Create file for retrieving data into
// 		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
// 		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
// 		if err != nil {
// 			t.Errorf("Error creating file: %s", err.Error())
// 			return
// 		}
// 		defer os.RemoveAll(retrievalFilePath)
// 		defer retrievalFile.Close()
//
// 		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), 0, os.TempDir())
//
// 		if err != nil {
// 			if err != io.EOF {
// 				t.Errorf("Retrieve Error: %s", err.Error())
// 			}
// 		}
//
// 		buffer := make([]byte, 5)
//
// 		retrievalFile.Seek(0, 0)
// 		_, _ = retrievalFile.Read(buffer)
//
// 		fmt.Printf("Retrieved data: %s", string(buffer))
//
// 		if string(buffer) != "butts" {
// 			t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
// 		}
// 	})
//
// 	t.Run("it retrieves data by offset successfully", func(t *testing.T) {
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		// Create file for retrieving data into
// 		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
// 		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
// 		if err != nil {
// 			t.Errorf("Error creating file: %s", err.Error())
// 			return
// 		}
// 		defer os.RemoveAll(retrievalFilePath)
// 		defer retrievalFile.Close()
//
// 		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), 2, os.TempDir())
//
// 		if err != nil {
// 			if err != io.EOF {
// 				t.Errorf("Retrieve Error: %s", err.Error())
// 			}
// 		}
//
// 		buffer := make([]byte, 3)
//
// 		retrievalFile.Seek(0, 0)
// 		_, _ = retrievalFile.Read(buffer)
//
// 		fmt.Printf("Retrieved data: %s", string(buffer))
//
// 		if string(buffer) != "tts" {
// 			t.Errorf("Expected data (tts) does not equal Actual data (%s)", string(buffer))
// 		}
// 	})
//
// 	t.Run("it retrieves data by chunk successfully", func(t *testing.T) {
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		// Create file for retrieving data into
// 		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
// 		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
// 		if err != nil {
// 			t.Errorf("Error creating file: %s", err.Error())
// 			return
// 		}
// 		defer os.RemoveAll(retrievalFilePath)
// 		defer retrievalFile.Close()
//
// 		_, err = Retrieve(hash, retrievalFile, 3, 0, os.TempDir())
//
// 		if err != nil {
// 			if err != io.EOF {
// 				t.Errorf("Retrieve Error: %s", err.Error())
// 			}
// 		}
//
// 		buffer := make([]byte, 3)
//
// 		retrievalFile.Seek(0, 0)
// 		_, _ = retrievalFile.Read(buffer)
//
// 		fmt.Printf("Retrieved data: %s", string(buffer))
//
// 		if string(buffer) != "but" {
// 			t.Errorf("Expected data (but) does not equal Actual data (%s)", string(buffer))
// 		}
// 	})
//
// 	// Test passing in negative offset
// 	t.Run("it should return an error when retrieving with offset less 0", func(t *testing.T) {
// 		assert := assert.New(t)
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		// Create file for retrieving data into
// 		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
// 		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
// 		if err != nil {
// 			t.Errorf("Error creating file: %s", err.Error())
// 			return
// 		}
// 		defer os.RemoveAll(retrievalFilePath)
// 		defer retrievalFile.Close()
//
// 		_, err = Retrieve(hash, retrievalFile, int64(fi.Size()), -1, os.TempDir())
// 		assert.NotNil(err)
// 		if err != nil {
// 			assert.Equal("argError: Invalid offset: -1", err.Error(), err.Error())
// 		}
// 	})
//
// 	// Test passing in negative length
// 	t.Run("it should return the entire file successfully when retrieving with negative length", func(t *testing.T) {
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		// Create file for retrieving data into
// 		retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
// 		retrievalFile, err := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
// 		if err != nil {
// 			t.Errorf("Error creating file: %s", err.Error())
// 			return
// 		}
// 		defer os.RemoveAll(retrievalFilePath)
// 		defer retrievalFile.Close()
//
// 		n, err := Retrieve(hash, retrievalFile, -1, 0, os.TempDir())
// 		fmt.Println(n)
// 		if err != nil {
// 			if err != io.EOF {
// 				t.Errorf("Retrieve Error: %s", err.Error())
// 			}
// 		}
//
// 		buffer := make([]byte, 5)
//
// 		retrievalFile.Seek(0, 0)
// 		_, _ = retrievalFile.Read(buffer)
//
// 		fmt.Printf("Retrieved data: %s", string(buffer))
//
// 		if string(buffer) != "butts" {
// 			t.Errorf("Expected data (butts) does not equal Actual data (%s)", string(buffer))
// 		}
// 	})
//
// }
//
// func TestDelete(t *testing.T) {
// 	t.Run("it deletes data successfully", func(t *testing.T) {
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		folder1 := string(hash[0:2])
// 		folder2 := string(hash[2:4])
// 		fileName := string(hash[4:])
//
// 		if _, err := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName)); err != nil {
// 			t.Errorf("Failed to Store test file")
// 			return
// 		}
//
// 		Delete(hash, os.TempDir())
// 		_, err = os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName))
// 		if err == nil {
// 			t.Errorf("Failed to Delete test file")
// 			return
// 		}
// 	})
//
// 	// Test passing in a hash that doesn't exist
// 	t.Run("it returns an error if hash doesn't exist", func(t *testing.T) {
// 		assert := assert.New(t)
// 		file, err := os.Open(tmpfile)
// 		if err != nil {
// 			t.Errorf("Error opening tmp file: %s", err.Error())
// 			return
// 		}
//
// 		fi, err := file.Stat()
// 		if err != nil {
// 			t.Errorf("Could not stat test file: %s", err.Error())
// 			return
// 		}
//
// 		defer file.Close()
//
// 		hash := "0123456789ABCDEFGHIJ"
// 		Store(hash, file, int64(fi.Size()), 0, os.TempDir())
//
// 		folder1 := string(hash[0:2])
// 		folder2 := string(hash[2:4])
// 		fileName := string(hash[4:])
//
// 		if _, err := os.Stat(path.Join(os.TempDir(), folder1, folder2, fileName)); err != nil {
// 			t.Errorf("Failed to Store test file")
// 			return
// 		}
//
// 		falseHash := ""
//
// 		err = Delete(falseHash, os.TempDir())
// 		assert.NotNil(err)
// 		if err != nil {
// 			assert.NotEqual(err.Error(), "argError: Hash folder does not exist", "They should be equal")
// 		}
// 	})

func TestMain(m *testing.M) {
	m.Run()
}
