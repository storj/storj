// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"go.uber.org/zap"
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
	c, err := New(logger, tempfile())
	if err != nil {
		t.Error("Failed to create test db")
	}
	defer func() {
		c.Close()
		os.Remove(c.Path)
	}()

	testEntry1 := PointerEntry{
		Path:    []byte(`test/path`),
		Pointer: []byte(`pointer1`),
	}

	testEntry2 := PointerEntry{
		Path:    []byte(`test/path2`),
		Pointer: []byte(`pointer2`),
	}

	// tests Put function
	if err := c.Put(testEntry1); err != nil {
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
	if err := c.Put(testEntry2); err != nil {
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
