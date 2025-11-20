// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/uplink"
	"storj.io/uplink/private/piecestore"
)

const storjrelease = "v1.0.0" // uses storj.io/uplink v1.0.0-rc.5.0.20200311190324-aee82d3f05aa

func TestOldUplink(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// TODO add different kinds of files: inline, multi segment, multipart

		cmd := exec.Command("go", "install", "storj.io/storj/cmd/uplink@"+storjrelease)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GOBIN="+ctx.Dir("binary"))
		output, err := cmd.CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		oldExpectedData := testrand.Bytes(5 * memory.KiB)
		newExpectedData := testrand.Bytes(5 * memory.KiB)
		srcOldFile := ctx.File("src-old")
		dstOldFile := ctx.File("dst-old")
		dstNewFile := ctx.File("dst-new")

		err = os.WriteFile(srcOldFile, oldExpectedData, 0644)
		require.NoError(t, err)

		access, err := planet.Uplinks[0].Access[planet.Satellites[0].ID()].Serialize()
		require.NoError(t, err)

		projectID := planet.Uplinks[0].Projects[0].ID

		runBinary := func(t *testing.T, args ...string) string {
			args = append(args, "--access="+access)
			output, err := exec.Command(ctx.File("binary", "uplink"), args...).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
			return string(output)
		}

		t.Run("not versioned bucket", func(t *testing.T) {
			err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "bucket")
			require.NoError(t, err)

			// upload with old uplink
			runBinary(t, "cp", srcOldFile, "sj://bucket/old-uplink")

			// upload with new uplink (using SHA-256 piece hash algorithm)
			err = planet.Uplinks[0].Upload(piecestore.WithPieceHashAlgo(ctx, pb.PieceHashAlgorithm_SHA256), planet.Satellites[0], "bucket", "new-uplink-sha256", newExpectedData)
			require.NoError(t, err)

			// upload with new uplink (using BLAKE3 piece hash algorithm)
			err = planet.Uplinks[0].Upload(piecestore.WithPieceHashAlgo(ctx, pb.PieceHashAlgorithm_BLAKE3), planet.Satellites[0], "bucket", "new-uplink-blake3", newExpectedData)
			require.NoError(t, err)

			// uploaded with old uplink and downloaded with old uplink
			runBinary(t, "cp", "sj://bucket/old-uplink", dstOldFile)

			oldData, err := os.ReadFile(dstOldFile)
			require.NoError(t, err)
			require.Equal(t, oldExpectedData, oldData)

			// uploaded with old uplink and downloaded with latest uplink
			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "bucket", "old-uplink")
			require.NoError(t, err)
			require.Equal(t, oldExpectedData, data)

			// uploaded with new uplink and downloaded with old uplink (sha256)
			runBinary(t, "cp", "sj://bucket/new-uplink-sha256", dstNewFile)
			newData, err := os.ReadFile(dstNewFile)
			require.NoError(t, err)
			require.Equal(t, newExpectedData, newData)

			// uploaded with new uplink and downloaded with old uplink (blake3)
			runBinary(t, "cp", "sj://bucket/new-uplink-blake3", dstNewFile)
			newData, err = os.ReadFile(dstNewFile)
			require.NoError(t, err)
			require.Equal(t, newExpectedData, newData)

			cmdResult := runBinary(t, "ls", "sj://bucket/")
			require.Contains(t, cmdResult, "5120 old-uplink")
			require.Contains(t, cmdResult, "5120 new-uplink")

			runBinary(t, "rm", "sj://bucket/old-uplink")
			runBinary(t, "rm", "sj://bucket/new-uplink-blake3")
			runBinary(t, "rm", "sj://bucket/new-uplink-sha256")
		})

		t.Run("versioned bucket", func(t *testing.T) {
			bucketName := "versioned-bucket"
			require.NoError(t, planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, planet.Satellites[0].DB.Buckets().EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			numberOfObjects := 4
			numberOfAllVersions := 0
			expectedObjectKeys := make([]string, numberOfObjects)
			exptectedLatestContent := make([][]byte, numberOfObjects)

			checkListingOutput := func(output string) {
				for _, key := range expectedObjectKeys {
					require.Contains(t, output, key)
				}
			}

			// upload few object with multiple versions
			for i := range numberOfObjects {
				objectKey := "object" + strconv.Itoa(i)
				expectedObjectKeys[i] = objectKey
				numberOfVersions := testrand.Intn(3) + 1
				for range numberOfVersions {
					exptectedLatestContent[i] = testrand.Bytes(10 * memory.KiB)
					err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, objectKey, exptectedLatestContent[i])
					require.NoError(t, err)

					numberOfAllVersions++
				}
			}

			allVersions, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, allVersions, numberOfAllVersions)

			cmdResult := runBinary(t, "ls", "sj://versioned-bucket/")
			checkListingOutput(cmdResult)
			require.Len(t, strings.Split(cmdResult, "\n"), numberOfObjects+1) // all objects + header

			// download latest versions and delete them to create delete markers
			for i, objectKey := range expectedObjectKeys {
				runBinary(t, "cp", "sj://versioned-bucket/"+objectKey, dstOldFile)

				data, err := os.ReadFile(dstOldFile)
				require.NoError(t, err)
				require.Equal(t, exptectedLatestContent[i], data)

				runBinary(t, "rm", "sj://versioned-bucket/"+objectKey)

				// creating delete marker increase number of versions
				numberOfAllVersions++
			}

			cmdResult = runBinary(t, "ls", "sj://versioned-bucket/")
			// all objects have delete markers as latest so no object will be displayed
			require.Len(t, strings.Split(cmdResult, "\n"), 1)

			// upload latest versions using old uplink and try download them
			for _, objectKey := range expectedObjectKeys {
				runBinary(t, "cp", srcOldFile, "sj://versioned-bucket/"+objectKey)

				// uploading new versions
				numberOfAllVersions++

				runBinary(t, "cp", "sj://versioned-bucket/"+objectKey, dstOldFile)

				data, err := os.ReadFile(dstOldFile)
				require.NoError(t, err)
				require.Equal(t, oldExpectedData, data)
			}

			cmdResult = runBinary(t, "ls", "sj://versioned-bucket/")
			checkListingOutput(cmdResult)
			require.Len(t, strings.Split(cmdResult, "\n"), numberOfObjects+1) // all objects + header

			allVersions, err = planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, allVersions, numberOfAllVersions)
		})
	})
}

