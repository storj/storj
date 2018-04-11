// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/vivint/infectious"
)

func TestRS(t *testing.T) {
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	rs := NewRSScheme(fc, 8*1024)
	readers := EncodeReader(bytes.NewReader(data), rs)
	readerMap := make(map[int]io.Reader, len(readers))
	for i, reader := range readers {
		readerMap[i] = reader
	}
	data2, err := ioutil.ReadAll(DecodeReaders(readerMap, rs))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("rs encode/decode failed")
	}
}
