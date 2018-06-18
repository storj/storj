// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"go.uber.org/zap"
	"storj.io/storj/pkg/netstate"
)

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

func TestNetState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c, err := NewClient(logger, tempfile(), "test_bucket")
	if err != nil {
		t.Error("Failed to create test db")
	}
	defer func() {
		c.Close()
		switch client := c.(type) {
		case *boltClient:
			os.Remove(client.Path)
		}
	}()

	testEntry1 := netstate.PointerEntry{
		Path:    []byte(`test/path`),
		Pointer: []byte(`pointer1`),
	}

	testEntry2 := netstate.PointerEntry{
		Path:    []byte(`test/path2`),
		Pointer: []byte(`pointer2`),
	}

	// tests Put function
	if err := c.Put(testEntry1.Path, testEntry1.Pointer); err != nil {
		t.Error("Failed to save testFile to pointers bucket")
	}

	// tests Get function
	retrvValue, err := c.Get([]byte("test/path"))
	if err != nil {
		t.Error("Failed to get saved test pointer")
	}
	if !bytes.Equal(retrvValue, testEntry1.Pointer) {
		t.Error("Retrieved pointer was not same as put pointer")
	}

	// tests Delete function
	if err := c.Delete([]byte("test/path")); err != nil {
		t.Error("Failed to delete test entry")
	}

	// tests List function
	if err := c.Put(testEntry2.Path, testEntry2.Pointer); err != nil {
		t.Error("Failed to put testEntry2 to pointers bucket")
	}
	testPaths, err := c.List()
	if err != nil {
		t.Error("Failed to list Path keys in pointers bucket")
	}

	// tests List + Delete function
	if !bytes.Equal(testPaths[0], []byte("test/path2")) {
		t.Error("Expected only testEntry2 path in list")
	}
}
