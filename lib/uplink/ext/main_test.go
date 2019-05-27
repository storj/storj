// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/satellite/console"
)

var (
	cLibDir,
	cSrcDir,
	libuplink string

	testConfig = new(uplink.Config)
)

func init() {
	// TODO: is there a cleaner way to do this?
	_, thisFile, _, _ := runtime.Caller(0)
	cLibDir = filepath.Join(filepath.Dir(thisFile), "c")
	cSrcDir = filepath.Join(cLibDir, "src")
	libuplink = filepath.Join(cLibDir, "..", "uplink-cgo.so")

	testConfig.Volatile.TLS.SkipPeerCAWhitelist = true
}

func TestCCommonTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	runCTest(t, ctx, "common_test.c")
}

func TestCUplinkTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	project := newProject(t, planet)
	apikeyStr := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikeyStr,
	}

	runCTest(t, ctx, "uplink_test.c", envVars...)
}

func TestCProjectTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	createBucketName := "create-bucket-" + t.Name()
	deleteBucketName := "delete-Bucket-" + t.Name()
	project := newProject(t, planet)
	apikey := newAPIKey(t, ctx, planet, project.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"CREATE_BUCKET_NAME=" + createBucketName,
		"DELETE_BUCKET_NAME=" + deleteBucketName,
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + apikey,
	}

	{
		u, err := uplink.NewUplink(ctx, testConfig)
		require.NoError(t, err)

		apikey, err := uplink.ParseAPIKey(apikey)
		require.NoError(t, err)

		project, err := u.OpenProject(ctx, satelliteAddr, apikey, nil)
		require.NoError(t, err)

		deleteBucket, err := project.CreateBucket(ctx, deleteBucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, deleteBucket)

		buckets, err := project.ListBuckets(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, buckets)
		require.NotEmpty(t, buckets.Items)
		assert.Len(t, buckets.Items, 1)

		runCTest(t, ctx, "project_test.c", envVars...)

		// TODO: why does this error when opts is nil?
		buckets, err = project.ListBuckets(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, buckets)
		require.NotEmpty(t, buckets.Items)
		assert.Len(t, buckets.Items, 1)
		assert.Equal(t, createBucketName, buckets.Items[0].Name)
	}
}

//func TestCBucketTest(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	_, err := runCTest(t, ctx, "bucket_test.c")
//	require.NoError(t, err)
//	// TODO: add assertions for expected side-effects in services/dbs on the network (planet)
//}

func runCTest(t *testing.T, ctx *testcontext.Context, filename string, envVars ...string) {
	runCTests(t, ctx, envVars, filepath.Join(cLibDir, "tests", filename))
}

func runCTests(t *testing.T, ctx *testcontext.Context, envVars []string, srcGlobs ...string) {
	srcGlobs = append([]string{
		libuplink,
		filepath.Join(cLibDir, "tests", "unity.*"),
		filepath.Join(cSrcDir, "*.c"),
	}, srcGlobs...)
	testBinPath := ctx.CompileC(srcGlobs...)
	commandPath := testBinPath

	if path, ok := os.LookupEnv("STORJ_DEBUG"); ok {
		err := copyFile(testBinPath, path)
		require.NoError(t, err)
	}

	cmd := exec.Command(commandPath)
	cmd.Env = append(os.Environ(), envVars...)

	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	require.NoError(t, err)
}

func copyFile(src, dest string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, input, 0755)
	if err != nil {
		return err
	}
	return nil
}

func startTestPlanet(t *testing.T, ctx *testcontext.Context) *testplanet.Planet {
	planet, err := testplanet.NewCustom(
		zap.NewNop(),
		testplanet.Config{
			SatelliteCount:     1,
			StorageNodeCount:   8,
			UplinkCount:        0,
			UsePeerCAWhitelist: false,
		},
	)
	require.NoError(t, err)

	planet.Start(ctx)
	return planet
}

func newProject(t *testing.T, planet *testplanet.Planet) *console.Project {
	// TODO: support multiple satellites?
	projectName := t.Name()
	consoleDB := planet.Satellites[0].DB.Console()

	project, err := consoleDB.Projects().Insert(
		context.Background(),
		&console.Project{
			Name: projectName,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, project)

	return project
}

func newAPIKey(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, id uuid.UUID) string {
	// TODO: support multiple satellites?
	projectName := t.Name()
	APIKey := console.APIKeyFromBytes([]byte(projectName))
	consoleDB := planet.Satellites[0].DB.Console()

	project, err := consoleDB.Projects().Get(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, project)

	_, err = consoleDB.APIKeys().Create(
		context.Background(),
		*APIKey,
		console.APIKeyInfo{
			Name:      "root",
			ProjectID: project.ID,
		},
	)
	require.NoError(t, err)
	return APIKey.String()
}
