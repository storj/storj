// Copyright (C) 2019 Storj Labs, Inc.
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

func TestTee_Basic(t *testing.T) {
	testTees(t, func(t *testing.T, readers []sync2.PipeReader, writer sync2.PipeWriter) {
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

		for i := 0; i < len(readers); i++ {
			i := i
			group.Go(func() error {
				data, err := ioutil.ReadAll(readers[i])
				assert.Equal(t, []byte{1, 2, 3, 1, 2, 3}, data)
				if err != nil {
					assert.Equal(t, io.EOF, err)
				}
				assert.NoError(t, readers[i].Close())
				return nil
			})
		}

		assert.NoError(t, group.Wait())
	})
}

func TestTee_CloseWithError(t *testing.T) {
	testTees(t, func(t *testing.T, readers []sync2.PipeReader, writer sync2.PipeWriter) {
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

		for i := 0; i < len(readers); i++ {
			i := i
			group.Go(func() error {
				_, err := ioutil.ReadAll(readers[i])
				if err != nil {
					assert.Equal(t, failure, err)
				}
				assert.NoError(t, readers[i].Close())
				return nil
			})
		}

		assert.NoError(t, group.Wait())
	})
}

func testTees(t *testing.T, test func(t *testing.T, readers []sync2.PipeReader, writer sync2.PipeWriter)) {
	t.Parallel()
	t.Run("File", func(t *testing.T) {
		t.Parallel()
		readers, writer, err := sync2.NewTeeFile(2, "")
		if err != nil {
			t.Fatal(err)
		}
		test(t, readers, writer)
	})
}
