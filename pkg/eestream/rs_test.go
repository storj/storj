// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
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

// Check that io.ReadFull will return io.ErrUnexpectedEOF
// if DecodeReaders return less data than expected.
func TestRSUnexectedEOF(t *testing.T) {
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
	// Try ReadFull more data from DecodeReaders than available
	data2 := make([]byte, len(data)+1024)
	_, err = io.ReadFull(DecodeReaders(readerMap, rs), data2)
	assert.EqualError(t, err, io.ErrUnexpectedEOF.Error())
}

// Some pieces will read error.
// Test will pass if at least required number of pieces are still good.
func TestRSErrors(t *testing.T) {
	for i, tt := range []testCase{
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
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			return readcloser.FatalReadCloser(
				errors.New("I am an error piece"))
		})
	}
}

// Some pieces will read EOF at the beginning (byte 0).
// Test will pass if those pieces are less than required.
func TestRSEOF(t *testing.T) {
	for i, tt := range []testCase{
		{4 * 1024, 1024, 1, 1, 0, false},
		{4 * 1024, 1024, 1, 1, 1, true},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 1, 2, 1, true},
		{4 * 1024, 1024, 1, 2, 2, true},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{4 * 1024, 1024, 2, 4, 2, true},
		{4 * 1024, 1024, 2, 4, 3, true},
		{4 * 1024, 1024, 2, 4, 4, true},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, true},
		{6 * 1024, 1024, 3, 7, 4, true},
		{6 * 1024, 1024, 3, 7, 5, true},
		{6 * 1024, 1024, 3, 7, 6, true},
		{6 * 1024, 1024, 3, 7, 7, true},
	} {
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			return readcloser.LimitReadCloser(
				ioutil.NopCloser(bytes.NewReader(in)), 0)
		})
	}
}

// Some pieces will read EOF earlier than expected
// Test will pass if those pieces are less than required.
func TestRSEarlyEOF(t *testing.T) {
	for i, tt := range []testCase{
		{4 * 1024, 1024, 1, 1, 0, false},
		{4 * 1024, 1024, 1, 1, 1, true},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 1, 2, 1, true},
		{4 * 1024, 1024, 1, 2, 2, true},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{4 * 1024, 1024, 2, 4, 2, true},
		{4 * 1024, 1024, 2, 4, 3, true},
		{4 * 1024, 1024, 2, 4, 4, true},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, true},
		{6 * 1024, 1024, 3, 7, 4, true},
		{6 * 1024, 1024, 3, 7, 5, true},
		{6 * 1024, 1024, 3, 7, 6, true},
		{6 * 1024, 1024, 3, 7, 7, true},
	} {
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			// Read EOF after 500 bytes
			return readcloser.LimitReadCloser(
				ioutil.NopCloser(bytes.NewReader(in)), 500)
		})
	}
}

// Some pieces will read EOF later than expected.
// Test will pass if at least required number of pieces are still good.
func TestRSLateEOF(t *testing.T) {
	for i, tt := range []testCase{
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
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			// extend the input with random number of random bytes
			random := randData(1 + rand.Intn(10000))
			extended := append(in, random...)
			return ioutil.NopCloser(bytes.NewReader(extended))
		})
	}
}

// Some pieces will read random data.
// Test will pass if there are enough good pieces for error correction.
func TestRSRandomData(t *testing.T) {
	for i, tt := range []testCase{
		{4 * 1024, 1024, 1, 1, 0, false},
		{4 * 1024, 1024, 1, 1, 1, true},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 1, 2, 1, true},
		{4 * 1024, 1024, 1, 2, 2, true},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{4 * 1024, 1024, 2, 4, 2, true},
		{4 * 1024, 1024, 2, 4, 3, true},
		{4 * 1024, 1024, 2, 4, 4, true},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, true},
		{6 * 1024, 1024, 3, 7, 4, true},
		{6 * 1024, 1024, 3, 7, 5, true},
		{6 * 1024, 1024, 3, 7, 6, true},
		{6 * 1024, 1024, 3, 7, 7, true},
	} {
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			// return random data instead of expected one
			return ioutil.NopCloser(bytes.NewReader(randData(len(in))))
		})
	}
}

type testCase struct {
	dataSize    int
	blockSize   int
	required    int
	total       int
	problematic int
	fail        bool
}

type problematicReadCloser func([]byte) io.ReadCloser

func testRSProblematic(t *testing.T, tt testCase, i int, fn problematicReadCloser) {
	errTag := fmt.Sprintf("Test case #%d", i)
	data := randData(tt.dataSize)
	fc, err := infectious.NewFEC(tt.required, tt.total)
	if !assert.NoError(t, err, errTag) {
		return
	}
	rs := NewRSScheme(fc, tt.blockSize)
	readers := EncodeReader(bytes.NewReader(data), rs)
	// read all readers in []byte buffers to avoid deadlock if later
	// we don't read in parallel from all of them
	pieces, err := readAll(readers)
	if !assert.NoError(t, err, errTag) {
		return
	}
	readerMap := make(map[int]io.ReadCloser, len(readers))
	// some readers will return EOF later
	for i := 0; i < tt.problematic; i++ {
		readerMap[i] = fn(pieces[i])
	}
	// the rest will operate normally
	for i := tt.problematic; i < tt.total; i++ {
		readerMap[i] = ioutil.NopCloser(bytes.NewReader(pieces[i]))
	}
	data2, err := ioutil.ReadAll(DecodeReaders(readerMap, rs))
	if tt.fail {
		if err == nil && bytes.Compare(data, data2) == 0 {
			assert.Fail(t, "expected to fail, but didn't", errTag)
		}
	} else if assert.NoError(t, err, errTag) {
		assert.Equal(t, data, data2, errTag)
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
