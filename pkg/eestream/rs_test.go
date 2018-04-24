// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/pkg/readcloser"
)

func TestRS(t *testing.T) {
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 8*1024)
	readers := EncodeReader(bytes.NewReader(data), rs)
	readerMap := make(map[int]io.ReadCloser, len(readers))
	for i, reader := range readers {
		readerMap[i] = ioutil.NopCloser(reader)
	}
	data2, err := ioutil.ReadAll(DecodeReaders(readerMap, rs))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("rs encode/decode failed")
	}
}

func TestRSErrors(t *testing.T) {
	for i, tt := range []struct {
		dataSize    int
		blockSize   int
		required    int
		total       int
		problematic int
		fail        bool
	}{
		{4 * 1024, 1024, 1, 1, 0, false},
		{4 * 1024, 1024, 1, 1, 1, true},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 1, 2, 1, false},
		{4 * 1024, 1024, 1, 2, 2, true},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{4 * 1024, 1024, 2, 4, 2, false},
		{4 * 1024, 1024, 2, 4, 3, true},
		{4 * 1024, 1024, 2, 4, 4, true},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, false},
		{6 * 1024, 1024, 3, 7, 4, false},
		{6 * 1024, 1024, 3, 7, 5, true},
		{6 * 1024, 1024, 3, 7, 6, true},
		{6 * 1024, 1024, 3, 7, 7, true},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		data := randData(tt.dataSize)
		fc, err := infectious.NewFEC(tt.required, tt.total)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		rs := NewRSScheme(fc, tt.blockSize)
		readers := EncodeReader(bytes.NewReader(data), rs)
		// read all readers in []byte buffers to avoid deadlock if later
		// we don't read in parallel from all of them
		pieces, err := readAll(readers)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		readerMap := make(map[int]io.ReadCloser, len(readers))
		// some readers will return error on read
		for i := 0; i < tt.problematic; i++ {
			readerMap[i] = readcloser.FatalReadCloser(
				errors.New("I am an error piece"))
		}
		// the rest will operate normally
		for i := tt.problematic; i < tt.total; i++ {
			readerMap[i] = ioutil.NopCloser(bytes.NewReader(pieces[i]))
		}
		data2, err := ioutil.ReadAll(DecodeReaders(readerMap, rs))
		if tt.fail {
			assert.Error(t, err, errTag)
		} else if assert.NoError(t, err, errTag) {
			assert.Equal(t, data, data2, errTag)
		}
	}
}

func readAll(readers []io.Reader) ([][]byte, error) {
	pieces := make([][]byte, len(readers))
	errs := make(chan error, len(readers))
	var err error
	for i := range readers {
		go func(i int) {
			pieces[i], err = ioutil.ReadAll(readers[i])
			errs <- err
		}(i)
	}
	for range readers {
		err := <-errs
		if err != nil {
			return nil, err
		}
	}
	return pieces, nil
}
