// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"io"
	"testing"
)

type testBytes [][]byte

func (t *testBytes) Next() (rv []byte, err error) {
	if len(*t) > 0 {
		rv, *t = (*t)[0], (*t)[1:]
		return rv, nil
	}
	return nil, io.EOF
}

func TestReaderSource(t *testing.T) {
	tb := testBytes([][]byte{
		[]byte("hello there"),
		[]byte("cool"),
		[]byte("beans"),
	})

	rs := NewReaderSource(tb.Next)

	buf := make([]byte, 1)
	n, err := rs.Read(buf)
	if n != 1 || err != nil || string(buf) != "h" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	buf = make([]byte, 10)
	n, err = rs.Read(buf)
	if n != 10 || err != nil || string(buf) != "ello there" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	buf = make([]byte, 5)
	n, err = rs.Read(buf)
	if n != 4 || err != nil || string(buf[:4]) != "cool" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	n, err = rs.Read(buf)
	if n != 5 || err != nil || string(buf[:5]) != "beans" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	n, err = rs.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}
}

type testBytesFastEOF [][]byte

func (t *testBytesFastEOF) Next() (rv []byte, err error) {
	if len(*t) > 0 {
		rv, *t = (*t)[0], (*t)[1:]
		if len(*t) == 0 {
			return rv, io.EOF
		}
		return rv, nil
	}
	return nil, io.EOF
}

func TestReaderSourceFastEOF(t *testing.T) {
	tb := testBytesFastEOF([][]byte{
		[]byte("hello there"),
		[]byte("cool"),
		[]byte("beans"),
	})

	rs := NewReaderSource(tb.Next)

	buf := make([]byte, 1)
	n, err := rs.Read(buf)
	if n != 1 || err != nil || string(buf) != "h" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	buf = make([]byte, 10)
	n, err = rs.Read(buf)
	if n != 10 || err != nil || string(buf) != "ello there" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	buf = make([]byte, 5)
	n, err = rs.Read(buf)
	if n != 4 || err != nil || string(buf[:4]) != "cool" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}

	n, err = rs.Read(buf)
	if n != 5 || err != io.EOF || string(buf[:5]) != "beans" {
		t.Fatalf("unexpected result: %d, %v", n, err)
	}
}
