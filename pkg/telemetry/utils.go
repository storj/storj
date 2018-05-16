// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"math/rand"
	"time"
)

func jitter(t time.Duration) time.Duration {
	nanos := rand.NormFloat64()*float64(t/4) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}
