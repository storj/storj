// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestFileRanger(t *testing.T) {
	for _, example := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		fail                 bool
	}{
		{"", 0, 0, 0, "", false},
		{"abcdef", 6, 0, 0, "", false},
		{"abcdef", 6, 3, 0, "", false},
		{"abcdef", 6, 0, 6, "abcdef", false},
		{"abcdef", 6, 0, 5, "abcde", false},
		{"abcdef", 6, 0, 4, "abcd", false},
		{"abcdef", 6, 1, 4, "bcde", false},
		{"abcdef", 6, 2, 4, "cdef", false},
		{"abcdefg", 7, 1, 4, "bcde", false},
		{"abcdef", 6, 0, 7, "", true},
		{"abcdef", 6, -1, 7, "abcde", true},
		{"abcdef", 6, 0, -1, "abcde", true},
	} {
		fh, err := ioutil.TempFile("", "test")
		if err != nil {
			t.Fatalf("failed making tempfile")
		}
		_, err = fh.Write([]byte(example.data))
		if err != nil {
			t.Fatalf("failed writing data")
		}
		name := fh.Name()
		err = fh.Close()
		if err != nil {
			t.Fatalf("failed closing data")
		}
		r, err := FileRanger(name)
		if err != nil {
			t.Fatalf("failed opening tempfile")
		}
		defer r.Close()
		if r.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.size)
		}
		data, err := ioutil.ReadAll(r.Range(example.offset, example.length))
		if example.fail {
			if err == nil {
				t.Fatalf("expected error")
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !bytes.Equal(data, []byte(example.substr)) {
				t.Fatalf("invalid subrange: %#v != %#v", string(data), example.substr)
			}
		}
	}
}
