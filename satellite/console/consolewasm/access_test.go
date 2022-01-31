// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	console "storj.io/storj/satellite/console/consolewasm"
	"storj.io/uplink"
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

// TestDefaultAccess confirms that you can perform basic uplink operations with
// the default access grant created from wasm code.
func TestDefaultAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		projectID := uplinkPeer.Projects[0].ID.String()
		require.Equal(t, 1, len(uplinkPeer.Projects))

		passphrase := "supersecretpassphrase"
		testbucket1 := "buckettest1"
		testfilename := "file.txt"
		testdata := []byte("fun data")

		// Create an access with the console access grant code that allows full access.
		access, err := console.GenAccessGrant(satelliteNodeURL, APIKey.Serialize(), passphrase, projectID)
		require.NoError(t, err)
		newAccess, err := uplink.ParseAccess(access)
		require.NoError(t, err)
		uplinkPeer.Access[satellitePeer.ID()] = newAccess

		// Confirm that we can create a bucket, upload/download/delete an object, and delete the bucket with the new access.
		require.NoError(t, uplinkPeer.CreateBucket(ctx, satellitePeer, testbucket1))
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, testfilename, testdata)
		require.NoError(t, err)
		data, err := uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		buckets, err := uplinkPeer.ListBuckets(ctx, satellitePeer)
		require.NoError(t, err)
		require.Equal(t, len(buckets), 1)
		err = uplinkPeer.DeleteObject(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.NoError(t, uplinkPeer.DeleteBucket(ctx, satellitePeer, testbucket1))
	})
}
