// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	console "storj.io/storj/satellite/console/consolewasm"
)

// TestGenerateAccessGrant confirms that the access grant produced by the wasm access code
// is the same as the code the uplink cli uses to create access grants.
func TestGenerateAccessGrant(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()

		uplinkPeer := planet.Uplinks[0]
		apiKeyString := uplinkPeer.Projects[0].APIKey
		projectID := uplinkPeer.Projects[0].ID.String()

		passphrase := "supersecretpassphrase"

		wasmAccessString, err := console.GenAccessGrant(satelliteNodeURL, apiKeyString, passphrase, projectID)
		require.NoError(t, err)

		uplinkCliAccess, err := uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkCliAccessString, err := uplinkCliAccess.Serialize()
		require.NoError(t, err)
		require.Equal(t, wasmAccessString, uplinkCliAccessString)
	})
}
