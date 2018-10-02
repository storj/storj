// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"testing"
)

func TestIncrementBytes(t *testing.T) {
	for i, test := range []struct {
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
		reverseBytes(test.inbuf)
		reverseBytes(test.outbuf)

		trunc, err := incrementBytes(test.inbuf, test.amount)
		if err != nil {
			if !test.err {
				t.Fatalf("%d: unexpected err: %v", i, err)
			}
			continue
		}
		if test.err {
			t.Fatalf("%d: err expected but no err happened", i)
		}
		if trunc != test.truncated {
			t.Fatalf("%d: truncated rv mismatch", i)
		}
		if !bytes.Equal(test.outbuf, test.inbuf) {
			t.Fatalf("%d: result mismatch\n%v\n%v", i, test.inbuf, test.outbuf)
		}
	}
}

func reverseBytes(xs []byte) {
	for i := len(xs)/2 - 1; i >= 0; i-- {
		opp := len(xs) - 1 - i
		xs[i], xs[opp] = xs[opp], xs[i]
	}
}
