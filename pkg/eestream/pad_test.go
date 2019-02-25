// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"storj.io/storj/pkg/ranger"
)

func TestPad(t *testing.T) {
	for examplenum, example := range []struct {
		data      string
		blockSize int
		padding   int
	}{
		{"abcdef", 24, 24 - 6},
		{"abcdef", 6, 6},
		{"abcdef", 7, 8},
		{"abcdef", 8, 10},
		{"abcdef", 9, 12},
		{"abcdef", 10, 4},
		{"abcdef", 11, 5},
		{"abcde", 7, 9},
		{"abcdefg", 7, 7},
		{"abcdef", 512, 506},
		{"abcdef", 32 * 1024, 32*1024 - 6},
		{"", 32 * 1024, 32 * 1024},
		{strings.Repeat("\x00", 16*1024), 32 * 1024, 16 * 1024},
		{strings.Repeat("\x00", 32*1024+1), 32 * 1024, 32*1024 - 1},
	} {
		ctx := context.Background()
		padded, padding := Pad(ranger.ByteRanger([]byte(example.data)),
			example.blockSize)
		if padding != example.padding {
			t.Fatalf("invalid padding: %d, %v != %v", examplenum,
				padding, example.padding)
		}
		if int64(padding+len(example.data)) != padded.Size() {
			t.Fatalf("invalid padding")
		}
		unpadded, err := Unpad(padded, padding)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		r, err := unpadded.Range(ctx, 0, unpadded.Size())
		if err != nil {
			t.Fatalf("unexpected error")
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if !bytes.Equal(data, []byte(example.data)) {
			t.Fatalf("mismatch")
		}
		unpadded, err = UnpadSlow(ctx, padded)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		r, err = unpadded.Range(ctx, 0, unpadded.Size())
		if err != nil {
			t.Fatalf("unexpected error")
		}
		data, err = ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if !bytes.Equal(data, []byte(example.data)) {
			t.Fatalf("mismatch")
		}
	}
}
