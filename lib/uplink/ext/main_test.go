// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/console"
)

var cLibDir, cSrcDir, cHeadersDir, libuplink string

func init() {
	// TODO: is there a cleaner way to do this?
	_, thisFile, _, _ := runtime.Caller(0)
	cLibDir = filepath.Join(filepath.Dir(thisFile), "c")
	cSrcDir = filepath.Join(cLibDir, "src")
	cHeadersDir = filepath.Join(cLibDir, "headers")
	libuplink = filepath.Join(cLibDir, "..", "uplink-cgo.so")
}

func TestCCommonTest(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := runCTest(t, ctx, "common_test.c")
	defer ctx.Check(planet.Shutdown)
}

func TestCUplinkTest(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := runCTest(t, ctx, "uplink_test.c")
	defer ctx.Check(planet.Shutdown)
}

func TestCProjectTest(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := runCTest(t, ctx, "project_test.c")
	defer ctx.Check(planet.Shutdown)
	// TODO: add assertions for expected side-effects in services/dbs on the network (planet)
}

//func TestCBucketTest(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	_, err := runCTest(t, ctx, "bucket_test.c")
//	require.NoError(t, err)
//	// TODO: add assertions for expected side-effects in services/dbs on the network (planet)
//}

func runCTest(t *testing.T, ctx *testcontext.Context, filename string) *testplanet.Planet {
	return runCTests(t, ctx, filepath.Join(cLibDir, "tests", filename))
}

func runCTests(t *testing.T, ctx *testcontext.Context, srcGlobs ...string) *testplanet.Planet {
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

	// TODO: support multiple satelllites?
	projectName := t.Name()
	APIKey := console.APIKeyFromBytes([]byte(projectName))
	consoleDB := planet.Satellites[0].DB.Console()

	project, err := consoleDB.Projects().Insert(
		context.Background(),
		&console.Project{
			Name: projectName,
		},
	)
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
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SATELLITEADDR=%s", planet.Satellites[0].Addr()),
		fmt.Sprintf("APIKEY=%s", APIKey.String()),
	)

	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	t.Log(string(out))
	return planet
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