func TestMove(t *testing.T) {
	if _, err := exec.LookPath("go1.17.13"); err != nil {
		// TODO consider doing to overcome this issue
		// GOTOOLCHAIN=go1.17.13 go download
		// GOTOOLCHAIN=go1.17.13 go install

		// uplink@v1.40.4 requires an older compiler due to quic dependency.
		t.Fatalf("missing suitable compiler go1.17.13: %v", err)
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// old upling is uploading object and moving it
		// new uplink should be able to list it

		cmd := exec.Command("go1.17.13", "install", "storj.io/storj/cmd/uplink@v1.40.4")
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GOBIN="+ctx.Dir("binary"))
		output, err := cmd.CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "bucket")
		require.NoError(t, err)

		expectedData := testrand.Bytes(1 * memory.KiB)
		srcFile := ctx.File("src")

		err = os.WriteFile(srcFile, expectedData, 0644)
		require.NoError(t, err)

		access, err := planet.Uplinks[0].Access[planet.Satellites[0].ID()].Serialize()
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		runBinary := func(args ...string) {
			output, err = exec.Command(ctx.File("binary", "uplink"), args...).CombinedOutput()
			t.Log(string(output))
			require.NoError(t, err)
		}

		// upload with old uplink
		runBinary("cp", srcFile, "sj://bucket/move/old-uplink", "--access="+access)

		// move with old uplink
		runBinary("mv", "sj://bucket/move/old-uplink", "sj://bucket/move/old-uplink-moved", "--access="+access)

		testit := func(key string) {
			cases := []uplink.ListObjectsOptions{
				{System: false, Custom: false},
				{System: true, Custom: false},
				{System: false, Custom: true},
				{System: true, Custom: true},
			}

			for _, tc := range cases {
				tc.Prefix = "move/"
				iterator := project.ListObjects(ctx, "bucket", &tc)
				require.True(t, iterator.Next())
				require.Equal(t, key, iterator.Item().Key)
				require.NoError(t, iterator.Err())
			}
		}

		testit("move/old-uplink-moved")

		// move with old uplink second time
		runBinary("mv", "sj://bucket/move/old-uplink-moved", "sj://bucket/move/old-uplink-moved-second", "--access="+access)

		testit("move/old-uplink-moved-second")
	})
}
