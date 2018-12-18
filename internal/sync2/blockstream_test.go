// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
)

func TestBlockStream(t *testing.T) {
	stream, err := sync2.NewBlockStream("", 4, 1024)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := stream.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	blockData := [][]byte{
		nil,
		make([]byte, 32),   // a little
		make([]byte, 1024), // exactly the block
		make([]byte, 2048), // more than the block
	}

	for _, block := range blockData {
		_, _ = rand.Read(block[:])
	}

	var group errgroup.Group
	for i := 0; i < 4; i++ {
		index := i
		block := blockData[index]
		reader, writer := stream.Pipe(index)

		group.Go(func() (err error) {
			n, err := writer.Write(block)
			closeErr := writer.Close()
			if len(block) >= 1024 || n == 1024 {
				if err != io.EOF {
					return fmt.Errorf("wrote too much (err %v, close %v)", err, closeErr)
				}
				return nil
			}
			if err != nil {
				return fmt.Errorf("got error %v (close %v)", err, closeErr)
			}
			return closeErr
		})

		group.Go(func() error {
			read := make([]byte, 1024)
			n, err := reader.Read(read[:])
			closeErr := reader.Close()
			if n >= 1024 {
				if err != io.EOF {
					return fmt.Errorf("expected io.EOF got %v", err)
				}
			} else if len(block) == 0 && err == io.EOF {
				return closeErr
			} else if err != nil {
				return fmt.Errorf("reading %v got %v (closeErr %v)", n, err, closeErr)
			}
			if !bytes.Equal(read[:n], block[:n]) {
				return fmt.Errorf("different content (closeErr %v)", closeErr)
			}

			return closeErr
		})
	}

	err = group.Wait()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBlockStream_CloseWithError(t *testing.T) {
	stream, err := sync2.NewBlockStream("", 1, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := stream.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	reader, writer := stream.Pipe(0)
	var failure = errors.New("writer error")
	var group errgroup.Group

	group.Go(func() error {
		return writer.CloseWithError(failure)
	})

	group.Go(func() error {
		_, err := reader.Read([]byte{})
		closeErr := reader.Close()
		if err != failure || closeErr != failure {
			return fmt.Errorf("got %v (closeErr %v)", err, closeErr)
		}
		return nil
	})

	err = group.Wait()
	if err != nil {
		t.Fatal(err)
	}
}
