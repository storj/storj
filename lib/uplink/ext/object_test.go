package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/lib/uplink"
	"testing"
)

// TODO: Start up test planet and call these from bash instead
//func TestCObjectTests(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//
//	planet := startTestPlanet(t, ctx)
//	defer ctx.Check(planet.Shutdown)
//
//	consoleProject := newProject(t, planet)
//	consoleApikey := newAPIKey(t, ctx, planet, consoleProject.ID)
//	satelliteAddr := planet.Satellites[0].Addr()
//
//	envVars := []string{
//		"SATELLITE_ADDR=" + satelliteAddr,
//		"APIKEY=" + consoleApikey,
//	}
//
//	runCTest(t, ctx, "object_test.c", envVars...)
//}

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

			donech := make(chan bool)

			go func() {
				DownloadRange(objectRef, 0, 100, &cErr, func(bytes CBytes_t, done Cbool) {
					// convert bytes to thing we can use
					if done {
						donech <- true
					}

					fmt.Println(bytes)
				})

			}()

			<-donech

			// check if bytes received are what we put in
			require.Empty(t, cCharToGoString(cErr))
		}
	})
}