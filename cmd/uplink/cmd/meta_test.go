// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func min(x, y int) int {
	if x < y {
		return x
	}

	return y
}

func TestSetGetMeta(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkExe := ctx.Compile("storj.io/storj/cmd/uplink")

		// Configure uplink.
		{
			output, err := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"setup",
				"--non-interactive",
				"--scope", planet.Uplinks[0].GetConfig(planet.Satellites[0]).Scope,
			).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
		}

		// Create bucket.
		bucketName := testrand.BucketName()

		{
			output, err := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"mb",
				"sj://"+bucketName,
			).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
		}

		// Upload file with metadata.
		metadata := testrand.Metadata()

		metadataBs, err := json.Marshal(metadata)
		require.NoError(t, err)

		metadataStr := string(metadataBs)

		var metadataNorm map[string]string
		err = json.Unmarshal(metadataBs, &metadataNorm)
		require.NoError(t, err)

		path := testrand.Path()
		uri := "sj://" + bucketName + "/" + path

		{
			output, err := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"cp",
				"--metadata", metadataStr,
				"-", uri,
			).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
		}

		// Get all metadata.
		{
			cmd := exec.Command(uplinkExe,
				"--config-dir", ctx.Dir("uplink"),
				"meta", "get", uri,
			)
			t.Log(cmd)

			output, err := cmd.Output()
			t.Log(string(output))
			if !assert.NoError(t, err) {
				if ee, ok := err.(*exec.ExitError); ok {
					t.Log(ee)
					t.Log(string(ee.Stderr))
				}

				return
			}

			var md map[string]string
			err = json.Unmarshal(output, &md)
			require.NoError(t, err)

			assert.Equal(t, metadataNorm, md)
		}

		// Get specific metadata.
		//
		// NOTE: The CLI expects JSON encoded strings for input and
		// output. The key and value returned from the CLI have to be
		// converted from the JSON encoded string into the Go native
		// string for comparison.
		for key, value := range metadataNorm {
			key, value := key, value

			t.Run(fmt.Sprintf("Fetching key %q", key[:min(len(key), 8)]), func(t *testing.T) {
				keyNorm, err := json.Marshal(key)
				require.NoError(t, err)

				cmd := exec.Command(uplinkExe,
					"--config-dir", ctx.Dir("uplink"),
					"meta", "get", "--", string(keyNorm[1:len(keyNorm)-1]), uri,
				)
				t.Log(cmd)

				output, err := cmd.Output()
				assert.NoError(t, err)
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						t.Log(ee)
						t.Log(string(ee.Stderr))
					}

					return
				}

				// Remove trailing newline.
				if len(output) > 0 && string(output[len(output)-1]) == "\n" {
					output = output[:len(output)-1]
				}

				var outputNorm string
				err = json.Unmarshal([]byte("\""+string(output)+"\""), &outputNorm)
				require.NoError(t, err)

				assert.Equal(t, value, outputNorm)
			})
		}
	})
}
