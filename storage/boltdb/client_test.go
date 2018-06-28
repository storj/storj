// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/storage"
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

func TestClient(t *testing.T) {
	cases := []struct {
		bucket  string
		path    []byte
		pointer []byte
	}{
		{
			bucket:  "test-bucket",
			path:    []byte(`test/path`),
			pointer: []byte(`pointer1`),
		},
	}

	for _, c := range cases {
		logger, err := zap.NewDevelopment()
		assert.NoError(t, err)

		client, err := NewClient(logger, tempfile(), c.bucket)
		assert.NoError(t, err)
		defer cleanup(client)

		// tests Put function
		err = client.Put(c.path, c.pointer)
		assert.NoError(t, err)

		// tests Get function
		v, err := client.Get(c.path)
		assert.NoError(t, err)
		assert.Equal(t, c.pointer, []byte(v))

		// tests Delete function
		err = client.Delete(c.path)
		assert.NoError(t, err)

		v, err = client.Get(c.path)
		assert.Error(t, err)
		assert.Empty(t, v)

		// tests List function
		err = client.Put(c.path, c.pointer)
		assert.NoError(t, err)

		p, err := client.List()
		assert.NoError(t, err)
		assert.Len(t, p, 1)
		assert.Equal(t, c.path, p.ByteSlices()[0])
	}
}

func cleanup(c storage.KeyValueStore) {
	c.Close()
	switch client := c.(type) {
	case *boltClient:
		os.Remove(client.Path)
	}
}
