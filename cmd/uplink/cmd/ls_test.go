// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestLsPending(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkExe := ctx.Compile("storj.io/storj/cmd/uplink")

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

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "testbucket")
		require.NoError(t, err)

		// Create pending objects and committed objects.
		{
			uplinkPeer := planet.Uplinks[0]
			satellite := planet.Satellites[0]
			project, err := uplinkPeer.GetProject(ctx, satellite)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.BeginUpload(ctx, bucketName, "pending-object", nil)
			require.NoError(t, err)

			_, err = project.BeginUpload(ctx, bucketName, "prefixed/pending-object", nil)
			require.NoError(t, err)

			err = uplinkPeer.Upload(ctx, satellite, "testbucket", "committed-object", testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			err = uplinkPeer.Upload(ctx, satellite, "testbucket", "prefixed/committed-object", testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}

		// List pending objects non-recursively.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)
			checkOutput(t, output)
		}

		// List pending objects recursively.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
				"--recursive",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)
			checkOutput(t, output,
				bucketName,
				"prefixed/pending-object",
				"pending-object",
			)
		}

		// List pending objects from bucket non-recursively.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
				"sj://"+bucketName,
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			checkOutput(t, output,
				"prefixed",
				"pending-object",
			)
		}

		// List pending object from bucket recursively.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
				"--recursive",
				"sj://"+bucketName,
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			checkOutput(t, output,
				"prefixed/pending-object",
				"pending-object",
			)
		}
		// List pending objects with prefix.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
				"sj://"+bucketName+"/prefixed",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			checkOutput(t, output,
				"prefixed/pending-object",
			)
		}
		// List pending object by specifying object key.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"ls",
				"--pending",
				"sj://"+bucketName+"/prefixed/pending-object",
			)
			t.Log(cmd)

			output, err := cmd.Output()
			require.NoError(t, err)

			checkOutput(t, output,
				"prefixed/pending-object",
			)
		}
	})
}

func checkOutput(t *testing.T, output []byte, objectKeys ...string) {
	lines := strings.Split(string(output), "\n")

	objectKeyFound := false
	foundObjectKeys := make(map[string]bool, len(objectKeys))

	for _, line := range lines {
		if line != "" {
			for _, objectKey := range objectKeys {
				if strings.Contains(line, objectKey) {
					objectKeyFound = true
					foundObjectKeys[objectKey] = true
				}
			}
			require.True(t, objectKeyFound, line, " Object should not be listed.")
		}
	}
	require.Len(t, foundObjectKeys, len(objectKeys))
}
