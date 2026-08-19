// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite/metabase/rangedloop"
)

// A safepoint that is not configured must be usable without special casing.
func TestSafepointDisabled(t *testing.T) {
	ctx := context.Background()

	safepoint, err := rangedloop.HoldSafepoint(ctx, zaptest.NewLogger(t), nil, rangedloop.SafepointConfig{
		// set, so that a config which only misses the endpoints stays disabled
		MaxDuration: time.Hour,
		TTL:         time.Hour,
	})
	require.NoError(t, err)

	require.True(t, safepoint.ReadTime().IsZero())

	scanCtx, cancel := safepoint.Context(ctx)
	defer cancel()
	require.NoError(t, scanCtx.Err())
	_, hasDeadline := scanCtx.Deadline()
	require.False(t, hasDeadline)

	require.NoError(t, safepoint.Close())
}
