// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/ranger"
)

func TestCalcEncompassingBlocks(t *testing.T) {
	for _, example := range []struct {
		blockSize                              int
		offset, length, firstBlock, blockCount int64
	}{
		{2, 3, 4, 1, 3},
		{4, 0, 0, 0, 0},
		{4, 0, 1, 0, 1},
		{4, 0, 2, 0, 1},
		{4, 0, 3, 0, 1},
		{4, 0, 4, 0, 1},
		{4, 0, 5, 0, 2},
		{4, 0, 6, 0, 2},
		{4, 1, 0, 0, 0},
		{4, 1, 1, 0, 1},
		{4, 1, 2, 0, 1},
		{4, 1, 3, 0, 1},
		{4, 1, 4, 0, 2},
		{4, 1, 5, 0, 2},
		{4, 1, 6, 0, 2},
		{4, 2, 0, 0, 0},
		{4, 2, 1, 0, 1},
		{4, 2, 2, 0, 1},
		{4, 2, 3, 0, 2},
		{4, 2, 4, 0, 2},
		{4, 2, 5, 0, 2},
		{4, 2, 6, 0, 2},
		{4, 3, 0, 0, 0},
		{4, 3, 1, 0, 1},
		{4, 3, 2, 0, 2},
		{4, 3, 3, 0, 2},
		{4, 3, 4, 0, 2},
		{4, 3, 5, 0, 2},
		{4, 3, 6, 0, 3},
		{4, 4, 0, 1, 0},
		{4, 4, 1, 1, 1},
		{4, 4, 2, 1, 1},
		{4, 4, 3, 1, 1},
		{4, 4, 4, 1, 1},
		{4, 4, 5, 1, 2},
		{4, 4, 6, 1, 2},
	} {
		first, count := calcEncompassingBlocks(
			example.offset, example.length, example.blockSize)
		if first != example.firstBlock || count != example.blockCount {
			t.Fatalf("invalid calculation for %#v: %v %v", example, first, count)
		}
	}
}

func TestCRC(t *testing.T) {
	const blocks = 3
	rr, err := addCRC(ranger.ByteRanger(bytes.Repeat([]byte{0}, blocks*64)),
		crc32.IEEETable)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if rr.Size() != blocks*(64+4+8) {
		t.Fatalf("invalid crc padded size")
	}
	ctx := context.Background()
	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil || int64(len(data)) != rr.Size() {
		t.Fatalf("unexpected: %v", err)
	}

	rr, err = checkCRC(ranger.ByteRanger(data), crc32.IEEETable)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if rr.Size() != blocks*64 {
		t.Fatalf("invalid crc padded size")
	}

	r, err = rr.Range(ctx, 0, rr.Size())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	data, err = ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if !bytes.Equal(data, bytes.Repeat([]byte{0}, blocks*64)) {
		t.Fatalf("invalid data")
	}
}

func TestCRCSubranges(t *testing.T) {
	const blocks = 3
	data := bytes.Repeat([]byte{0, 1, 2}, blocks*64)
	internal, err := addCRC(ranger.ByteRanger(data), crc32.IEEETable)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	external, err := checkCRC(internal, crc32.IEEETable)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if external.Size() != int64(len(data)) {
		t.Fatalf("wrong size")
	}

	ctx := context.Background()
	for i := 0; i < len(data); i++ {
		for j := i; j < len(data); j++ {
			r, err := external.Range(ctx, int64(i), int64(j-i))
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			read, err := ioutil.ReadAll(r)
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !bytes.Equal(read, data[i:j]) {
				t.Fatalf("bad subrange")
			}
		}
	}
}

type nopTransformer struct {
	blockSize int
}

func NopTransformer(blockSize int) Transformer {
	return &nopTransformer{blockSize: blockSize}
}

func (t *nopTransformer) InBlockSize() int {
	return t.blockSize
}

func (t *nopTransformer) OutBlockSize() int {
	return t.blockSize
}

func (t *nopTransformer) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	out = append(out, in...)
	return out, nil
}

func TestTransformer(t *testing.T) {
	transformer := NopTransformer(4 * 1024)
	data := randData(transformer.InBlockSize() * 10)
	transformed := TransformReader(
		ioutil.NopCloser(bytes.NewReader(data)), transformer, 0)
	data2, err := ioutil.ReadAll(transformed)
	if assert.NoError(t, err) {
		assert.Equal(t, data, data2)
	}
}

func TestTransformerSize(t *testing.T) {
	for i, tt := range []struct {
		blockSize     int
		blocks        int
		expectedSize  int64
		unexpectedEOF bool
	}{
		{4, 10, 0, false},
		{4, 10, 3 * 10, false},
		{4, 10, 4*10 - 1, false},
		{4, 10, 4 * 10, false},
		{4, 10, 4*10 + 1, true},
		{4, 10, 4 * 11, true},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		transformer := NopTransformer(tt.blockSize)
		data := randData(transformer.InBlockSize() * tt.blocks)
		transformed := TransformReaderSize(
			ioutil.NopCloser(bytes.NewReader(data)),
			transformer, 0, tt.expectedSize)
		data2, err := ioutil.ReadAll(transformed)
		if tt.unexpectedEOF {
			assert.EqualError(t, err, io.ErrUnexpectedEOF.Error(), errTag)
		} else if assert.NoError(t, err, errTag) {
			assert.Equal(t, data, data2, errTag)
		}
	}
}
