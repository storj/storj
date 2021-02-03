// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	console "storj.io/storj/satellite/console/consolewasm"
	"storj.io/uplink"
)

func TestSetPermissionWithBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		apiKeyString := APIKey.Serialize()
		projectID := uplinkPeer.Projects[0].ID.String()
		require.Equal(t, 1, len(uplinkPeer.Projects))
		passphrase := "supersecretpassphrase"

		// Create an access grant with the uplink API key. With that access grant, create 2 buckets and upload an object.
		uplinkAccess, err := uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkPeer.Access[satellitePeer.ID()] = uplinkAccess
		testbucket1 := "buckettest1"
		testbucket2 := "buckettest2"
		testfilename := "file.txt"
		testdata := []byte("fun data")
		require.NoError(t, uplinkPeer.CreateBucket(ctx, satellitePeer, testbucket1))
		require.NoError(t, uplinkPeer.CreateBucket(ctx, satellitePeer, testbucket2))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket1, testfilename, testdata))
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, testbucket2, testfilename, testdata))
		data, err := uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)

		buckets := []string{testbucket1}

		// Restrict the uplink access grant with read only permissions and only allows actions for 1 bucket.
		var sharePrefixes []uplink.SharePrefix
		for _, path := range buckets {
			sharePrefixes = append(sharePrefixes, uplink.SharePrefix{
				Bucket: path,
			})
		}
		restrictedUplinkAccess, err := uplinkAccess.Share(uplink.ReadOnlyPermission(), sharePrefixes...)
		require.NoError(t, err)

		// Expect that we can download the object with the restricted access for the 1 allowed bucket.
		uplinkPeer.Access[satellitePeer.ID()] = restrictedUplinkAccess
		uplinkPeer.APIKey[satellitePeer.ID()] = APIKey
		data, err = uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, "file2", testdata)
		require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		_, err = uplinkPeer.Download(ctx, satellitePeer, testbucket2, testfilename)
		require.Error(t, err)

		// Create restricted access with the console access grant code that allows full access to only 1 bucket.
		readOnlyPermission := console.Permission{
			AllowDownload: true,
			AllowUpload:   false,
			AllowList:     true,
			AllowDelete:   false,
			NotBefore:     time.Now().Add(-24 * time.Hour),
			NotAfter:      time.Now().Add(48 * time.Hour),
		}
		restrictedKey, err := console.SetPermission(apiKeyString, buckets, readOnlyPermission)
		require.NoError(t, err)
		restrictedAccessGrant, err := console.GenAccessGrant(satelliteNodeURL, restrictedKey.Serialize(), passphrase, projectID)
		require.NoError(t, err)
		restrictedAccess, err := uplink.ParseAccess(restrictedAccessGrant)
		require.NoError(t, err)

		// Expect that we can download the object with the restricted access for the 1 allowed bucket.
		uplinkPeer.APIKey[satellitePeer.ID()] = restrictedKey
		uplinkPeer.Access[satellitePeer.ID()] = restrictedAccess
		data, err = uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, "file2", testdata)
		require.True(t, errs2.IsRPC(err, rpcstatus.PermissionDenied))
		_, err = uplinkPeer.Download(ctx, satellitePeer, testbucket2, testfilename)
		require.Error(t, err)
	})
}

func TestSetPermissionUplinkOperations(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		apiKeyString := APIKey.Serialize()
		projectID := uplinkPeer.Projects[0].ID.String()
		require.Equal(t, 1, len(uplinkPeer.Projects))

		allPermission := console.Permission{
			AllowDownload: true,
			AllowUpload:   true,
			AllowList:     true,
			AllowDelete:   true,
			NotBefore:     time.Now().Add(-24 * time.Hour),
			NotAfter:      time.Now().Add(48 * time.Hour),
		}
		restrictedKey, err := console.SetPermission(apiKeyString, []string{}, allPermission)
		require.NoError(t, err)
		passphrase := "supersecretpassphrase"
		restrictedAccessGrant, err := console.GenAccessGrant(satelliteNodeURL, restrictedKey.Serialize(), passphrase, projectID)
		require.NoError(t, err)
		restrictedAccess, err := uplink.ParseAccess(restrictedAccessGrant)
		require.NoError(t, err)

		uplinkPeer.APIKey[satellitePeer.ID()] = restrictedKey
		uplinkPeer.Access[satellitePeer.ID()] = restrictedAccess
		testbucket1 := "buckettest1"
		testfilename := "file.txt"
		testdata := []byte("fun data")

		// Confirm that we can create a bucket, upload/download/delete an object, and delete the bucket with the new restricted access.
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
