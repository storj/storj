// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import "github.com/zeebo/errs"

// readVarint reads a varint encoded integer from the front of buf, returning the
// remaining bytes, the value, and if there was a success. if ok is false, the
// returned buffer is the same as the passed in buffer.
func readVarint(buf []byte) (rem []byte, out uint64, ok bool, err error) {
	rem = buf
	for shift := uint(0); shift < 64; shift += 7 {
		if len(rem) == 0 {
			return buf, 0, false, nil
		}
		val := uint64(rem[0])
		out, rem = out|((val&127)<<shift), rem[1:]
		if val < 128 {
			return rem, out, true, nil
		}
	}
	return rem, 0, false, errs.New("varint too long")
}

// appendVarint appends the varint encoding of x to the buffer and returns it.
func appendVarint(buf []byte, x uint64) []byte {
	for x >= 128 {
		buf = append(buf, byte(x&127|128))
		x >>= 7
	}
	return append(buf, byte(x))
}
