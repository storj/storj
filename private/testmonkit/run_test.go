// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package testmonkit_test

import (
	"context"
	"testing"
	"time"

	"storj.io/storj/private/testmonkit"
)

func TestBasic(t *testing.T) {
	// Set STORJ_TEST_MONKIT=svg,json for this to see the output.
	testmonkit.Run(t.Context(), t, func(ctx context.Context) {
		time.Sleep(100 * time.Millisecond)
	})
}
