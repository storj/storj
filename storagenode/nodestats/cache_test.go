// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"testing"
	"time"
)

func TestCacheSleep32bitBug(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()

	// Ensure that a large maxSleep doesn't roll over to negative values on 32 bit systems.
	_ = (&Cache{maxSleep: 1 << 32}).sleep(ctx)
}
