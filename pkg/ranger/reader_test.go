// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestByteRanger(t *testing.T) {
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
		r := ByteRanger([]byte(example.data))
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

func TestConcatReader(t *testing.T) {
	for _, example := range []struct {
		data                 []string
		size, offset, length int64
		substr               string
	}{
		{[]string{}, 0, 0, 0, ""},
		{[]string{""}, 0, 0, 0, ""},
		{[]string{"abcdefghijkl"}, 12, 1, 4, "bcde"},
		{[]string{"abcdef", "ghijkl"}, 12, 1, 4, "bcde"},
		{[]string{"abcdef", "ghijkl"}, 12, 1, 5, "bcdef"},
		{[]string{"abcdef", "ghijkl"}, 12, 1, 6, "bcdefg"},
		{[]string{"abcdef", "ghijkl"}, 12, 5, 4, "fghi"},
		{[]string{"abcdef", "ghijkl"}, 12, 6, 4, "ghij"},
		{[]string{"abcdef", "ghijkl"}, 12, 7, 4, "hijk"},
		{[]string{"abcdef", "ghijkl"}, 12, 7, 5, "hijkl"},
		{[]string{"abcdef", "ghijkl", "mnopqr"}, 18, 7, 7, "hijklmn"},
		{[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"},
			12, 7, 3, "hij"},
	} {
		var readers []Ranger
		for _, data := range example.data {
			readers = append(readers, ByteRanger([]byte(data)))
		}
		r := Concat(readers...)
		if r.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.size)
		}
		data, err := ioutil.ReadAll(r.Range(example.offset, example.length))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !bytes.Equal(data, []byte(example.substr)) {
			t.Fatalf("invalid subrange: %#v != %#v", string(data), example.substr)
		}
	}
}

func TestSubranger(t *testing.T) {
	for _, example := range []struct {
		data             string
		offset1, length1 int64
		offset2, length2 int64
		substr           string
	}{
		{"abcdefghijkl", 0, 4, 0, 4, "abcd"},
		{"abcdefghijkl", 0, 4, 0, 3, "abc"},
		{"abcdefghijkl", 0, 4, 1, 3, "bcd"},
		{"abcdefghijkl", 1, 4, 0, 4, "bcde"},
		{"abcdefghijkl", 1, 4, 0, 3, "bcd"},
		{"abcdefghijkl", 1, 4, 1, 3, "cde"},
		{"abcdefghijkl", 8, 4, 0, 4, "ijkl"},
		{"abcdefghijkl", 8, 4, 0, 3, "ijk"},
		{"abcdefghijkl", 8, 4, 1, 3, "jkl"},
	} {
		r, err := Subrange(ByteRanger([]byte(example.data)),
			example.offset1, example.length1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if r.Size() != example.length1 {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.length1)
		}
		data, err := ioutil.ReadAll(r.Range(example.offset2, example.length2))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !bytes.Equal(data, []byte(example.substr)) {
			t.Fatalf("invalid subrange: %#v != %#v", string(data), example.substr)
		}
	}
}
