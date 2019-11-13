// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"math/rand"
	"time"
)

func init() {
	// Initialize the seed for the math/rand default source
	rand.Seed(time.Now().UnixNano())
}
