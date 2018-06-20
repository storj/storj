// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	tests := []struct {
		it              string
		id              string
		content         []byte
		expectedContent []byte
		err             string
	}{
		{
			it:              "should successfully store data",
			id:              "0123456789ABCDEFGHIJ",
			content:         []byte("butts"),
			expectedContent: []byte("butts"),
			err:             "",
		},
		{
			it:              "should return an error when given an invalid id",
			id:              "012",
			content:         []byte("butts"),
			expectedContent: []byte("butts"),
			err:             "argError: Invalid id length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)
			storeFile, err := StoreWriter(tt.id, os.TempDir())
			if tt.err != "" {
				if assert.NotNil(err) {
					assert.Equal(tt.err, err.Error())
				}
				return
			} else if err != nil {
				t.Errorf("Error: %s", err.Error())
				return
			}

			// Write chunk received to disk
			_, err = storeFile.Write(tt.content)
			assert.Nil(err)

			storeFile.Close()

			folder1 := string(tt.id[0:2])
			folder2 := string(tt.id[2:4])
			fileName := string(tt.id[4:])

			createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)

			createdFile, err := os.Open(createdFilePath)
			if err != nil {
				t.Errorf("Error: %s opening created file %s", err.Error(), createdFilePath)
				return
			}
			buffer := make([]byte, int64(len(tt.content)))
			createdFile.Seek(0, 0)
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
	tests := []struct {
		it              string
		id              string
		size            int64
		offset          int64
		content         []byte
		expectedContent []byte
		err             string
	}{
		{
			it:              "should successfully retrieve data",
			id:              "0123456789ABCDEFGHIJ",
			size:            5,
			offset:          0,
			content:         []byte("butts"),
			expectedContent: []byte("butts"),
			err:             "",
		},
		{
			it:              "should successfully retrieve data by offset",
			id:              "0123456789ABCDEFGHIJ",
			size:            5,
			offset:          5,
			content:         []byte("butts"),
			expectedContent: []byte("butts"),
			err:             "",
		},
		{
			it:              "should successfully retrieve data by chunk",
			id:              "0123456789ABCDEFGHIJ",
			size:            2,
			offset:          5,
			content:         []byte("bu"),
			expectedContent: []byte("bu"),
			err:             "",
		},
		{
			it:              "should return an error when given negative offset",
			id:              "0123456789ABCDEFGHIJ",
			size:            0,
			offset:          -1337,
			content:         []byte("butts"),
			expectedContent: []byte(""),
			err:             "argError: Invalid offset: -1337",
		},
		{
			it:              "should successfully retrieve data with negative length",
			id:              "0123456789ABCDEFGHIJ",
			size:            -1,
			offset:          0,
			content:         []byte("butts"),
			expectedContent: []byte("butts"),
			err:             "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			folder1 := string(tt.id[0:2])
			folder2 := string(tt.id[2:4])
			fileName := string(tt.id[4:])

			createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)

			if err := os.MkdirAll(filepath.Dir(createdFilePath), 0700); err != nil {
				t.Errorf("Error: %s when creating dir", err.Error())
				return
			}

			createdFile, err := os.OpenFile(createdFilePath, os.O_RDWR|os.O_CREATE, 0755)
			if err != nil {
				t.Errorf("Error: %s opening created file %s", err.Error(), createdFilePath)
				return
			}

			createdFile.Seek(tt.offset, 0)
			_, err = createdFile.Write(tt.content)
			if err != nil {
				t.Errorf("Error: %s writing to created file", err.Error())
				return
			}
			createdFile.Close()

			storeFile, err := RetrieveReader(context.Background(), tt.id, tt.offset, tt.size, os.TempDir())
			if tt.err != "" {
				if assert.NotNil(err) {
					assert.Equal(tt.err, err.Error())
				}
				return
			} else if err != nil {
				t.Errorf("Error: %s", err.Error())
				return
			}

			size := tt.size
			if tt.size < 0 {
				size = int64(len(tt.content))
			}
			buffer := make([]byte, size)
			storeFile.Read(buffer)
			storeFile.Close()

			os.RemoveAll(path.Join(os.TempDir(), folder1))

			if string(buffer) != string(tt.expectedContent) {
				t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
				return
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		it  string
		id  string
		err string
	}{
		{
			it:  "should successfully delete data",
			id:  "11111111111111111111",
			err: "",
		},
		{
			it:  "should return nil-err with non-existent id",
			id:  "11111111111111111111",
			err: "",
		},
		{
			it:  "should err with invalid id length",
			id:  "111111",
			err: "argError: Invalid id length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			folder1 := string(tt.id[0:2])
			folder2 := string(tt.id[2:4])
			fileName := string(tt.id[4:])

			createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)

			if err := os.MkdirAll(filepath.Dir(createdFilePath), 0700); err != nil {
				t.Errorf("Error: %s when creating dir", err.Error())
				return
			}

			createdFile, err := os.OpenFile(createdFilePath, os.O_RDWR|os.O_CREATE, 0755)
			if err != nil {
				t.Errorf("Error: %s opening created file %s", err.Error(), createdFilePath)
				return
			}

			createdFile.Close()

			err = Delete(tt.id, os.TempDir())
			if tt.err != "" {
				if assert.NotNil(err) {
					assert.Equal(tt.err, err.Error())
				}
				return
			} else if err != nil {
				t.Errorf("Error: %s", err.Error())
				return
			}

			if _, err = os.Stat(createdFilePath); os.IsExist(err) {
				t.Errorf("Error deleting file")
				return
			}
			return
		})
	}
}

func TestDetermineID(t *testing.T) {
	t.Run("should return an id string", func(t *testing.T) {
		assert := assert.New(t)
		id := DetermineID()
		assert.Equal(len(id) >= IDLength, true)
	})

	t.Run("should return a different string on each call", func(t *testing.T) {
		assert := assert.New(t)
		assert.NotEqual(DetermineID(), DetermineID())
	})
}

func TestMain(m *testing.M) {
	m.Run()
}
