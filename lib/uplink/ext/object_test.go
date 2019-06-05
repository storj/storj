package main

import (
	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
	"testing"
	"unsafe"
)

// TODO: Start up test planet and call these from bash instead
func TestCObjectTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	consoleProject := newProject(t, planet)
	consoleApikey := newAPIKey(t, ctx, planet, consoleProject.ID)
	satelliteAddr := planet.Satellites[0].Addr()

	envVars := []string{
		"SATELLITE_ADDR=" + satelliteAddr,
		"APIKEY=" + consoleApikey,
	}

	runCTest(t, ctx, "object_test.c", envVars...)
}

func TestObjectMeta(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(1)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))
		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)

			require.Empty(t, cCharToGoString(cErr))

			path := stringToCCharPtr(string(testObj.Path))
			objectRef := OpenObject(cBucketRef, path, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			objectMeta := ObjectMeta(objectRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))
			require.Equal(t, objectMeta.Path, path)
			// TODO: Check other objectMeta Fields
		}

	})
}

func TestDownloadRange(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	var cErr Cchar
	bucketName := "TestBucket"
	project, _ := openTestProject(t, ctx, planet)

	testObjects := newTestObjects(1)
	testEachBucketConfig(t, func(bucketCfg *uplink.BucketConfig) {
		_, err := project.CreateBucket(ctx, bucketName, bucketCfg)
		require.NoError(t, err)

		openBucket, err := project.OpenBucket(ctx, bucketName, nil)
		require.NoError(t, err)
		require.NotNil(t, openBucket)

		cBucketRef := CBucketRef(structRefMap.Add(openBucket))
		for _, testObj := range testObjects {
			testObj.goUpload(t, ctx, openBucket)

			require.Empty(t, cCharToGoString(cErr))

			path := stringToCCharPtr(string(testObj.Path))
			objectRef := OpenObject(cBucketRef, path, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			objectMeta := ObjectMeta(objectRef, &cErr)
			require.Empty(t, cCharToGoString(cErr))

			reader := DownloadRange(objectRef, 0, Cint64(objectMeta.Size), &cErr)
			require.Empty(t, cCharToGoString(cErr))

			var downloadedData []byte

			for {
				bytes := new(CBytes_t)
				readSize := Download(reader, bytes, &cErr)
				if readSize == CEOF {
					break
				}
				require.Empty(t, cCharToGoString(cErr))

				data := CGoBytes(unsafe.Pointer(bytes.bytes), Cint(bytes.length))

				downloadedData = append(downloadedData, data...)
			}

			require.Equal(t, len(testObj.Data), len(downloadedData))
			require.Equal(t, testObj.Data, downloadedData)
		}
	})
}