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
type CBucketRef = C.BucketRef_t
type CBufferRef = C.BufferRef_t
type CObjectRef = C.ObjectRef_t

// Struct types
type CBucket = C.Bucket_t
type CObject = C.Object_t
type CUploadOptions = C.UploadOptions_t

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

func testEachBucketConfig(t *testing.T, f func(*uplink.BucketConfig)) {
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
			f(&bucketCfg)
		}
	}
}

func newGoBucket(cBucket *CBucket) storj.Bucket {
	params := storj.EncryptionParameters{}
	if unsafe.Pointer(cBucket.encryption_parameters) != nil {
		params = newGoEncryptionParams(cBucket.encryption_parameters)
	}

	scheme := storj.RedundancyScheme{}
	if unsafe.Pointer(cBucket.redundancy_scheme) != nil {
		scheme = newGoRedundancyScheme(cBucket.redundancy_scheme)
	}

	return storj.Bucket{
		EncryptionParameters: params,
		RedundancyScheme:     scheme,
		Name:                 C.GoString(cBucket.name),
		Created:              time.Unix(int64(cBucket.created), 0).UTC(),
		PathCipher:           storj.Cipher(cBucket.path_cipher),
		SegmentsSize:         int64(cBucket.segment_size),
	}
}

func newGoBucketConfig(cBucketConfig *C.BucketConfig_t) uplink.BucketConfig {
	return uplink.BucketConfig{
		EncryptionParameters: newGoEncryptionParams(cBucketConfig.encryption_parameters),
		PathCipher:           storj.CipherSuite(cBucketConfig.path_cipher),
	}
}

func newGoEncryptionParams(cParams *C.EncryptionParameters_t) storj.EncryptionParameters {
	return storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(cParams.cipher_suite),
		BlockSize:   int32(cParams.block_size),
	}
}

func newGoRedundancyScheme(cScheme *C.RedundancyScheme_t) storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(cScheme.algorithm),
		ShareSize:      int32(cScheme.share_size),
		RequiredShares: int16(cScheme.required_shares),
		RepairShares:   int16(cScheme.repair_shares),
		OptimalShares:  int16(cScheme.optimal_shares),
		TotalShares:    int16(cScheme.total_shares),
	}
}

func newGoObject(t *testing.T, cObj *C.Object_t) *storj.Object {
	var metadata map[string]string
	if uintptr(cObj.metadata) != 0 {
		var ok bool
		metadata, ok = structRefMap.Get(token(cObj.metadata)).(map[string]string)
		require.True(t, ok)
		require.NotEmpty(t, metadata)
	}
	cBucket := cObj.bucket

	return &storj.Object{
		Version:     uint32(cObj.version),
		Bucket:      newGoBucket(&cBucket),
		Path:        C.GoString(cObj.path),
		IsPrefix:    bool(cObj.is_prefix),
		Metadata:    metadata,
		ContentType: C.GoString(cObj.content_type),
		Created:     time.Unix(int64(cObj.created), 0),
		Modified:    time.Unix(int64(cObj.modified), 0),
		Expires:     time.Unix(int64(cObj.expires), 0),
	}
}

func newCUploadOpts(opts *uplink.UploadOptions) *C.UploadOptions_t {
	metadataRef := C.MapRef_t(structRefMap.Add(opts.Metadata))
	return &C.UploadOptions_t{
		content_type: C.CString(opts.ContentType),
		metadata:     metadataRef,
		expires:      C.time_t(opts.Expires.Unix()),
	}
}
