// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"testing"
	"time"
	"unsafe"
)

var (
	CLibDir, CSrcDir, CTestsDir, LibuplinkSO string

	testConfig   = new(uplink.Config)
	ciphers      = []storj.CipherSuite{storj.EncNull, storj.EncAESGCM, storj.EncSecretBox}
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

func MemoryFile(data *CUint8, data_len CSize) *CFile {
	return (*CFile)(CFMemOpen(unsafe.Pointer(data), data_len, CCString("r")))
}

func runCTests(t *testing.T, ctx *testcontext.Context, envVars []string, srcGlobs ...string) {
	srcGlobs = append([]string{
		LibuplinkSO,
		filepath.Join(CTestsDir, "unity.c"),
		filepath.Join(CTestsDir, "helpers.c"),
		//filepath.Join(CSrcDir, "*.c"),
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
		zap.NewNop(),
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

func newUplinkInsecure(t *testing.T, ctx *testcontext.Context) *uplink.Uplink {
	cfg := uplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true

	goUplink, err := uplink.NewUplink(ctx, &cfg)
	require.NoError(t, err)
	require.NotEmpty(t, goUplink)

	return goUplink
}

func openTestProject(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) (*uplink.Project, CProjectRef) {
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

	return project, CProjectRef(universe.Add(project))
}

func stringToCCharPtr(str string) CCharPtr {
	return (CCharPtr)(unsafe.Pointer(CCString(str)))
}

func cCharToGoString(cchar CCharPtr) string {
	return CGoString(cchar)
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

	// NB: static code analysis tools can't dereference dynamic types
	params := cBucket.encryption_parameters
	scheme := cBucket.redundancy_scheme

	return storj.Bucket{
		EncryptionParameters: newGoEncryptionParams(&params),
		RedundancyScheme:     newGoRedundancyScheme(&scheme),
		Name:                 CGoString(cBucket.name),
		Created:              time.Unix(int64(cBucket.created), 0).UTC(),
		PathCipher:           storj.Cipher(cBucket.path_cipher),
		SegmentsSize:         int64(cBucket.segment_size),
	}
}

func newGoBucketConfig(cBucketConfig *CBucketConfig) uplink.BucketConfig {
	params := cBucketConfig.encryption_parameters

	return uplink.BucketConfig{
		EncryptionParameters: newGoEncryptionParams(&params),
		PathCipher:           storj.CipherSuite(cBucketConfig.path_cipher),
	}
}

func newGoEncryptionParams(cParams *CEncryptionParameters) storj.EncryptionParameters {
	return storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(cParams.cipher_suite),
		BlockSize:   int32(cParams.block_size),
	}
}

func newGoRedundancyScheme(cScheme *CRedundancyScheme) storj.RedundancyScheme {
	return storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(cScheme.algorithm),
		ShareSize:      int32(cScheme.share_size),
		RequiredShares: int16(cScheme.required_shares),
		RepairShares:   int16(cScheme.repair_shares),
		OptimalShares:  int16(cScheme.optimal_shares),
		TotalShares:    int16(cScheme.total_shares),
	}
}

func newGoObject(t *testing.T, cObj *CObject) *storj.Object {
	var metadata map[string]string
	if uintptr(cObj.metadata) != 0 {
		var ok bool
		metadata, ok = universe.Get(Token(cObj.metadata)).(map[string]string)
		require.True(t, ok)
		require.NotEmpty(t, metadata)
	}
	cBucket := cObj.bucket

	return &storj.Object{
		Version:     uint32(cObj.version),
		Bucket:      newGoBucket(&cBucket),
		Path:        CGoString(cObj.path),
		IsPrefix:    bool(cObj.is_prefix),
		Metadata:    metadata,
		ContentType: CGoString(cObj.content_type),
		Created:     time.Unix(int64(cObj.created), 0),
		Modified:    time.Unix(int64(cObj.modified), 0),
		Expires:     time.Unix(int64(cObj.expires), 0),
	}
}

func newCUploadOpts(opts *uplink.UploadOptions) *CUploadOptions {
	metadataRef := CMapRef(universe.Add(opts.Metadata))
	ts := opts.Expires.Unix()
	return &CUploadOptions{
		content_type: CCString(opts.ContentType),
		metadata:     metadataRef,
		expires:      CNewTime((*CTime)(&ts)),
	}
}
