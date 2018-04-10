package boltdb

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func tempfile() string {
	f, _ := ioutil.TempFile("", "TempBolt-")
	f.Close()
	os.Remove(f.Name())
	return f.Name()
}

func TestNetState(t *testing.T) {
	c, err := New(tempfile())
	if err != nil {
		t.Error("Failed to create test db")
	}
	defer func() {
		c.Close()
	}()

	testFile := File{
		Path:  `test/path`,
		Value: `test value`,
	}

	testFile2 := File{
		Path:  `test/path2`,
		Value: `value2`,
	}

	// tests Put function
	if err := c.Put(testFile); err != nil {
		t.Error("Expected testFile saved to files bucket")
	}

	// tests Get function
	retrvFile, err := c.Get([]byte("test/path"))
	if err != nil {
		t.Error("Failed to get saved test value")
	}
	if retrvFile.Value != testFile.Value {
		t.Error("Retrieved file was not same as original file")
	}

	// tests Delete function
	if err := c.Delete([]byte("test/path")); err != nil {
		t.Error("Expected to delete testfile")
	}

	// tests List function
	if err := c.Put(testFile2); err != nil {
		t.Error("Expected testFile2 saved to files bucket")
	}
	testFiles, err := c.List([]byte("files"))
	if err != nil {
		t.Error("Failed to list file keys")
	}

	// tests List + Delete function
	testString := strings.Join(testFiles, "")
	if testString != "test/path2" {
		t.Error("Expected only testFile2 in list")
	}
}
