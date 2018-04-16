// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"testing"
)

func TestIncrementBytes(t *testing.T) {
	for _, test := range []struct {
		inbuf     []byte
		amount    int64
		err       bool
		outbuf    []byte
		truncated bool
	}{
		{nil, 10, false, nil, true},
		{nil, 0, false, nil, false},
		{nil, -1, true, nil, false},
		{nil, -1, true, nil, false},
		{nil, -457, true, nil, false},
		{[]byte{0}, 0, false, []byte{0}, false},
		{[]byte{0}, 1, false, []byte{1}, false},
		{[]byte{0}, 254, false, []byte{0xfe}, false},
		{[]byte{1}, 254, false, []byte{0xff}, false},
		{[]byte{0}, 255, false, []byte{0xff}, false},
		{[]byte{0, 0, 1}, 3, false, []byte{0, 0, 4}, false},
		{[]byte{0}, 256, false, []byte{0}, true},
		{[]byte{0}, 257, false, []byte{1}, true},
		{[]byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 1,
			false, []byte{0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 1,
			false, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{[]byte{0xfe, 0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xff}, 1,
			false, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0}, false},
		{[]byte{0xfe, 0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0, 0xff, 0xfe}, 0xff0001,
			false, []byte{0xfe, 0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xff},
			false},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0xff, 0xfe}, 0xff0002,
			false, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0xff, 0xfe}, 0xff0003,
			false, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, true},
	} {
		trunc, err := incrementBytes(test.inbuf, test.amount)
		if err != nil {
			if !test.err {
				t.Fatalf("unexpected err: %v", err)
			}
			continue
		}
		if test.err {
			t.Fatalf("err expected but no err happened")
		}
		if trunc != test.truncated {
			t.Fatalf("truncated rv mismatch")
		}
		if !bytes.Equal(test.outbuf, test.inbuf) {
			t.Fatalf("result mismatch")
		}
	}
}
