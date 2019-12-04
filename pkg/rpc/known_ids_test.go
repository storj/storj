// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKnownID(t *testing.T) {
	id, ok := KnownNodeID("us-central-1.tardigrade.io:7777")
	require.True(t, ok)
	require.Equal(t, id.String(), "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S")
	_, ok = KnownNodeID("non-existent.example.com:7777")
	require.False(t, ok)

	id, ok = KnownNodeID("us-central-1.tardigrade.io:10000")
	require.True(t, ok)
	require.Equal(t, id.String(), "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S")

	id, ok = KnownNodeID("us-central-1.tardigrade.io")
	require.True(t, ok)
	require.Equal(t, id.String(), "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S")
}
