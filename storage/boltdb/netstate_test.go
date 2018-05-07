// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func tempfile() string {
	f, err := ioutil.TempFile("", "TempBolt-")
	if err != nil {
		panic(err)
	}
	f.Close()
	err := os.Remove(f.Name())
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

	testFile := File{
		Path:  `test/path`,
		Value: []byte(`test value`),
	}

	testFile2 := File{
		Path:  `test/path2`,
		Value: []byte(`value2`),
	}

	// tests Put function
	if err := c.Put(testFile); err != nil {
		t.Error("Failed to save testFile to files bucket")
	}

	// tests Get function
	retrvFile, err := c.Get([]byte("test/path"))
	if err != nil {
		t.Error("Failed to get saved test value")
	}
	if !bytes.Equal(retrvFile.Value, testFile.Value) {
		t.Error("Retrieved file was not same as original file")
	}

	// tests Delete function
	if err := c.Delete([]byte("test/path")); err != nil {
		t.Error("Failed to delete testfile")
	}

	// tests List function
	if err := c.Put(testFile2); err != nil {
		t.Error("Failed to save testFile2 to files bucket")
	}
	testFiles, err := c.List()
	if err != nil {
		t.Error("Failed to list file keys")
	}

	// tests List + Delete function
	testString := strings.Join(testFiles, "")
	if testString != "test/path2" {
		t.Error("Expected only testFile2 in list")
	}
}
