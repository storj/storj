// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
)

func TestDeleteObjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(13*memory.KiB),
			),
		},
		UplinkCount: 5, SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplinks := planet.Uplinks
		require.Len(t, uplinks, 5) // The test is based on 5 uplinks

		bucketsObjects := map[string]map[string][]byte{
			"bucket1": {
				"single-segment-object":        testrand.Bytes(10 * memory.KiB),
				"multi-segment-object":         testrand.Bytes(50 * memory.KiB),
				"remote-segment-inline-object": testrand.Bytes(1 * memory.KiB),
			},
			"bucket2": {
				"multi-segment-object": testrand.Bytes(100 * memory.KiB),
			},
			"bucket3": {},
		}

		// 1st Uplink has a project with all the buckets.
		for bucketName, objects := range bucketsObjects {
			for objectName, bytes := range objects {
				require.NoError(t, uplinks[0].Upload(ctx, sat, bucketName, objectName, bytes))
			}
		}

		// 2nd Uplink has a project with one bucket with one object.
		require.NoError(t, uplinks[1].Upload(
			ctx, sat, "my-bucket", "multi-segment-object", bucketsObjects["bucket2"]["multi-segment-object"]),
		)

		// 3rd Uplink has a project with one empty bucket.
		require.NoError(t, uplinks[2].CreateBucket(ctx, sat, "empty-bucket"))

		// 4th Uplink has an empty project.
		// 5th Uplink has project with some buckets and objects & a second project with a bucket with data.
		for bucketName, objects := range bucketsObjects {
			for objectName, bytes := range objects {
				require.NoError(t, uplinks[4].Upload(ctx, sat, bucketName, objectName, bytes))
			}
		}

		var ulkExtProject *uplink.Project
		{ // Create a new project associated with the 5th Uplink user and upload some objects.
			require.Len(t, uplinks[4].Projects, 1)
			owner := uplinks[4].Projects[0].Owner
			proj, err := sat.AddProject(ctx, owner.ID, "a second project")
			require.NoError(t, err)

			userCtx, err := sat.UserContext(ctx, owner.ID)
			require.NoError(t, err)
			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				userCtx, proj.ID, "root", macaroon.APIKeyVersionLatest,
			)
			require.NoError(t, err)

			access, err := uplinks[4].Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)
			ulkExtProject, err = uplink.OpenProject(ctx, access)
			require.NoError(t, err)
			_, err = ulkExtProject.EnsureBucket(ctx, "my-test-bucket")
			require.NoError(t, err)
			upload, err := ulkExtProject.UploadObject(ctx, "my-test-bucket", "test-object", nil)
			require.NoError(t, err)
			_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
		}

		// Ensure the number of objects before the deletion.
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 10)

		// Set the accounts in "pending deletion" status.
		for _, ul := range uplinks {
			pendingStatus := console.PendingDeletion
			require.NoError(t,
				sat.DB.Console().Users().Update(ctx, ul.Projects[0].Owner.ID, console.UpdateUserRequest{
					Status: &pendingStatus,
				}))
		}

		// Create a CSV with the users' emails to delete.
		var csvData io.Reader
		{
			emails := "email"
			for _, uplnk := range uplinks {
				emails += fmt.Sprintf("\n%s", uplnk.User[sat.ID()].Email)
			}

			csvData = bytes.NewBufferString(emails)
		}

		// Delete all the data of the accounts.
		require.NoError(t, deleteObjects(ctx, zap.NewNop(), sat.DB, sat.Metabase.DB, csvData))

		// Check that all the data was deleted.
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 0)

		// check that there aren't buckets.
		for _, uplnk := range uplinks {
			buckets, err := uplnk.ListBuckets(ctx, sat)
			require.NoError(t, err)
			require.Len(t, buckets, 0)
		}

		ulkExtBuckets := ulkExtProject.ListBuckets(ctx, &uplink.ListBucketsOptions{})
		require.False(t, ulkExtBuckets.Next())
	})
}
