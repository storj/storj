// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"errors"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
)

func TestFilePipe_Basic(t *testing.T) {
	reader, writer, err := sync2.NewFilePipe("")
	if err != nil {
		t.Fatal(err)
	}

	var group errgroup.Group
	group.Go(func() error {
		n, err := writer.Write([]byte{1, 2, 3})
		assert.Equal(t, n, 3)
		assert.NoError(t, err)

		n, err = writer.Write([]byte{1, 2, 3})
		assert.Equal(t, n, 3)
		assert.NoError(t, err)

		assert.NoError(t, writer.Close())
		return nil
	})

	group.Go(func() error {
		data, err := ioutil.ReadAll(reader)
		assert.Equal(t, []byte{1, 2, 3, 1, 2, 3}, data)
		if err != nil {
			assert.Equal(t, io.EOF, err)
		}
		assert.NoError(t, reader.Close())
		return nil
	})

	assert.NoError(t, group.Wait())
}

func TestFilePipe_CloseWithError(t *testing.T) {
	reader, writer, err := sync2.NewFilePipe("")
	if err != nil {
		t.Fatal(err)
	}

	var failure = errors.New("write failure")

	var group errgroup.Group
	group.Go(func() error {
		n, err := writer.Write([]byte{1, 2, 3})
		assert.Equal(t, n, 3)
		assert.NoError(t, err)

		err = writer.CloseWithError(failure)
		assert.NoError(t, err)

		return nil
	})

	group.Go(func() error {
		data, err := ioutil.ReadAll(reader)
		assert.Equal(t, []byte{1, 2, 3}, data)
		if err != nil {
			assert.Equal(t, failure, err)
		}
		assert.NoError(t, reader.Close())
		return nil
	})

	assert.NoError(t, group.Wait())
}
