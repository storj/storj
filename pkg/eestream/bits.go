// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import "math/big"

// incrementBytes takes a byte slice buf and treats it like a big-endian
// encoded unsigned integer. it adds amount to it (which must be nonnegative)
// in place. if rollover happens (the most significant bytes don't fit
// anymore), truncated is true.
func incrementBytes(buf []byte, amount int64) (truncated bool,
	err error) {
	if amount < 0 {
		return false, Error.New("amount was negative")
	}
	// use math/big for the actual incrementing
	var val big.Int
	val.SetBytes(buf)
	val.Add(&val, big.NewInt(amount))
	data := val.Bytes()

	// we went past the available memory. truncate the most significant bytes
	// off
	if len(data) > len(buf) {
		data = data[len(data)-len(buf):]
		truncated = true
	}

	// math/big doesn't return leading 0 bytes so add them back if they're
	// missing
	for i := len(buf) - len(data) - 1; i >= 0; i-- {
		buf[i] = 0
	}

	// write the data out inplace
	copy(buf[len(buf)-len(data):], data[:])

	return truncated, nil
}
