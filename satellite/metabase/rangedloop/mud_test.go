// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectSafepoint(t *testing.T) {
	require.NoError(t, rejectSafepoint(Config{}))
	require.Error(t, rejectSafepoint(Config{
		Safepoint: SafepointConfig{PDEndpoints: "localhost:2379"},
	}))
}
