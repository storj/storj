// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"storj.io/storj/storage"
)

type BoltClientTest struct {
	*testing.T
	c storage.KeyValueStore
}

func NewBoltClientTest(t *testing.T) *BoltClientTest {
	logger, _ := zap.NewDevelopment()
	dbName := tempfile()

	c, err := NewClient(logger, dbName, "test_bucket")
	if err != nil {
		t.Error("Failed to create test db")
		panic(err)
	}

	return &BoltClientTest{
		T: t,
		c: c,
	}
}

func (bt *BoltClientTest) Close() {
	bt.c.Close()
	switch client := bt.c.(type) {
	case *Client:
		os.Remove(client.Path)
	}
}

func (bt *BoltClientTest) HandleErr(err error, msg string) {
	bt.Error(msg)
	if err != nil {
		panic(err)
	}
	panic(msg)
}

func tempfile() string {
	f, err := ioutil.TempFile("", "TempBolt-")
	if err != nil {
		panic(err)
	}
	f.Close()
	err = os.Remove(f.Name())
	if err != nil {
		panic(err)
	}
	return f.Name()
}

func TestPut(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	if err := bt.c.Put([]byte("test/path/1"), []byte("pointer1")); err != nil {
		bt.HandleErr(err, "Failed to save pointer1 to pointers bucket")
	}
}

func TestGet(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	if err := bt.c.Put([]byte("test/path/1"), []byte("pointer1")); err != nil {
		bt.HandleErr(err, "Failed to save pointer1 to pointers bucket")
	}

	retrvValue, err := bt.c.Get([]byte("test/path/1"))
	if err != nil {
		bt.HandleErr(err, "Failed to get")
	}
	if retrvValue.IsZero() {
		bt.HandleErr(nil, "Failed to get saved test pointer")
	}
	if !bytes.Equal(retrvValue, []byte("pointer1")) {
		bt.HandleErr(nil, "Retrieved pointer was not same as put pointer")
	}

	// tests Get non-existent path
	getRes, err := bt.c.Get([]byte("fake/path"))
	if err != nil {
		bt.HandleErr(err, "Failed to get")
	}
	if !getRes.IsZero() {
		bt.HandleErr(nil, "Expected zero-value response for getting fake path")
	}
}

func TestDelete(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	if err := bt.c.Put([]byte("test/path/1"), []byte("pointer1")); err != nil {
		bt.HandleErr(err, "Failed to save pointer1 to pointers bucket")
	}

	if err := bt.c.Delete([]byte("test/path/1")); err != nil {
		bt.HandleErr(err, "Failed to delete test entry")
	}
}

func TestList(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	if err := bt.c.Put([]byte("test/path/2"), []byte("pointer2")); err != nil {
		bt.HandleErr(err, "Failed to put pointer2 to pointers bucket")
	}
	testPaths, err := bt.c.List([]byte("test/path/2"), storage.Limit(1))
	if err != nil {
		bt.HandleErr(err, "Failed to list Path keys in pointers bucket")
	}

	if !bytes.Equal(testPaths[0], []byte("test/path/2")) {
		bt.HandleErr(nil, "Expected only test/path/2 in list")
	}
}

func TestListNoStartingKey(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	if err := bt.c.Put([]byte("test/path/1"), []byte("pointer1")); err != nil {
		bt.HandleErr(err, "Failed to save pointer1 to pointers bucket")
	}
	if err := bt.c.Put([]byte("test/path/2"), []byte("pointer2")); err != nil {
		bt.HandleErr(err, "Failed to save pointer2 to pointers bucket")
	}
	if err := bt.c.Put([]byte("test/path/3"), []byte("pointer3")); err != nil {
		bt.HandleErr(err, "Failed to save pointer3 to pointers bucket")
	}

	testPaths, err := bt.c.List(nil, storage.Limit(3))
	if err != nil {
		bt.HandleErr(err, "Failed to list Paths")
	}

	if !bytes.Equal(testPaths[2], []byte("test/path/3")) {
		bt.HandleErr(nil, "Expected test/path/3 to be last in list")
	}
}

func TestGetAll(t *testing.T) {
	bt := NewBoltClientTest(t)
	defer bt.Close()

	cases := []struct {
		setup       func(bt *BoltClientTest) storage.Keys
		expected    storage.Values
		expectedErr error
	}{
		{
			setup: func(bt *BoltClientTest) storage.Keys {
				assert.NoError(t, bt.c.Put([]byte("hello"), []byte("world")))
				return storage.Keys{storage.Key([]byte("hello"))}
			},
			expected:    storage.Values{storage.Value([]byte("world"))},
			expectedErr: nil,
		},
		{
			setup: func(bt *BoltClientTest) storage.Keys {
				keys := storage.Keys{}
				for i := 0; i < 101; i++ {
					key := fmt.Sprintf("hello%d", i)
					assert.NoError(t, bt.c.Put([]byte(key), []byte("world")))
					keys = append(keys, storage.Key([]byte(key)))
				}
				return keys
			},
			expected:    nil,
			expectedErr: Error.New("requested 101 keys, maximum is 100"),
		},
		{
			setup: func(bt *BoltClientTest) storage.Keys {
				assert.NoError(t, bt.c.Put([]byte("hello01"), []byte("world")))
				assert.NoError(t, bt.c.Put([]byte("hello02"), []byte("world")))
				assert.NoError(t, bt.c.Put([]byte("hello03"), []byte("world")))
				return storage.Keys{storage.Key([]byte("hello01")), storage.Key([]byte("hello02")), storage.Key([]byte("hello03"))}
			},
			expected:    storage.Values{storage.Value([]byte("world")), storage.Value([]byte("world")), storage.Value([]byte("world"))},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		bt := NewBoltClientTest(t)
		defer bt.Close()

		keys := v.setup(bt)
		actual, actualErr := bt.c.GetAll(keys)
		if v.expectedErr != nil || actualErr != nil {
			fmt.Println(actualErr)
			assert.EqualError(t, v.expectedErr, actualErr.Error())
		}

		assert.Equal(t, v.expected, actual)
	}
}
