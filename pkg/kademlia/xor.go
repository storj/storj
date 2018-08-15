// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"fmt"
	"strconv"
)

func xor(a, b []byte) (int, error) {
	c := make([]byte, len(b))
	rngr := a

	if len(a) >= len(b) {
		c = make([]byte, len(a))
		rngr = b
	}

	for i := range rngr {
		c[i] = a[i] ^ b[i]
	}

	// TODO(coyle): know this isn't performant but couldn't figure how else to do it
	// might be a good PR for someone in the future
	var binString string
	for _, s := range c {
		binString = fmt.Sprintf("%s%b", binString, s)
	}

	i, err := strconv.ParseInt(binString, 2, 64)
	if err != nil {
		return -1, err
	}

	return int(i), nil
}
