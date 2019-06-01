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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

// C types
type Cchar = *C.char
type CUint = C.uint
type CBytes = C.Bytes_t
type CUint8 = C.uint8_t
type CInt32 = C.int32_t

// Ref types
type CAPIKeyRef = C.APIKeyRef_t
type CUplinkRef = C.UplinkRef_t
type CProjectRef = C.ProjectRef_t
type CBufferRef = C.BufferRef_t

// Struct types
type CBucket = C.Bucket_t

var (
	cLibDir, cSrcDir, cTestsDir, libuplink string

	testConfig = new(uplink.Config)
	ciphers    = []storj.CipherSuite{storj.EncNull, storj.EncAESGCM, storj.EncSecretBox}
)

func init() {
	// TODO: is there a cleaner way to do this?
	_, thisFile, _, _ := runtime.Caller(0)
	cLibDir = filepath.Join(filepath.Dir(thisFile), "c")
	cSrcDir = filepath.Join(cLibDir, "src")
	cTestsDir = filepath.Join(cLibDir, "tests")
	libuplink = filepath.Join(cLibDir, "..", "uplink-cgo.so")

	testConfig.Volatile.TLS.SkipPeerCAWhitelist = true
}

func runCTests(t *testing.T, ctx *testcontext.Context, envVars []string, srcGlobs ...string) {
	srcGlobs = append([]string{
		libuplink,
		filepath.Join(cTestsDir, "unity.c"),
		filepath.Join(cTestsDir, "helpers.c"),
		filepath.Join(cSrcDir, "*.c"),
	}, srcGlobs...)
	testBinPath := ctx.CompileC(srcGlobs...)
	commandPath := testBinPath

	if dir, ok := os.LookupEnv("STORJ_DEBUG"); ok {
		err := copyFile(testBinPath, filepath.Join(dir, t.Name()))
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

func runCTest(t *testing.T, ctx *testcontext.Context, filename string, envVars ...string) {
	runCTests(t, ctx, envVars, filepath.Join(cLibDir, "tests", filename))
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

func newUplinkInsecure(t *testing.T, ctx *testcontext.Context) *uplink.Uplink {
	cfg := uplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true

	goUplink, err := uplink.NewUplink(ctx, &cfg)
	require.NoError(t, err)
	require.NotEmpty(t, goUplink)

	return goUplink
}

func openTestProject(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) (*uplink.Project, C.ProjectRef_t) {
	consoleProject := newProject(t, planet)
	consoleAPIKey := newAPIKey(t, ctx, planet, consoleProject.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	goUplink := newUplinkInsecure(t, ctx)
	defer ctx.Check(goUplink.Close)

	apikey, err := uplink.ParseAPIKey(consoleAPIKey)
	require.NoError(t, err)
	require.NotEmpty(t, apikey)

	project, err := goUplink.OpenProject(ctx, satelliteAddr, apikey, nil)
	require.NoError(t, err)
	require.NotNil(t, project)

	return project, CProjectRef(structRefMap.Add(project))
}

func stringToCCharPtr(str string) *C.char {
	return (*C.char)(unsafe.Pointer(C.CString(str)))
}

func cCharToGoString(cchar *C.char) string {
	return C.GoString(cchar)
}

func testEachBucketConfig(t *testing.T, f func(uplink.BucketConfig)) {
	for _, suite1 := range ciphers {
		for _, suite2 := range ciphers {
			t.Log(fmt.Sprintf(
				"path cipher: %v; enc params cipher suite: %v",
				suite1, suite2,
			))
			bucketCfg := uplink.BucketConfig{
				PathCipher: suite1,
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: suite2,
				},
			}
			f(bucketCfg)
		}
	}
}

func newGoBucket(cBucket *CBucket) storj.Bucket {
	return storj.Bucket{
		EncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(cBucket.encryption_parameters.cipher_suite),
			BlockSize: int32(cBucket.encryption_parameters.block_size),
		},
		RedundancyScheme: storj.RedundancyScheme{
			Algorithm: storj.RedundancyAlgorithm(cBucket.redundancy_scheme.algorithm),
			ShareSize: int32(cBucket.redundancy_scheme.share_size),
			RequiredShares: int16(cBucket.redundancy_scheme.required_shares),
			RepairShares: int16(cBucket.redundancy_scheme.repair_shares),
			OptimalShares: int16(cBucket.redundancy_scheme.optimal_shares),
			TotalShares: int16(cBucket.redundancy_scheme.total_shares),
		},
		Name: C.GoString(cBucket.name),
		Created: time.Unix(int64(cBucket.created), 0),
		PathCipher: storj.Cipher(cBucket.path_cipher),
		SegmentsSize: int64(cBucket.segment_size),
	}
}
