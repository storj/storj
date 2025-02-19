// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package cleanup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	load, err := getLoad()
	require.NoError(t, err)
	require.True(t, load >= float64(0))
	// assuming the build machine is not overloaded
	require.True(t, load < float64(1000))
}
