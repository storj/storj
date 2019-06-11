// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include <stdio.h>
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"context"
	"fmt"
	"go.uber.org/zap"
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

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)


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
					BlockSize:   (4 * memory.KiB).Int32(),
				},
			}
			// TODO: we shouldn't have to do this
			bucketCfg.Volatile.RedundancyScheme = storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      1024,
				RequiredShares: 4,
				RepairShares:   6,
				OptimalShares:  8,
				TotalShares:    10,
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
		Name:                 C.GoString(cBucket.name),
		Created:              time.Unix(int64(cBucket.created), 0).UTC(),
		PathCipher:           storj.Cipher(cBucket.path_cipher),
		SegmentsSize:         int64(cBucket.segment_size),
	}
}

func newGoBucketConfig(cBucketConfig *C.BucketConfig_t) uplink.BucketConfig {
	params := cBucketConfig.encryption_parameters

	return uplink.BucketConfig{
		EncryptionParameters: newGoEncryptionParams(&params),
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
		Created:     time.Unix(int64(cObj.created), 0).UTC(),
		Modified:    time.Unix(int64(cObj.modified), 0).UTC(),
		Expires:     time.Unix(int64(cObj.expires), 0).UTC(),
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

func newGoObjectMeta(t *testing.T, cObj *C.ObjectMeta_t) uplink.ObjectMeta {
	var metadata *MapRef
	if uintptr(cObj.MetaData) != 0 {
		var ok bool
		metadata, ok = structRefMap.Get(token(cObj.MetaData)).(*MapRef)
		require.True(t, ok)
	}

	var checksum []byte
	if cObj.Checksum.length > 0 {
		checksum = C.GoBytes(unsafe.Pointer(cObj.Checksum.bytes), cObj.Checksum.length)
	}

	return uplink.ObjectMeta{
		Bucket:      C.GoString(cObj.Bucket),
		Path:        C.GoString(cObj.Path),
		IsPrefix:    bool(cObj.IsPrefix),
		ContentType: C.GoString(cObj.ContentType),
		Metadata:    metadata.m,
		Created:     time.Unix(0, int64(cObj.Created)).UTC(),
		Modified:    time.Unix(0, int64(cObj.Modified)).UTC(),
		Expires:     time.Unix(0, int64(cObj.Expires)).UTC(),
		Size:        int64(cObj.Size),
		Checksum:    checksum,
	}
}
