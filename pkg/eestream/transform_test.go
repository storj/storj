// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"hash/crc32"
	"io/ioutil"
	"testing"

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

	data, err := ioutil.ReadAll(rr.Range(0, rr.Size()))
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

	data, err = ioutil.ReadAll(rr.Range(0, rr.Size()))
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

	for i := 0; i < len(data); i++ {
		for j := i; j < len(data); j++ {
			read, err := ioutil.ReadAll(external.Range(int64(i), int64(j-i)))
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !bytes.Equal(read, data[i:j]) {
				t.Fatalf("bad subrange")
			}
		}
	}
}
