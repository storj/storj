// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

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
		{[]byte{1, 0, 0}, 3, false, []byte{4, 0, 0}, false},
		{[]byte{0}, 256, false, []byte{0}, true},
		{[]byte{0}, 257, false, []byte{1}, true},
		{
			[]byte("\xff\xff\xff\xff\xff\xff\xff\xff\xff\xfe"), 1, false,
			[]byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff"), false},
		{
			[]byte("\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff"), 1, false,
			[]byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), true},
		{
			[]byte("\xff\xff\xff\xff\xff\xfe\xff\xff\xff\xfe"), 1, false,
			[]byte("\x00\x00\x00\x00\x00\xff\xff\xff\xff\xfe"), false},
		{
			[]byte("\xfe\xff\x00\xff\xff\xfe\xff\xff\xff\xfe"), 0xff0001, false,
			[]byte("\xff\xff\xff\xff\xff\xfe\xff\xff\xff\xfe"), false},
		{
			[]byte("\xfe\xff\x00\xff\xff\xff\xff\xff\xff\xff"), 0xff0002, false,
			[]byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), true},
		{
			[]byte("\xfe\xff\x00\xff\xff\xff\xff\xff\xff\xff"), 0xff0003, false,
			[]byte("\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"), true},
	} {

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
