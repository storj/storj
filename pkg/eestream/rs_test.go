// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/pkg/readcloser"
	"storj.io/storj/pkg/ranger"
)

func TestRS(t *testing.T) {
	ctx := context.Background()
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 8*1024)
	readers, err := EncodeReader(ctx, bytes.NewReader(data), rs, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	readerMap := make(map[int]io.ReadCloser, len(readers))
	for i, reader := range readers {
		readerMap[i] = ioutil.NopCloser(reader)
	}
	decoder := DecodeReaders(ctx, readerMap, rs, 32*1024, 0)
	defer decoder.Close()
	data2, err := ioutil.ReadAll(decoder)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("rs encode/decode failed")
	}
}

// Check that io.ReadFull will return io.ErrUnexpectedEOF
// if DecodeReaders return less data than expected.
func TestRSUnexpectedEOF(t *testing.T) {
	ctx := context.Background()
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 8*1024)
	readers, err := EncodeReader(ctx, bytes.NewReader(data), rs, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	readerMap := make(map[int]io.ReadCloser, len(readers))
	for i, reader := range readers {
		readerMap[i] = ioutil.NopCloser(reader)
	}
	decoder := DecodeReaders(ctx, readerMap, rs, 32*1024, 0)
	defer decoder.Close()
	// Try ReadFull more data from DecodeReaders than available
	data2 := make([]byte, len(data)+1024)
	_, err = io.ReadFull(decoder, data2)
	assert.EqualError(t, err, io.ErrUnexpectedEOF.Error())
}

func TestRSRanger(t *testing.T) {
	ctx := context.Background()
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 8*1024)
	encKey := sha256.Sum256([]byte("the secret key"))
	var firstNonce [12]byte
	encrypter, err := NewAESGCMEncrypter(
		&encKey, &firstNonce, rs.DecodedBlockSize())
	if err != nil {
		t.Fatal(err)
	}
	readers, err := EncodeReader(ctx, TransformReader(PadReader(ioutil.NopCloser(
		bytes.NewReader(data)), encrypter.InBlockSize()), encrypter, 0),
		rs, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	pieces, err := readAll(readers)
	if err != nil {
		t.Fatal(err)
	}
	rrs := map[int]ranger.Ranger{}
	for i, piece := range pieces {
		rrs[i] = ranger.ByteRanger(piece)
	}
	decrypter, err := NewAESGCMDecrypter(
		&encKey, &firstNonce, rs.DecodedBlockSize())
	rr, err := Decode(rrs, rs, 0)
	if err != nil {
		t.Fatal(err)
	}
	rr, err = Transform(rr, decrypter)
	if err != nil {
		t.Fatal(err)
	}
	rr, err = UnpadSlow(ctx, rr)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		t.Fatal(err)
	}
	data2, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("rs encode/decode failed")
	}
}

