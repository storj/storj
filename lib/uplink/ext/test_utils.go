// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/satellite/console"
)

/* C types */

// Cchar is an alias to the c character pointer type.
type Cchar = *C.char

// CUint is an alias to the c uint type.
type CUint = C.uint

/* Ref types */

// CAPIKeyRef is an alias to the APIKeyRef_t c type.
type CAPIKeyRef = C.APIKeyRef_t

var (
	// CLibDir is the path to the storj/lib/uplink/ext/c directory.
	CLibDir string
	// CSrcDir is the path to the storj/lib/uplink/ext/c/src directory.
	CSrcDir string
	// CTestsDir is the path to the storj/lib/uplink/ext/c/src directory.
	CTestsDir string
	// LibuplinkSO is the path to the storj/lib/uplink/ext/uplink-cgo.so shared object.
	LibuplinkSO string

	testConfig = new(uplink.Config)
)

func init() {
	// TODO: is there a cleaner way to do this?
	_, thisFile, _, _ := runtime.Caller(0)
	CLibDir = filepath.Join(filepath.Dir(thisFile), "c")
	CSrcDir = filepath.Join(CLibDir, "src")
	CTestsDir = filepath.Join(CLibDir, "tests")
	LibuplinkSO = filepath.Join(CLibDir, "..", "uplink-cgo.so")

	testConfig.Volatile.TLS.SkipPeerCAWhitelist = true
}

func runCTests(t *testing.T, ctx *testcontext.Context, envVars []string, srcGlobs ...string) {
	srcGlobs = append([]string{
		LibuplinkSO,
		filepath.Join(CTestsDir, "unity.c"),
		filepath.Join(CTestsDir, "helpers.c"),
		filepath.Join(CSrcDir, "*.c"),
	}, srcGlobs...)
	testBinPath := ctx.CompileC(srcGlobs...)

	if dir, ok := os.LookupEnv("STORJ_C_TEST_BIN_DIR"); ok {
		err := copyFile(testBinPath, filepath.Join(dir, t.Name()))
		require.NoError(t, err)
	}

	cmd := exec.Command(testBinPath)
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

func runCTest(t *testing.T, ctx *testcontext.Context, filename string, envVars ...string) {
	runCTests(t, ctx, envVars, filename)
}

func startTestPlanet(t *testing.T, ctx *testcontext.Context) *testplanet.Planet {
	planet, err := testplanet.NewCustom(
		zaptest.NewLogger(t),
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 8,
			UplinkCount:      0,
			Reconfigure:      testplanet.DisablePeerCAWhitelist,
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
	APIKey, err := macaroon.NewAPIKey([]byte("testSecret"))
	require.NoError(t, err)

	consoleDB := planet.Satellites[0].DB.Console()

	project, err := consoleDB.Projects().Get(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, project)

	_, err = consoleDB.APIKeys().Create(
		context.Background(),
		APIKey.Head(),
		console.APIKeyInfo{
			Name:      "root",
			ProjectID: project.ID,
			Secret:    []byte("testSecret"),
		},
	)
	require.NoError(t, err)
	return APIKey.Serialize()
}

func stringToCCharPtr(str string) *C.char {
	return (*C.char)(unsafe.Pointer(C.CString(str)))
}

func cCharToGoString(cchar *C.char) string {
	return C.GoString(cchar)
}
