// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKebabToPascal(t *testing.T) {
	require.Equal(t, "StoragenodeUpdater", kebabToPascal("StoragenodeUpdater"))
	require.Equal(t, "StoragenodeUpdater", kebabToPascal("storagenode-updater"))
	require.Equal(t, "Satellite", kebabToPascal("satellite"))
	require.Equal(t, "Storagenode", kebabToPascal("storagenode"))
	require.Equal(t, "Uplink", kebabToPascal("uplink"))
	require.Equal(t, "Gateway", kebabToPascal("gateway"))
	require.Equal(t, "Identity", kebabToPascal("identity"))
}