func TestRSEncoderInputParams(t *testing.T) {
	for i, tt := range []struct {
		min       int
		opt       int
		mbm       int
		errString string
	}{
		{0, 0, 0, ""},
		{-1, 0, 0, "eestream error: negative minimum threshold"},
		{1, 0, 0, "eestream error: minimum threshold less than required count"},
		{5, 0, 0, "eestream error: minimum threshold greater than total count"},
		{0, -1, 0, "eestream error: negative optimum threshold"},
		{0, 1, 0, "eestream error: optimum threshold less than required count"},
		{0, 5, 0, "eestream error: optimum threshold greater than total count"},
		{3, 4, 0, ""},
		{0, 3, 0, "eestream error: minimum threshold greater than optimum threshold"},
		{4, 3, 0, "eestream error: minimum threshold greater than optimum threshold"},
		{0, 0, -1, "eestream error: negative max buffer memory"},
		{4, 4, 1024, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		ctx := context.Background()
		data := randData(32 * 1024)
		fc, err := infectious.NewFEC(2, 4)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		rs := NewRSScheme(fc, 8*1024)
		_, err = EncodeReader(ctx, bytes.NewReader(data), rs, tt.min, tt.opt, tt.mbm)
		if tt.errString == "" {
			assert.NoError(t, err, errTag)
		} else {
			assert.EqualError(t, err, tt.errString, errTag)
		}
	}
}

func TestRSRangerInputParams(t *testing.T) {
	for i, tt := range []struct {
		min       int
		opt       int
		mbm       int
		errString string
	}{
		{0, 0, 0, ""},
		{-1, 0, 0, "eestream error: negative minimum threshold"},
		{1, 0, 0, "eestream error: minimum threshold less than required count"},
		{5, 0, 0, "eestream error: minimum threshold greater than total count"},
		{0, -1, 0, "eestream error: negative optimum threshold"},
		{0, 1, 0, "eestream error: optimum threshold less than required count"},
		{0, 5, 0, "eestream error: optimum threshold greater than total count"},
		{3, 4, 0, ""},
		{0, 3, 0, "eestream error: minimum threshold greater than optimum threshold"},
		{4, 3, 0, "eestream error: minimum threshold greater than optimum threshold"},
		{0, 0, -1, "eestream error: negative max buffer memory"},
		{4, 4, 1024, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		ctx := context.Background()
		data := randData(32 * 1024)
		fc, err := infectious.NewFEC(2, 4)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		rs := NewRSScheme(fc, 8*1024)
		_, err = EncodeReader(ctx, bytes.NewReader(data), rs, tt.min, tt.opt, tt.mbm)
		if tt.errString == "" {
			assert.NoError(t, err, errTag)
		} else {
			assert.EqualError(t, err, tt.errString, errTag)
		}
	}
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
		{4 * 1024, 1024, 1, 1, 1, false},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 1, 2, 1, false},
		{4 * 1024, 1024, 1, 2, 2, false},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{4 * 1024, 1024, 2, 4, 2, false},
		{4 * 1024, 1024, 2, 4, 3, false},
		{4 * 1024, 1024, 2, 4, 4, false},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, false},
		{6 * 1024, 1024, 3, 7, 4, false},
		{6 * 1024, 1024, 3, 7, 5, false},
		{6 * 1024, 1024, 3, 7, 6, false},
		{6 * 1024, 1024, 3, 7, 7, false},
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

// Some pieces will read slowly
func TestRSSlow(t *testing.T) {
	for i, tt := range []testCase{
		{4 * 1024, 1024, 1, 1, 0, false},
		{4 * 1024, 1024, 1, 2, 0, false},
		{4 * 1024, 1024, 2, 4, 0, false},
		{4 * 1024, 1024, 2, 4, 1, false},
		{6 * 1024, 1024, 3, 7, 0, false},
		{6 * 1024, 1024, 3, 7, 1, false},
		{6 * 1024, 1024, 3, 7, 2, false},
		{6 * 1024, 1024, 3, 7, 3, false},
	} {
		start := time.Now()
		testRSProblematic(t, tt, i, func(in []byte) io.ReadCloser {
			// sleep 1 second before every read
			return ioutil.NopCloser(SlowReader(bytes.NewReader(in), 1*time.Second))
		})
		if time.Since(start) > 1*time.Second {
			t.Fatalf("waited for slow reader")
		}
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
	ctx := context.Background()
	data := randData(tt.dataSize)
	fc, err := infectious.NewFEC(tt.required, tt.total)
	if !assert.NoError(t, err, errTag) {
		return
	}
	rs := NewRSScheme(fc, tt.blockSize)
	readers, err := EncodeReader(ctx, bytes.NewReader(data), rs, 0, 0, 3*1024)
	if !assert.NoError(t, err, errTag) {
		return
	}
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
	decoder := DecodeReaders(ctx, readerMap, rs, int64(tt.dataSize), 3*1024)
	defer decoder.Close()
	data2, err := ioutil.ReadAll(decoder)
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

func SlowReader(r io.Reader, delay time.Duration) io.Reader {
	return &slowReader{Reader: r, Delay: delay}
}

type slowReader struct {
	Reader io.Reader
	Delay  time.Duration
}

func (s *slowReader) Read(p []byte) (n int, err error) {
	time.Sleep(s.Delay)
	return s.Reader.Read(p)
}

func TestEncoderStalledReaders(t *testing.T) {
	ctx := context.Background()
	data := randData(120 * 1024)
	fc, err := infectious.NewFEC(30, 60)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 1024)
	readers, err := EncodeReader(ctx, bytes.NewReader(data), rs, 35, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = readAllStalled(readers, 25)
	assert.NoError(t, err)
	if time.Since(start) > 1*time.Second {
		t.Fatalf("waited for slow reader")
	}
}

func readAllStalled(readers []io.Reader, stalled int) ([][]byte, error) {
	pieces := make([][]byte, len(readers))
	errs := make(chan error, len(readers))
	var err error
	for i := stalled; i < len(readers); i++ {
		go func(i int) {
			pieces[i], err = ioutil.ReadAll(readers[i])
			errs <- err
		}(i)
	}
	for i := stalled; i < len(readers); i++ {
		err := <-errs
		if err != nil {
			return nil, err
		}
	}
	return pieces, nil
}
