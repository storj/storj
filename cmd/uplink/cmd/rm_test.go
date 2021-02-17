// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/uplink"
	"storj.io/uplink/private/multipart"
)

func TestRmPending(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkExe := ctx.Compile("storj.io/storj/cmd/uplink")

		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		project, err := uplinkPeer.GetProject(ctx, satellite)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// Configure uplink.
		{
			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]

			accessString, err := access.Serialize()
			require.NoError(t, err)

			output, err := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"import",
				accessString,
			).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
		}

		// Create bucket.
		bucketName := "testbucket"

		err = uplinkPeer.CreateBucket(ctx, satellite, "testbucket")
		require.NoError(t, err)

		// Create pending objects and one committed object.
		{

			_, err = multipart.NewMultipartUpload(ctx, project, bucketName, "pending-object", nil)
			require.NoError(t, err)

			_, err = multipart.NewMultipartUpload(ctx, project, bucketName, "prefixed/pending-object", nil)
			require.NoError(t, err)

			err = uplinkPeer.Upload(ctx, satellite, "testbucket", "committed-object", testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}

		// Try to delete a non-existing object.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"rm",
				"--pending",
				"sj://"+bucketName+"/does-not-exist",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(output), "Deleted sj://"+bucketName+"/does-not-exist"))
		}

		// Try to delete a pending object without specifying --pending.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"rm",
				"sj://"+bucketName+"/pending-object",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(output), "Deleted sj://"+bucketName+"/pending-object"))
			require.True(t, pendingObjectExists(ctx, satellite, project, bucketName, "pending-object"))

		}

		// Try to delete a committed object.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"rm",
				"--pending",
				"sj://"+bucketName+"/committed-object",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(output), "Deleted sj://"+bucketName+"/committed-object"))
			require.True(t, committedObjectExists(ctx, satellite, project, bucketName, "committed-object"))

		}

		// Delete pending object without prefix.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"rm",
				"--pending",
				"sj://"+bucketName+"/pending-object",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			require.True(t, strings.HasPrefix(string(output), "Deleted sj://"+bucketName+"/pending-object"))
			require.False(t, pendingObjectExists(ctx, satellite, project, bucketName, "pending-object"))
		}

		// Delete pending object with prefix.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"rm",
				"--pending",
				"sj://"+bucketName+"/prefixed/pending-object",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			require.True(t, strings.HasPrefix(string(output), "Deleted sj://"+bucketName+"/prefixed/pending-object"))
			require.False(t, pendingObjectExists(ctx, satellite, project, bucketName, "prefixed/pending-object"))
		}
	})
}

func pendingObjectExists(ctx context.Context, satellite *testplanet.Satellite, project *uplink.Project, bucketName string, objectKey string) bool {
	iterator := multipart.ListPendingObjectStreams(ctx, project, bucketName, objectKey, nil)
	return iterator.Next()
}

func committedObjectExists(ctx context.Context, satellite *testplanet.Satellite, project *uplink.Project, bucketName string, objectKey string) bool {
	_, err := project.StatObject(ctx, bucketName, objectKey)
	return err == nil
}
